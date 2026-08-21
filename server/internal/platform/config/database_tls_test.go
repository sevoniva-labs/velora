package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateDatabaseTLS(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		dsn      string
		wantErr  bool
	}{
		{name: "postgres verified URL", provider: "postgres", dsn: "postgres://user:secret@db:5432/app?sslmode=verify-full"},
		{name: "postgres verified keyword", provider: "postgres", dsn: "host=db user=app password=secret sslmode=verify-full"},
		{name: "postgres disabled", provider: "postgres", dsn: "postgres://user:secret@db/app?sslmode=disable", wantErr: true},
		{name: "postgres encryption without identity", provider: "postgres", dsn: "postgres://user:secret@db/app?sslmode=require", wantErr: true},
		{name: "postgres CA without hostname", provider: "postgres", dsn: "postgres://user:secret@db/app?sslmode=verify-ca", wantErr: true},
		{name: "postgres missing mode", provider: "postgres", dsn: "postgres://user:secret@db/app", wantErr: true},
		{name: "mysql verified", provider: "mysql", dsn: "user:secret@tcp(db:3306)/app?parseTime=true&tls=true"},
		{name: "oceanbase verified", provider: "oceanbase", dsn: "user:secret@tcp(db:2881)/app?tls=true"},
		{name: "mysql preferred downgrade", provider: "mysql", dsn: "user:secret@tcp(db:3306)/app?tls=preferred", wantErr: true},
		{name: "mysql skip verification", provider: "mysql", dsn: "user:secret@tcp(db:3306)/app?tls=skip-verify", wantErr: true},
		{name: "mysql missing TLS", provider: "mysql", dsn: "user:secret@tcp(db:3306)/app?parseTime=true", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDatabaseTLS(tt.provider, tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDatabaseTLS() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDatabaseRequiresVerifiedTLSInProduction(t *testing.T) {
	base := Config{
		App: App{Environment: "production"},
		Database: Database{
			Provider: "postgres", DSN: "postgres://user:secret@db/app?sslmode=verify-full",
			ReadOnlyDSN:  "postgres://user:secret@db-replica/app?sslmode=verify-full",
			MaxOpenConns: 10, MaxIdleConns: 5, MaxLifetime: time.Minute,
		},
	}
	if err := base.ValidateDatabase(); err != nil {
		t.Fatalf("verified production DSNs rejected: %v", err)
	}
	base.Database.ReadOnlyDSN = "postgres://user:secret@db-replica/app?sslmode=disable"
	if err := base.ValidateDatabase(); err == nil || !strings.Contains(err.Error(), "read-only database DSN") {
		t.Fatalf("unsafe replica DSN should be rejected, got %v", err)
	}
	base.App.Environment = "development"
	base.Database.DSN = "postgres://user:secret@db/app?sslmode=disable"
	base.Database.ReadOnlyDSN = ""
	if err := base.ValidateDatabase(); err != nil {
		t.Fatalf("development DSN unexpectedly rejected: %v", err)
	}
}
