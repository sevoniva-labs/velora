package database

import (
	"context"
	"database/sql"
	"testing"
)

func TestRebind(t *testing.T) {
	pg := &DB{Provider: "postgres"}
	got := pg.Rebind("SELECT * FROM users WHERE id=? AND organization_id=?")
	want := "SELECT * FROM users WHERE id=$1 AND organization_id=$2"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	mysql := &DB{Provider: "mysql"}
	q := "SELECT * FROM users WHERE id=?"
	if got := mysql.Rebind(q); got != q {
		t.Fatalf("mysql query changed: %q", got)
	}
}

func TestWithTxReusesContextTransaction(t *testing.T) {
	db := &DB{}
	want := &sql.Tx{}
	ctx := context.WithValue(context.Background(), transactionContextKey{}, want)
	called := false
	if err := db.WithTx(ctx, func(got *sql.Tx) error {
		called = true
		if got != want {
			t.Fatalf("WithTx transaction = %p, want %p", got, want)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithTx() error = %v", err)
	}
	if !called {
		t.Fatal("WithTx callback was not called")
	}
}

func TestWithinTxReusesContextTransaction(t *testing.T) {
	db := &DB{}
	want := &sql.Tx{}
	ctx := context.WithValue(context.Background(), transactionContextKey{}, want)
	called := false
	if err := db.WithinTx(ctx, func(got context.Context) error {
		called = true
		if tx := transactionFromContext(got); tx != want {
			t.Fatalf("WithinTx transaction = %p, want %p", tx, want)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithinTx() error = %v", err)
	}
	if !called {
		t.Fatal("WithinTx callback was not called")
	}
}
