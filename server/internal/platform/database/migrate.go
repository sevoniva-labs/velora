package database

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/postgres/*.sql migrations/mysql/*.sql
var migrationFS embed.FS

func Migrate(db *sql.DB, provider string) error {
	goose.SetBaseFS(migrationFS)
	dir := "migrations/postgres"
	dialect := "postgres"
	if provider == "mysql" || provider == "oceanbase" {
		dir = "migrations/mysql"
		dialect = "mysql"
	}
	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
