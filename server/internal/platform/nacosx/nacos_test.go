package nacosx

import (
	"strings"
	"testing"
)

func TestBuildTLS(t *testing.T) {
	cc, servers, err := Build(ClientSettings{
		Servers:       []string{"https://nacos.internal.example:8848"},
		TLSRequired:   true,
		TLSCAFile:     "/certs/ca.pem",
		TLSCertFile:   "/certs/client.pem",
		TLSKeyFile:    "/certs/client-key.pem",
		TLSServerName: "nacos.internal.example",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(servers) != 1 || servers[0].Scheme != "https" {
		t.Fatalf("unexpected servers: %#v", servers)
	}
	if !cc.TLSCfg.Appointed || !cc.TLSCfg.Enable || cc.TLSCfg.TrustAll {
		t.Fatalf("unsafe TLS config: %#v", cc.TLSCfg)
	}
	if cc.TLSCfg.CaFile != "/certs/ca.pem" || cc.TLSCfg.CertFile != "/certs/client.pem" || cc.TLSCfg.KeyFile != "/certs/client-key.pem" || cc.TLSCfg.ServerNameOverride != "nacos.internal.example" {
		t.Fatalf("TLS files not mapped: %#v", cc.TLSCfg)
	}
}

func TestBuildRejectsUnsafeServerConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		settings ClientSettings
		contains string
	}{
		{name: "required TLS with HTTP", settings: ClientSettings{Servers: []string{"http://nacos:8848"}, TLSRequired: true}, contains: "TLS is required"},
		{name: "required TLS with implicit HTTP", settings: ClientSettings{Servers: []string{"nacos:8848"}, TLSRequired: true}, contains: "TLS is required"},
		{name: "mixed schemes", settings: ClientSettings{Servers: []string{"https://nacos-a:8848", "http://nacos-b:8848"}}, contains: "must not mix"},
		{name: "unsupported scheme", settings: ClientSettings{Servers: []string{"ftp://nacos:8848"}}, contains: "unsupported scheme"},
		{name: "incomplete mTLS", settings: ClientSettings{Servers: []string{"https://nacos:8848"}, TLSCertFile: "/certs/client.pem"}, contains: "configured together"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Build(tt.settings)
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("Build() error = %v, want substring %q", err, tt.contains)
			}
		})
	}
}
