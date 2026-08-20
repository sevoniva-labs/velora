package main

import "testing"

func TestLoopbackAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:18080", "localhost:18080", "[::1]:18080"} {
		if !loopbackAddress(address) {
			t.Fatalf("loopbackAddress(%q) = false", address)
		}
	}
	for _, address := range []string{":18080", "0.0.0.0:18080", "10.0.0.2:18080", "invalid"} {
		if loopbackAddress(address) {
			t.Fatalf("loopbackAddress(%q) = true", address)
		}
	}
}

func TestLoadServerTLSRejectsInsecureExposure(t *testing.T) {
	if _, err := loadServerTLS(runtimeConfig{HTTPAddress: "0.0.0.0:18080", GRPCAddress: "0.0.0.0:19090"}); err == nil {
		t.Fatal("loadServerTLS() accepted non-loopback plaintext listeners")
	}
	if _, err := loadServerTLS(runtimeConfig{
		HTTPAddress: "127.0.0.1:18080", GRPCAddress: "127.0.0.1:19090", TLSCertFile: "only-cert.pem",
	}); err == nil {
		t.Fatal("loadServerTLS() accepted a partial mTLS configuration")
	}
	if tlsConfig, err := loadServerTLS(runtimeConfig{
		HTTPAddress: "127.0.0.1:18080", GRPCAddress: "127.0.0.1:19090",
	}); err != nil || tlsConfig != nil {
		t.Fatalf("loadServerTLS() = (%v, %v), want nil development TLS", tlsConfig, err)
	}
}
