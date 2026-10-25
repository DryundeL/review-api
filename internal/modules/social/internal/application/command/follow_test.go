package command

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"review-api/internal/modules/social/internal/domain"
)

type followTestTx struct{}

func (followTestTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type followTestWriter struct{ status string }

func (f followTestWriter) Create(context.Context, domain.Follow) (string, error) {
	return f.status, nil
}
func (followTestWriter) Approve(context.Context, domain.Follow) error              { return nil }
func (followTestWriter) DeleteRequest(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (followTestWriter) Delete(context.Context, uuid.UUID, uuid.UUID) error        { return nil }

type followTestPublisher struct {
	event   string
	payload map[string]string
	calls   int
}

func (p *followTestPublisher) Publish(_ context.Context, event, aggregate string, payload any) error {
	p.calls++
	p.event = event
	p.payload = payload.(map[string]string)
	return nil
}
func TestFollowPublishesMatchingEvent(t *testing.T) {
	for _, tc := range []struct{ status, event string }{{"pending", "user.follow_requested"}, {"following", "user.followed"}} {
		t.Run(tc.status, func(t *testing.T) {
			publisher := &followTestPublisher{}
			h := NewFollowHandler(followTestWriter{status: tc.status}, followTestTx{}, publisher)
			actor, owner := uuid.New(), uuid.New()
			status, err := h.Handle(context.Background(), actor, owner)
			if err != nil || status != tc.status || publisher.calls != 1 || publisher.event != tc.event {
				t.Fatalf("status=%s event=%s calls=%d err=%v", status, publisher.event, publisher.calls, err)
			}
			if publisher.payload["followerId"] != actor.String() || publisher.payload["followeeId"] != owner.String() {
				t.Fatalf("payload=%v", publisher.payload)
			}
			if status == "pending" {
				if _, err := uuid.Parse(publisher.payload["notificationId"]); err != nil {
					t.Fatal("notification needs a stable ID in event payload")
				}
			}
		})
	}
}
