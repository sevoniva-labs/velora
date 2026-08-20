package messageworker

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/messaging"
)

var ErrEventIdentityConflict = errors.New("consumed event identity conflict")

type transactioner interface {
	WithTx(context.Context, func(*sql.Tx) error) error
}

type consumptionStore interface {
	Seen(context.Context, *sql.Tx, string, string, string) (bool, error)
	Mark(context.Context, *sql.Tx, string, messaging.Delivery, string) error
}

type sqlConsumptionStore struct {
	db *database.DB
}

func (s *sqlConsumptionStore) Seen(ctx context.Context, tx *sql.Tx, group, eventID, bodyHash string) (bool, error) {
	var existingHash string
	err := tx.QueryRowContext(ctx, s.db.Rebind(`SELECT body_hash FROM consumed_messages WHERE consumer_group=? AND event_id=?`), group, eventID).Scan(&existingHash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if existingHash != bodyHash {
		return false, fmt.Errorf("%w: group=%q event_id=%q", ErrEventIdentityConflict, group, eventID)
	}
	return true, nil
}

func (s *sqlConsumptionStore) Mark(ctx context.Context, tx *sql.Tx, group string, delivery messaging.Delivery, bodyHash string) error {
	message := delivery.Message
	_, err := tx.ExecContext(ctx, s.db.Rebind(`INSERT INTO consumed_messages(consumer_group,event_id,organization_id,topic,event_type,body_hash,provider_message_id,consumed_at) VALUES(?,?,?,?,?,?,?,?)`),
		group, message.ID, nullIfEmpty(message.OrganizationID), message.Topic, message.Type, bodyHash, delivery.ProviderMessageID, time.Now().UTC())
	return err
}

func messageHash(message messaging.Message) string {
	digest := sha256.New()
	writeHashPart(digest, []byte(message.OrganizationID))
	writeHashPart(digest, []byte(message.Topic))
	writeHashPart(digest, []byte(message.Type))
	writeHashPart(digest, message.Key)
	writeHashPart(digest, []byte(message.OrderingKey))
	writeHashPart(digest, []byte(message.Tag))
	writeHashPart(digest, message.Body)
	return hex.EncodeToString(digest.Sum(nil))
}

func writeHashPart(digest hash.Hash, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = digest.Write(size[:])
	_, _ = digest.Write(value)
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
