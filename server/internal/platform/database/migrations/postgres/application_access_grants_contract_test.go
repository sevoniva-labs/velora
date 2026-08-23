package postgres

import (
	"os"
	"strings"
	"testing"
)

func TestApplicationAccessGrantMigrationKeepsLegacyAuthorizationRecoverable(t *testing.T) {
	data, err := os.ReadFile("00028_application_access_grants.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(data)
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS application_access_grants",
		"CREATE TABLE IF NOT EXISTS application_access_grant_roles",
		"CREATE TABLE IF NOT EXISTS user_application_entitlement_sources",
		"历史直接授权迁移",
		"历史访问策略迁移",
		"ON CONFLICT DO NOTHING",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}
