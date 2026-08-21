package messaging

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	HeaderEventID        = "x-forge-event-id"
	HeaderEventType      = "x-forge-event-type"
	HeaderOrganizationID = "x-forge-organization-id"
	HeaderKeyEncoding    = "x-forge-key-encoding"

	maxMessageBodyBytes = 4 << 20
	maxHeaderCount      = 64
	maxHeaderBytes      = 32 << 10
)

// Message is the provider-neutral business event envelope. ID is stable across
// retries and must be used by consumers as the idempotency key; it is not the
// provider-assigned message ID.
type Message struct {
	ID             string
	OrganizationID string
	Topic          string
	Key            []byte
	Type           string
	Body           []byte
	Headers        map[string]string
	Tag            string
	OrderingKey    string
	DeliverAt      time.Time
}

type Receipt struct {
	Provider          string
	ProviderMessageID string
}

// Validate checks the provider-neutral envelope before it is persisted. The
// provider repeats validation at publish time because stored data is untrusted.
func Validate(message Message) error {
	_, _, err := prepareMessage(context.Background(), message)
	return err
}

func prepareMessage(ctx context.Context, message Message) (Message, map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return Message{}, nil, err
	}
	message.ID = strings.TrimSpace(message.ID)
	message.OrganizationID = strings.TrimSpace(message.OrganizationID)
	message.Topic = strings.TrimSpace(message.Topic)
	message.Type = strings.TrimSpace(message.Type)
	message.Tag = strings.TrimSpace(message.Tag)
	message.OrderingKey = strings.TrimSpace(message.OrderingKey)
	if message.ID == "" || message.Topic == "" || message.Type == "" {
		return Message{}, nil, fmt.Errorf("messaging: id, topic and type are required")
	}
	if len(message.ID) > 200 || len(message.Topic) > 200 || len(message.Type) > 160 {
		return Message{}, nil, fmt.Errorf("messaging: id, topic or type exceeds its length limit")
	}
	if len(message.Key) > 1024 || len(message.OrderingKey) > 200 || len(message.Tag) > 128 {
		return Message{}, nil, fmt.Errorf("messaging: key, ordering key or tag exceeds its length limit")
	}
	if len(message.Body) > maxMessageBodyBytes {
		return Message{}, nil, fmt.Errorf("messaging: body exceeds %d bytes", maxMessageBodyBytes)
	}
	if len(message.Headers) > maxHeaderCount {
		return Message{}, nil, fmt.Errorf("messaging: more than %d headers", maxHeaderCount)
	}

	headers := make(map[string]string, len(message.Headers)+3)
	totalHeaderBytes := 0
	for rawName, value := range message.Headers {
		name := strings.ToLower(strings.TrimSpace(rawName))
		if !validHeaderName(name) || !utf8.ValidString(value) {
			return Message{}, nil, fmt.Errorf("messaging: invalid header %q", rawName)
		}
		if strings.HasPrefix(name, "x-forge-") {
			return Message{}, nil, fmt.Errorf("messaging: header %q is reserved", rawName)
		}
		totalHeaderBytes += len(name) + len(value)
		if totalHeaderBytes > maxHeaderBytes {
			return Message{}, nil, fmt.Errorf("messaging: headers exceed %d bytes", maxHeaderBytes)
		}
		headers[name] = value
	}
	headers[HeaderEventID] = message.ID
	headers[HeaderEventType] = message.Type
	if message.OrganizationID != "" {
		headers[HeaderOrganizationID] = message.OrganizationID
	}
	if headers["traceparent"] == "" {
		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))
	}
	message.Body = append([]byte(nil), message.Body...)
	message.Key = append([]byte(nil), message.Key...)
	return message, headers, nil
}

func validHeaderName(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			continue
		}
		switch c {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}
