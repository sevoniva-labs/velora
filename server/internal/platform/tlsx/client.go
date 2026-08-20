// Package tlsx centralizes outbound TLS policy for middleware and service
// clients. It deliberately has no insecure-skip-verify switch: enterprise and
// financial deployments should install the appropriate CA chain instead.
package tlsx

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

type ClientOptions struct {
	Enabled    bool
	CAFile     string
	CertFile   string
	KeyFile    string
	ServerName string
}

func ClientConfig(o ClientOptions) (*tls.Config, error) {
	if !o.Enabled && o.CAFile == "" && o.CertFile == "" && o.KeyFile == "" && o.ServerName == "" {
		return nil, nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: o.ServerName}
	if o.CAFile != "" {
		raw, err := os.ReadFile(o.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read tls ca: %w", err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(raw) {
			return nil, fmt.Errorf("invalid tls ca pem")
		}
		cfg.RootCAs = roots
	}
	if (o.CertFile == "") != (o.KeyFile == "") {
		return nil, fmt.Errorf("tls client cert and key must be configured together")
	}
	if o.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(o.CertFile, o.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load tls client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}
