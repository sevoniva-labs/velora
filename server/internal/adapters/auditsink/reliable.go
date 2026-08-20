package auditsink

import (
	"context"
	"database/sql"
	"errors"

	"github.com/sevoniva-labs/velora/server/internal/app/audit"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/reliablemsg"
)

type ReliableForwarder struct {
	topic string
}

func NewReliableForwarder(topic string) (*ReliableForwarder, error) {
	if topic == "" {
		return nil, errors.New("audit forwarder topic is required")
	}
	return &ReliableForwarder{topic: topic}, nil
}

func (f *ReliableForwarder) Provider() string { return "reliable-message-table" }

func (f *ReliableForwarder) EnqueueTx(ctx context.Context, db *database.DB, tx *sql.Tx, event audit.Event) error {
	_, err := reliablemsg.EnqueueTx(ctx, db, tx, reliablemsg.Event{
		ID:             "audit." + event.ID,
		OrganizationID: event.OrganizationID,
		Topic:          f.topic,
		Key:            event.ID,
		Type:           "audit.event",
		Payload:        event,
		OrderingKey:    event.OrganizationID,
		Headers: map[string]string{
			"x-forge-audit-action": event.Action,
		},
	})
	return err
}
