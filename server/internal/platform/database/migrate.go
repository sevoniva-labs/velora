package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/pressly/goose/v3"
	gooselock "github.com/pressly/goose/v3/lock"
)

//go:embed migrations/postgres/*.sql migrations/mysql/*.sql
var migrationFS embed.FS

func Migrate(db *sql.DB, provider string) error {
	return MigrateContext(context.Background(), db, provider)
}

// MigrateContext runs release migrations under a database-native session lock.
// The lock is acquired by goose on the same *sql.Conn used for migrations, so
// a second release job cannot interleave DDL or version-table updates.
func MigrateContext(ctx context.Context, db *sql.DB, provider string) error {
	if db == nil {
		return errors.New("migration database is required")
	}
	dir := "migrations/postgres"
	dialect := goose.DialectPostgres
	if provider == "mysql" || provider == "oceanbase" {
		dir = "migrations/mysql"
		dialect = goose.DialectMySQL
	}
	migrations, err := fs.Sub(migrationFS, dir)
	if err != nil {
		return fmt.Errorf("migration filesystem: %w", err)
	}
	var locker gooselock.SessionLocker
	if dialect == goose.DialectPostgres {
		locker, err = gooselock.NewPostgresSessionLocker(
			gooselock.WithLockTimeout(5, 60),
			gooselock.WithUnlockTimeout(2, 30),
		)
	} else {
		locker = mysqlMigrationLocker{name: "velora_schema_migration"}
	}
	if err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	gooseProvider, err := goose.NewProvider(dialect, db, migrations, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	if _, err := gooseProvider.Up(ctx); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// mysqlMigrationLocker uses GET_LOCK/RELEASE_LOCK on the same connection as
// goose. GET_LOCK is polled so cancellation and deployment timeouts remain
// effective instead of waiting inside the server indefinitely.
type mysqlMigrationLocker struct{ name string }

func (l mysqlMigrationLocker) SessionLock(ctx context.Context, conn *sql.Conn) error {
	for {
		var acquired sql.NullInt64
		if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 1)", l.name).Scan(&acquired); err != nil {
			return fmt.Errorf("acquire mysql migration lock: %w", err)
		}
		if acquired.Valid && acquired.Int64 == 1 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (l mysqlMigrationLocker) SessionUnlock(ctx context.Context, conn *sql.Conn) error {
	var released sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", l.name).Scan(&released); err != nil {
		return fmt.Errorf("release mysql migration lock: %w", err)
	}
	if !released.Valid || released.Int64 != 1 {
		return errors.New("mysql migration lock was not released")
	}
	return nil
}
