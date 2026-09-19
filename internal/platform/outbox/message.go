package outbox

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
)

type Message struct {
	ID            string
	EventName     string
	AggregateType string
	AggregateID   string
	Payload       json.RawMessage
	Status        Status
	CreatedAt     time.Time
	PublishedAt   *time.Time
}

func NewMessage(eventName, aggregateType, aggregateID string, payload any) (Message, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID:            uuid.NewString(),
		EventName:     eventName,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       body,
		Status:        StatusPending,
		CreatedAt:     time.Now().UTC(),
	}, nil
}
