// Package repository provides the removable development adapter for the
// settlement split-service example.
package repository

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/sevoniva-labs/velora/server/examples/settlement/domain"
)

type Memory struct {
	mu    sync.RWMutex
	items map[string]domain.Settlement
}

func NewMemory(items ...domain.Settlement) (*Memory, error) {
	repository := &Memory{items: make(map[string]domain.Settlement, len(items))}
	for _, item := range items {
		if strings.TrimSpace(item.OrganizationID) == "" || strings.TrimSpace(item.ID) == "" {
			return nil, errors.New("settlement organization and ID are required")
		}
		key := item.OrganizationID + "/" + item.ID
		if _, exists := repository.items[key]; exists {
			return nil, errors.New("duplicate settlement")
		}
		repository.items[key] = item
	}
	return repository, nil
}

func (r *Memory) Get(ctx context.Context, organizationID, settlementID string) (domain.Settlement, error) {
	if err := ctx.Err(); err != nil {
		return domain.Settlement{}, err
	}
	r.mu.RLock()
	item, ok := r.items[organizationID+"/"+settlementID]
	r.mu.RUnlock()
	if !ok {
		return domain.Settlement{}, domain.ErrNotFound
	}
	return item, nil
}
