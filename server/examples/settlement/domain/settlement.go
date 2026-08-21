// Package domain contains the infrastructure-independent settlement model used
// by the split-service reference implementation.
package domain

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("settlement not found")

type Status string

const (
	StatusPending Status = "PENDING"
	StatusSettled Status = "SETTLED"
	StatusFailed  Status = "FAILED"
)

type Settlement struct {
	ID             string
	OrganizationID string
	Status         Status
	Currency       string
	AmountMinor    int64
	Version        uint64
	UpdatedAt      time.Time
}

type Reader interface {
	Get(context.Context, string, string) (Settlement, error)
}
