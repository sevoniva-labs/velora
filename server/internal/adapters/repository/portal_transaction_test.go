package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
)

// TestPortalCreateApplicationWithinTransaction can be run by the PostgreSQL
// smoke gate with VELORA_TEST_POSTGRES_DSN. It protects the transaction path
// used by audited API mutations; PostgreSQL does not allow relation queries
// while the base application cursor is still open on the same connection.
func TestPortalCreateApplicationWithinTransaction(t *testing.T) {
	dsn := os.Getenv("VELORA_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("VELORA_TEST_POSTGRES_DSN is not set")
	}
	cfg := config.Default()
	cfg.Database.Provider = "postgres"
	cfg.Database.DSN = dsn
	db, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var orgID, userID, categoryID string
	if err := db.QueryRow(`SELECT organization_id,id FROM users ORDER BY created_at LIMIT 1`).Scan(&orgID, &userID); err != nil {
		t.Skipf("bootstrap user is unavailable: %v", err)
	}
	if err := db.QueryRow(`SELECT id FROM portal_categories WHERE organization_id=$1 ORDER BY created_at LIMIT 1`, orgID).Scan(&categoryID); err != nil {
		t.Skipf("portal category is unavailable: %v", err)
	}
	repo := NewPortalRepo(db)
	err = db.WithinTx(context.Background(), func(txCtx context.Context) error {
		_, createErr := repo.CreateApplication(txCtx, orgID, userID, ApplicationInput{
			Code: "tx-" + uuid.NewString(), Name: "Transaction smoke", CategoryID: categoryID,
			HomeURL: "https://ledger.example.test", LaunchURL: "https://ledger.example.test/login",
			LaunchType: "URL", Status: portaldomain.StatusEnabled,
		})
		return createErr
	})
	if err != nil {
		t.Fatalf("create application inside transaction: %v", err)
	}
}
