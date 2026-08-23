package velora

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const MaxProvisioningBody = 64 << 10

type ProvisioningEvent struct {
	SchemaVersion    string           `json:"schema_version"`
	EventID          string           `json:"event_id"`
	EventType        string           `json:"event_type"`
	AggregateVersion int64            `json:"aggregate_version"`
	OccurredAt       time.Time        `json:"occurred_at"`
	Source           string           `json:"source"`
	User             *ProvisionedUser `json:"user,omitempty"`
	Entitlements     *Entitlements    `json:"entitlements,omitempty"`
	Challenge        *Challenge       `json:"challenge,omitempty"`
}

type Challenge struct {
	ApplicationCode string `json:"application_code"`
	ChallengeID     string `json:"challenge_id"`
}
type ProvisionedUser struct {
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer"`
	LoginName   string `json:"login_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Status      string `json:"status"`
}
type Entitlements struct {
	ApplicationCode string   `json:"application_code"`
	Roles           []string `json:"roles"`
}

type ApplyResult string

const (
	Applied   ApplyResult = "APPLIED"
	Duplicate ApplyResult = "DUPLICATE"
	Stale     ApplyResult = "STALE"
)

type ProvisioningStore interface {
	Apply(context.Context, ProvisioningEvent) (ApplyResult, error)
}

type ProvisioningHandler struct {
	secret []byte
	store  ProvisioningStore
	now    func() time.Time
}

func NewProvisioningHandler(secret string, store ProvisioningStore) (*ProvisioningHandler, error) {
	if len(strings.TrimSpace(secret)) < 32 || store == nil {
		return nil, errors.New("provisioning secret of at least 32 bytes and a store are required")
	}
	return &ProvisioningHandler{secret: []byte(strings.TrimSpace(secret)), store: store, now: time.Now}, nil
}

func (h *ProvisioningHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		writeResult(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	timestamp, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get("X-Velora-Timestamp")), 10, 64)
	if err != nil || h.now().Sub(time.Unix(timestamp, 0)).Abs() > 5*time.Minute {
		writeResult(w, http.StatusUnauthorized, "TIMESTAMP_INVALID")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxProvisioningBody))
	if err != nil {
		writeResult(w, http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE")
		return
	}
	mac := hmac.New(sha256.New, h.secret)
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "."))
	_, _ = mac.Write(raw)
	provided, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(r.Header.Get("X-Velora-Signature")), "v1="))
	if err != nil || len(provided) != sha256.Size || subtle.ConstantTimeCompare(provided, mac.Sum(nil)) != 1 {
		writeResult(w, http.StatusUnauthorized, "SIGNATURE_INVALID")
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var event ProvisioningEvent
	if err := decoder.Decode(&event); err != nil || decoder.Decode(&struct{}{}) != io.EOF || event.SchemaVersion != "1.0" || event.EventID == "" || event.EventType == "" || event.AggregateVersion < 1 || event.Source != "velora" {
		writeResult(w, http.StatusBadRequest, "EVENT_INVALID")
		return
	}
	if event.EventType == "integration.challenge" && (event.Challenge == nil || event.Challenge.ApplicationCode == "" || event.Challenge.ChallengeID == "") {
		writeResult(w, http.StatusBadRequest, "CHALLENGE_INVALID")
		return
	}
	if event.EventType != "integration.challenge" && (event.User == nil || event.Entitlements == nil || event.User.Subject == "" || event.Entitlements.ApplicationCode == "" || (event.User.Status != "ACTIVE" && event.User.Status != "DISABLED") || (event.User.Status == "DISABLED" && len(event.Entitlements.Roles) != 0)) {
		writeResult(w, http.StatusBadRequest, "ENTITLEMENT_INVALID")
		return
	}
	result, err := h.store.Apply(r.Context(), event)
	if err != nil {
		writeResult(w, http.StatusInternalServerError, "APPLY_FAILED")
		return
	}
	if result != Applied && result != Duplicate && result != Stale {
		writeResult(w, http.StatusInternalServerError, "RESULT_INVALID")
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status":%q}`, result)
}

func writeResult(w http.ResponseWriter, status int, code string) {
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"status":"REJECTED","error_code":%q}`, code)
}

type MemoryProvisioningStore struct {
	mu       sync.Mutex
	events   map[string][32]byte
	versions map[string]int64
}

func NewMemoryProvisioningStore() *MemoryProvisioningStore {
	return &MemoryProvisioningStore{events: map[string][32]byte{}, versions: map[string]int64{}}
}
func (s *MemoryProvisioningStore) Apply(_ context.Context, event ProvisioningEvent) (ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := json.Marshal(event)
	digest := sha256.Sum256(raw)
	if existing, ok := s.events[event.EventID]; ok {
		if subtle.ConstantTimeCompare(existing[:], digest[:]) == 1 {
			return Duplicate, nil
		}
		return "", errors.New("event id reused with different body")
	}
	key := event.EventType
	if event.Challenge != nil {
		key = event.EventType + ":" + event.Challenge.ChallengeID
	} else if event.User != nil && event.Entitlements != nil {
		key = event.User.Subject + ":" + event.Entitlements.ApplicationCode
	}
	if event.AggregateVersion < s.versions[key] {
		s.events[event.EventID] = digest
		return Stale, nil
	}
	s.events[event.EventID] = digest
	s.versions[key] = event.AggregateVersion
	return Applied, nil
}
