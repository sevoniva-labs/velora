// Package application owns the settlement use cases and depends only on the
// domain port. The same service can be called in-process or through a transport.
package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sevoniva-labs/velora/server/examples/settlement/domain"
)

var (
	ErrInvalidIdentifier = errors.New("invalid settlement identifier")
	ErrScopeViolation    = errors.New("settlement organization scope violation")
)

const maxIdentifierLength = 64

type QueryService struct {
	reader domain.Reader
}

func NewQueryService(reader domain.Reader) (*QueryService, error) {
	if reader == nil {
		return nil, errors.New("settlement reader is required")
	}
	return &QueryService{reader: reader}, nil
}

func (s *QueryService) Get(ctx context.Context, organizationID, settlementID string) (domain.Settlement, error) {
	organizationID = strings.TrimSpace(organizationID)
	settlementID = strings.TrimSpace(settlementID)
	if !validIdentifier(organizationID) || !validIdentifier(settlementID) {
		return domain.Settlement{}, ErrInvalidIdentifier
	}
	settlement, err := s.reader.Get(ctx, organizationID, settlementID)
	if err != nil {
		return domain.Settlement{}, fmt.Errorf("read settlement: %w", err)
	}
	if settlement.OrganizationID != organizationID || settlement.ID != settlementID {
		return domain.Settlement{}, ErrScopeViolation
	}
	return settlement, nil
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > maxIdentifierLength {
		return false
	}
	for i := range len(value) {
		c := value[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		switch c {
		case '-', '_', '.', ':':
			continue
		default:
			return false
		}
	}
	return true
}
