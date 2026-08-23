package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type enrollment struct {
	ApplicationCode         string        `json:"application_code"`
	Issuer                  string        `json:"issuer"`
	ClientID                string        `json:"client_id"`
	ClientSecret            string        `json:"client_secret"`
	RedirectURIs            []string      `json:"redirect_uris"`
	Scopes                  []string      `json:"scopes"`
	ProvisioningEndpoint    string        `json:"provisioning_endpoint"`
	ProvisioningSecret      string        `json:"provisioning_secret"`
	ProvisioningKeyVersion  flexibleInt64 `json:"provisioning_key_version"`
	ProvisioningFingerprint string        `json:"provisioning_fingerprint"`
}

// flexibleInt64 accepts both protobuf JSON's quoted int64 representation and
// ordinary JSON numbers so the CLI remains compatible with both gateways.
type flexibleInt64 int64

func (v *flexibleInt64) UnmarshalJSON(raw []byte) error {
	value := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return err
	}
	*v = flexibleInt64(parsed)
	return nil
}

type envelope struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

func main() {
	if len(os.Args) < 2 {
		fatal("用法：velora-connect enroll|doctor [参数]")
	}
	var err error
	switch os.Args[1] {
	case "enroll":
		err = enroll(os.Args[2:], os.Stdin, http.DefaultClient)
	case "doctor":
		err = doctor(os.Args[2:])
	default:
		err = errors.New("未知命令，仅支持 enroll 或 doctor")
	}
	if err != nil {
		fatal(err.Error())
	}
}

func enroll(args []string, stdin io.Reader, client *http.Client) error {
	fs := flag.NewFlagSet("enroll", flag.ContinueOnError)
	portal := fs.String("portal", "", "Velora 门户地址")
	output := fs.String("output", "", "安全配置目录")
	tokenFile := fs.String("token-file", "", "包含 Enrollment Token 的只读文件")
	if err := fs.Parse(args); err != nil {
		return err
	}
	u, err := validatedPortal(*portal)
	if err != nil {
		return err
	}
	dir, err := prepareOutput(*output)
	if err != nil {
		return err
	}
	token, err := readToken(stdin, *tokenFile)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"enrollment_token": token})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String()+"/api/v1/application-enrollments:consume", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接 Velora 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var wrapped envelope
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return errors.New("Velora 返回了无效响应")
	}
	if resp.StatusCode != http.StatusOK || wrapped.Code != "000000" {
		return fmt.Errorf("领取失败: %s (request_id=%s)", safeMessage(wrapped.Message), wrapped.RequestID)
	}
	var bundle enrollment
	if err := json.Unmarshal(wrapped.Data, &bundle); err != nil || bundle.ApplicationCode == "" || len(bundle.ClientSecret) < 16 || len(bundle.ProvisioningSecret) < 32 {
		return errors.New("接入包不完整，未写入任何文件")
	}
	clientSecretPath := filepath.Join(dir, "oidc-client-secret")
	provisioningSecretPath := filepath.Join(dir, "provisioning-secret")
	configPath := filepath.Join(dir, "velora.env")
	if err := atomicWrite(clientSecretPath, []byte(bundle.ClientSecret+"\n"), 0o600); err != nil {
		return err
	}
	if err := atomicWrite(provisioningSecretPath, []byte(bundle.ProvisioningSecret+"\n"), 0o600); err != nil {
		return err
	}
	config := fmt.Sprintf("VELORA_APPLICATION_CODE=%s\nVELORA_OIDC_ISSUER=%s\nVELORA_OIDC_CLIENT_ID=%s\nVELORA_OIDC_CLIENT_SECRET_FILE=%s\nVELORA_OIDC_REDIRECT_URI=%s\nVELORA_OIDC_SCOPES=%s\nVELORA_PROVISIONING_ENDPOINT=%s\nVELORA_PROVISIONING_SECRET_FILE=%s\nVELORA_PROVISIONING_KEY_VERSION=%d\nVELORA_PROVISIONING_FINGERPRINT=%s\n", shellValue(bundle.ApplicationCode), shellValue(bundle.Issuer), shellValue(bundle.ClientID), shellValue(clientSecretPath), shellValue(first(bundle.RedirectURIs)), shellValue(strings.Join(bundle.Scopes, " ")), shellValue(bundle.ProvisioningEndpoint), shellValue(provisioningSecretPath), bundle.ProvisioningKeyVersion, shellValue(bundle.ProvisioningFingerprint))
	if err := atomicWrite(configPath, []byte(config), 0o600); err != nil {
		return err
	}
	fmt.Printf("接入配置已安全写入 %s（应用 %s，密钥指纹 %s）\n", dir, bundle.ApplicationCode, bundle.ProvisioningFingerprint)
	return nil
}

func doctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	configPath := fs.String("config", "", "velora.env 路径")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path, err := filepath.Abs(strings.TrimSpace(*configPath))
	if err != nil || strings.TrimSpace(*configPath) == "" {
		return errors.New("--config 必须是配置文件路径")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("配置文件必须是非符号链接且权限不得开放给组或其他用户")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	fmt.Printf("配置检查通过：path=%s sha256=%s permissions=%04o\n", path, hex.EncodeToString(digest[:8]), info.Mode().Perm())
	return nil
}

func validatedPortal(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("--portal 必须是无 Query/Fragment 的 HTTPS 地址")
	}
	return u, nil
}

func prepareOutput(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("--output 不能为空")
	}
	dir, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("输出路径必须是非符号链接目录")
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func readToken(stdin io.Reader, tokenFile string) (string, error) {
	var raw []byte
	var err error
	if strings.TrimSpace(tokenFile) != "" {
		info, statErr := os.Lstat(tokenFile)
		if statErr != nil {
			return "", statErr
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
			return "", errors.New("Token 文件必须是非符号链接且权限为 0600")
		}
		raw, err = os.ReadFile(tokenFile)
	} else {
		fmt.Fprint(os.Stderr, "请输入 Enrollment Token：")
		raw, err = bufio.NewReader(io.LimitReader(stdin, 4096)).ReadBytes('\n')
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	token := strings.TrimSpace(string(raw))
	if len(token) < 32 {
		return "", errors.New("Enrollment Token 格式无效")
	}
	return token, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".velora-connect-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func shellValue(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
func safeMessage(value string) string {
	if strings.TrimSpace(value) == "" {
		return "请求被拒绝"
	}
	return strings.TrimSpace(value)
}
func fatal(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
