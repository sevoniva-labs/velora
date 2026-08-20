package httpserver

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const cspNonceMarker = "__VELORA_CSP_NONCE__"

type SPAOptions struct {
	Root            string
	FrameSources    []string
	ConnectSources  []string
	WujieCSPEnabled bool
}

func SPA(options SPAOptions) http.Handler {
	absoluteRoot, err := filepath.Abs(options.Root)
	if err != nil {
		absoluteRoot = options.Root
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root, rootErr := os.OpenRoot(absoluteRoot)
		if rootErr != nil {
			writeError(w, http.StatusInternalServerError, "WEB_STATIC_RESOURCE_UNAVAILABLE", "web static root is unavailable")
			return
		}
		defer func() { _ = root.Close() }()
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "API route not found")
			return
		}
		for _, segment := range strings.Split(filepath.ToSlash(r.URL.Path), "/") {
			if segment == ".." {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
				return
			}
		}
		clean := strings.TrimPrefix(filepath.Clean(filepath.FromSlash(r.URL.Path)), string(filepath.Separator))
		if clean == "index.html" {
			serveSPAIndex(w, r, root, options)
			return
		}
		if clean != "." && clean != "" {
			if info, statErr := root.Stat(clean); statErr == nil && !info.IsDir() {
				file, openErr := root.Open(clean)
				if openErr != nil {
					writeError(w, http.StatusInternalServerError, "WEB_STATIC_RESOURCE_UNAVAILABLE", "web static resource is unavailable")
					return
				}
				defer func() { _ = file.Close() }()

				setStaticCacheControl(w.Header(), clean)
				http.ServeContent(w, r, filepath.Base(clean), info.ModTime(), file)
				return
			}
		}
		serveSPAIndex(w, r, root, options)
	})
}

func serveSPAIndex(w http.ResponseWriter, r *http.Request, root *os.Root, options SPAOptions) {
	info, err := root.Stat("index.html")
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		return
	}
	file, err := root.Open("index.html")
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		return
	}
	content, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil || !bytes.Contains(content, []byte(cspNonceMarker)) {
		writeError(w, http.StatusInternalServerError, "WEB_SECURITY_CONFIGURATION_INVALID", "web security configuration is invalid")
		return
	}
	nonceBytes := make([]byte, 24)
	if _, err := rand.Read(nonceBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "WEB_SECURITY_NONCE_UNAVAILABLE", "web security nonce is unavailable")
		return
	}
	nonce := base64.RawStdEncoding.EncodeToString(nonceBytes)
	content = bytes.ReplaceAll(content, []byte(cspNonceMarker), []byte(nonce))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", webContentSecurityPolicy(nonce, options))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", info.ModTime(), bytes.NewReader(content))
}

func webContentSecurityPolicy(_ string, options SPAOptions) string {
	frameSources := append([]string(nil), options.FrameSources...)
	connectSources := append([]string{"'self'"}, options.ConnectSources...)
	if options.WujieCSPEnabled {
		frameSources = append([]string{"'self'"}, frameSources...)
	}
	frameSource := "'none'"
	if len(frameSources) > 0 {
		frameSource = strings.Join(frameSources, " ")
	}
	directives := []string{
		"default-src 'self'",
		"base-uri 'self'",
		"connect-src " + strings.Join(connectSources, " "),
		"font-src 'self' data:",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"frame-src " + frameSource,
		"img-src 'self' data:",
		"manifest-src 'self'",
		"media-src 'self'",
		"object-src 'none'",
		"worker-src 'self'",
	}
	if options.WujieCSPEnabled {
		directives = append(directives,
			"script-src 'self' 'unsafe-inline'",
			"style-src 'self' 'unsafe-inline'",
			"style-src-attr 'unsafe-inline'",
			"style-src-elem 'self' 'unsafe-inline'",
		)
	} else {
		directives = append(directives,
			"script-src 'self'",
			"style-src 'self' 'unsafe-inline'",
			"style-src-attr 'unsafe-inline'",
			"style-src-elem 'self' 'unsafe-inline'",
		)
	}
	return strings.Join(directives, "; ")
}

func setStaticCacheControl(header http.Header, cleanPath string) {
	if cleanPath == "runtime-config.js" || strings.HasSuffix(cleanPath, "manifest.bundle.json") || strings.Contains(cleanPath, "manifest-") {
		header.Set("Cache-Control", "no-store")
		return
	}
	if strings.HasPrefix(filepath.ToSlash(cleanPath), "assets/") {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	header.Set("Cache-Control", "public, max-age=300")
}
