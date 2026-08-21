package messaging

import (
	"context"
	"testing"
)

func TestPrepareMessageRejectsReservedIdentityHeader(t *testing.T) {
	_, _, err := prepareMessage(context.Background(), Message{
		ID: "event-1", Topic: "events", Type: "test",
		Headers: map[string]string{HeaderEventID: "spoofed"},
	})
	if err == nil {
		t.Fatal("prepareMessage() accepted a reserved identity header")
	}
}

func TestPrepareMessagePreservesCausalTraceHeader(t *testing.T) {
	_, headers, err := prepareMessage(context.Background(), Message{
		ID: "event-1", OrganizationID: "org-1", Topic: "events", Type: "test",
		Headers: map[string]string{"TraceParent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
	})
	if err != nil {
		t.Fatalf("prepareMessage() error = %v", err)
	}
	if headers["traceparent"] != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
		t.Fatalf("traceparent = %q", headers["traceparent"])
	}
	if headers[HeaderEventID] != "event-1" || headers[HeaderOrganizationID] != "org-1" {
		t.Fatalf("reserved envelope headers = %#v", headers)
	}
}
