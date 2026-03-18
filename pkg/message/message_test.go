package message

import (
	"testing"
	"time"
)

func TestNewSetsCoreFields(t *testing.T) {
	t.Parallel()

	msg := New("general", "alice", "hello")

	if msg.ID == "" {
		t.Fatal("expected generated message ID")
	}
	if msg.Room != "general" {
		t.Fatalf("expected room %q, got %q", "general", msg.Room)
	}
	if msg.From != "alice" {
		t.Fatalf("expected from %q, got %q", "alice", msg.From)
	}
	if msg.Content != "hello" {
		t.Fatalf("expected content %q, got %q", "hello", msg.Content)
	}
	if msg.Timestamp.IsZero() {
		t.Fatal("expected timestamp to be set")
	}
	if time.Since(msg.Timestamp) > time.Second {
		t.Fatalf("expected recent timestamp, got %v", msg.Timestamp)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	original := Message{
		ID:        "123",
		Room:      "general",
		From:      "alice",
		Content:   "hello",
		ReplyTo:   "456",
		Timestamp: time.Unix(1700000000, 123),
	}

	data, err := original.Encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Fatalf("expected ID %q, got %q", original.ID, decoded.ID)
	}
	if decoded.Room != original.Room {
		t.Fatalf("expected room %q, got %q", original.Room, decoded.Room)
	}
	if decoded.From != original.From {
		t.Fatalf("expected from %q, got %q", original.From, decoded.From)
	}
	if decoded.Content != original.Content {
		t.Fatalf("expected content %q, got %q", original.Content, decoded.Content)
	}
	if decoded.ReplyTo != original.ReplyTo {
		t.Fatalf("expected reply_to %q, got %q", original.ReplyTo, decoded.ReplyTo)
	}
	if !decoded.Timestamp.Equal(original.Timestamp) {
		t.Fatalf("expected timestamp %v, got %v", original.Timestamp, decoded.Timestamp)
	}
}

func TestDecodeRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	if _, err := Decode([]byte("not-json")); err == nil {
		t.Fatal("expected decode error for invalid JSON")
	}
}
