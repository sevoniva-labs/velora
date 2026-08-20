package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
)

type DB struct {
	*sql.DB
	Provider string
}

type transactionContextKey struct{}

func Open(ctx context.Context, cfg config.Database) (*DB, error) {
	driver := "pgx"
	if cfg.Provider == "mysql" || cfg.Provider == "oceanbase" {
		driver = "mysql"
	}
	raw, err := sql.Open(driver, cfg.DSN)
	if err != nil {
		return nil, err
	}
	raw.SetMaxOpenConns(cfg.MaxOpenConns)
	raw.SetMaxIdleConns(cfg.MaxIdleConns)
	raw.SetConnMaxLifetime(cfg.MaxLifetime)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := raw.PingContext(pingCtx); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("database ping: %w", err)
	}
	return &DB{DB: raw, Provider: cfg.Provider}, nil
}

func (db *DB) Rebind(query string) string {
	if db.Provider != "postgres" {
		return query
	}
	var b strings.Builder
	n := 1
	for _, r := range query {
		if r == '?' {
			fmt.Fprintf(&b, "$%d", n)
			n++
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (db *DB) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	if tx := transactionFromContext(ctx); tx != nil {
		return fn(tx)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// WithinTx propagates one local transaction through context so repositories
// and reliable side effects such as audit records commit or roll back together.
func (db *DB) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if transactionFromContext(ctx) != nil {
		return fn(ctx)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	txCtx := context.WithValue(ctx, transactionContextKey{}, tx)
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if tx := transactionFromContext(ctx); tx != nil {
		return tx.ExecContext(ctx, query, args...)
	}
	return db.DB.ExecContext(ctx, query, args...)
}

func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if tx := transactionFromContext(ctx); tx != nil {
		return tx.QueryContext(ctx, query, args...)
	}
	return db.DB.QueryContext(ctx, query, args...)
}

func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if tx := transactionFromContext(ctx); tx != nil {
		return tx.QueryRowContext(ctx, query, args...)
	}
	return db.DB.QueryRowContext(ctx, query, args...)
}

func transactionFromContext(ctx context.Context) *sql.Tx {
	tx, _ := ctx.Value(transactionContextKey{}).(*sql.Tx)
	return tx
}

// TransactionFromContext exposes the current transaction to infrastructure
// adapters that must enqueue a reliable side effect atomically.
func TransactionFromContext(ctx context.Context) (*sql.Tx, bool) {
	tx := transactionFromContext(ctx)
	return tx, tx != nil
}
