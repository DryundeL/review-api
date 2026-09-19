package outbox

import (
	"encoding/json"
	"testing"
)

func TestNewMessage(t *testing.T) {
	msg, err := NewMessage("review.published", "review", "1", map[string]string{"id": "1"})
	if err != nil {
		t.Fatal(err)
	}
	if msg.EventName != "review.published" || msg.Status != StatusPending || msg.ID == "" {
		t.Fatalf("%+v", msg)
	}
	var payload map[string]string
	if err := json.Unmarshal(msg.Payload, &payload); err != nil || payload["id"] != "1" {
		t.Fatalf("payload %s", msg.Payload)
	}
}
