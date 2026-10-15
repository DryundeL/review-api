package outboxpub

import (
	"context"
	"fmt"

	"review-api/internal/platform/outbox"
)

type Publisher struct {
	writer outbox.Writer
}

func New(writer outbox.Writer) *Publisher {
	return &Publisher{writer: writer}
}

func (p *Publisher) Publish(ctx context.Context, eventName, aggregateID string, payload any) error {
	if p == nil || p.writer == nil {
		return fmt.Errorf("outbox publisher is not configured")
	}
	msg, err := outbox.NewMessage(eventName, "media_object", aggregateID, payload)
	if err != nil {
		return err
	}
	return p.writer.Append(ctx, msg)
}
