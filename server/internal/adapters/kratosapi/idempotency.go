package kratosapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/proto"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

// idempotent executes a write once per organization, principal, operation and
// request. Clients may provide Idempotency-Key; when omitted we derive a
// deterministic key from the protobuf request so browser retries are still
// safe. Responses are stored only for operations whose response is safe to
// replay; one-time secrets deliberately do not use this helper.
func (s *PortalService) idempotent(ctx context.Context, principal domain.Principal, scope string, request proto.Message, newResponse func() proto.Message, operation func() (proto.Message, error)) (proto.Message, error) {
	return s.idempotentWith(ctx, principal, scope, request, newResponse, operation, nil)
}

// idempotentWith optionally transforms the response before it is persisted.
// This is used for the one-time Casdoor client secret: the first response may
// contain it, while a retry can only replay the non-sensitive resource state.
func (s *PortalService) idempotentWith(ctx context.Context, principal domain.Principal, scope string, request proto.Message, newResponse func() proto.Message, operation func() (proto.Message, error), cacheResponse func(proto.Message) proto.Message) (proto.Message, error) {
	if s.idem == nil {
		return operation()
	}
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return nil, kratoserrors.InternalServer("IDEMPOTENCY_REQUEST_INVALID", "request could not be fingerprinted")
	}
	sum := sha256.Sum256(encoded)
	requestHash := hex.EncodeToString(sum[:])
	key := strings.TrimSpace(requestHeader(ctx, "Idempotency-Key", 128))
	if key == "" {
		key = "auto-" + requestHash
	}
	scope = fmt.Sprintf("%s:%s", scope, principal.UserID)
	begin, err := s.idem.Begin(ctx, principal.OrganizationID, scope, key, requestHash, 24*time.Hour)
	if err != nil {
		return nil, kratoserrors.InternalServer("IDEMPOTENCY_UNAVAILABLE", "write retry protection is unavailable")
	}
	if !begin.Created {
		if begin.Conflict {
			return nil, kratoserrors.Conflict("IDEMPOTENCY_KEY_REUSED", "idempotency key was already used for another request")
		}
		if begin.Record.State != "COMPLETED" {
			return nil, kratoserrors.Conflict("IDEMPOTENCY_IN_PROGRESS", "the same write is already in progress")
		}
		response := newResponse()
		if len(begin.Record.ResponseBody) == 0 {
			return nil, kratoserrors.InternalServer("IDEMPOTENCY_RESPONSE_MISSING", "idempotent response is unavailable")
		}
		if err := proto.Unmarshal(begin.Record.ResponseBody, response); err != nil {
			return nil, kratoserrors.InternalServer("IDEMPOTENCY_RESPONSE_INVALID", "idempotent response is unavailable")
		}
		return response, nil
	}

	response, operationErr := operation()
	if operationErr != nil {
		_ = s.idem.Forget(ctx, begin.Record.ID)
		return nil, operationErr
	}
	if response == nil {
		_ = s.idem.Forget(ctx, begin.Record.ID)
		return nil, errors.New("idempotency: operation returned an empty response")
	}
	responseToCache := response
	if cacheResponse != nil {
		responseToCache = cacheResponse(response)
	}
	responseBody, err := proto.Marshal(responseToCache)
	if err != nil {
		_ = s.idem.Forget(ctx, begin.Record.ID)
		return nil, kratoserrors.InternalServer("IDEMPOTENCY_RESPONSE_INVALID", "write response could not be persisted")
	}
	if err := s.idem.Complete(ctx, begin.Record.ID, 200, responseBody); err != nil {
		return nil, kratoserrors.InternalServer("IDEMPOTENCY_UNAVAILABLE", "write retry protection could not be completed")
	}
	return response, nil
}
