package nacosx

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
)

// ClientSettings is shared by Nacos config-center and service-registry adapters.
type ClientSettings struct {
	Servers       []string
	Namespace     string
	Username      string
	Password      string
	LogLevel      string
	TLSRequired   bool
	TLSCAFile     string
	TLSCertFile   string
	TLSKeyFile    string
	TLSServerName string
}

func Build(settings ClientSettings) (constant.ClientConfig, []constant.ServerConfig, error) {
	if len(settings.Servers) == 0 {
		return constant.ClientConfig{}, nil, fmt.Errorf("nacos servers required")
	}
	cc := constant.ClientConfig{
		NamespaceId:          settings.Namespace,
		TimeoutMs:            5000,
		NotLoadCacheAtStart:  false,
		UpdateCacheWhenEmpty: false,
		Username:             settings.Username,
		Password:             settings.Password,
		LogDir:               "/tmp/forge-nacos/log",
		CacheDir:             "/tmp/forge-nacos/cache",
		LogLevel:             defaultString(settings.LogLevel, "warn"),
	}
	servers := make([]constant.ServerConfig, 0, len(settings.Servers))
	scheme := ""
	for _, raw := range settings.Servers {
		sc, err := parseServer(raw)
		if err != nil {
			return constant.ClientConfig{}, nil, err
		}
		if scheme != "" && scheme != sc.Scheme {
			return constant.ClientConfig{}, nil, fmt.Errorf("nacos servers must not mix %s and %s schemes", scheme, sc.Scheme)
		}
		scheme = sc.Scheme
		servers = append(servers, sc)
	}
	if settings.TLSRequired && scheme != "https" {
		return constant.ClientConfig{}, nil, fmt.Errorf("nacos TLS is required but server scheme is %s", scheme)
	}
	if (settings.TLSCertFile == "") != (settings.TLSKeyFile == "") {
		return constant.ClientConfig{}, nil, fmt.Errorf("nacos TLS certificate and key must be configured together")
	}
	cc.TLSCfg = constant.TLSConfig{
		Appointed:          true,
		Enable:             scheme == "https",
		TrustAll:           false,
		CaFile:             settings.TLSCAFile,
		CertFile:           settings.TLSCertFile,
		KeyFile:            settings.TLSKeyFile,
		ServerNameOverride: settings.TLSServerName,
	}
	return cc, servers, nil
}

func parseServer(raw string) (constant.ServerConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return constant.ServerConfig{}, fmt.Errorf("empty nacos server")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return constant.ServerConfig{}, fmt.Errorf("nacos server %q: %w", raw, err)
	}
	host := u.Hostname()
	if host == "" {
		return constant.ServerConfig{}, fmt.Errorf("nacos server %q missing host", raw)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return constant.ServerConfig{}, fmt.Errorf("nacos server %q uses unsupported scheme %q", raw, u.Scheme)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return constant.ServerConfig{}, fmt.Errorf("nacos server %q must not contain credentials, query, or fragment", raw)
	}
	port := uint64(8848)
	if p := u.Port(); p != "" {
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return constant.ServerConfig{}, fmt.Errorf("nacos server %q invalid port", raw)
		}
		port = n
	}
	path := strings.TrimSpace(u.Path)
	if path == "/" {
		path = ""
	}
	return constant.ServerConfig{IpAddr: host, Port: port, Scheme: scheme, ContextPath: path}, nil
}

func defaultString(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}
