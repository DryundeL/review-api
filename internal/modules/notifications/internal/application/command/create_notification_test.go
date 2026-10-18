package command

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/notifications/internal/application/ports"
	userscontract "review-api/internal/modules/users/contract"
)

type createFake struct {
	likeEnabled bool
	inserted    bool
	botCalls    int
	muted       bool
}

func (f *createFake) EnsureDefaults(context.Context, uuid.UUID) error { return nil }
func (f *createFake) List(context.Context, uuid.UUID) ([]ports.PrefItem, error) {
	return nil, nil
}
func (f *createFake) Patch(context.Context, uuid.UUID, *bool, *bool) error { return nil }
func (f *createFake) TypeEnabled(_ context.Context, _ uuid.UUID, name string) (bool, error) {
	if name == "like" {
		return f.likeEnabled, nil
	}
	return true, nil
}
func (f *createFake) TypeMeta(_ context.Context, name string) (ports.TypeMeta, error) {
	return ports.TypeMeta{Name: name, Title: "T", Text: "base"}, nil
}
func (f *createFake) Insert(context.Context, ports.Notification) error {
	f.inserted = true
	return nil
}
func (f *createFake) ListInbox(_ context.Context, _ uuid.UUID, _, _ int) ([]ports.Notification, error) {
	return nil, nil
}
func (f *createFake) UnreadCount(context.Context, uuid.UUID) (int64, error) { return 0, nil }
func (f *createFake) MarkRead(context.Context, uuid.UUID, uuid.UUID) error  { return nil }
func (f *createFake) MarkAllRead(context.Context, uuid.UUID) error          { return nil }

func (f *createFake) UserRef(_ context.Context, id uuid.UUID) (userscontract.UserRef, error) {
	return userscontract.UserRef{ID: id, TelegramID: 99, Username: "ann", FirstName: "Ann"}, nil
}
func (f *createFake) NotificationsMutedByUserID(context.Context, uuid.UUID) (bool, error) {
	return f.muted, nil
}
func (f *createFake) SendMessage(context.Context, int64, string, any) error {
	f.botCalls++
	return nil
}

func TestCreateSkipsDisabledType(t *testing.T) {
	t.Parallel()
	f := &createFake{likeEnabled: false}
	h := NewCreateNotificationHandler(f, f, f, f, f, slog.Default())
	err := h.Handle(context.Background(), CreateInput{
		RecipientID: uuid.New(), ActorID: uuid.New(), TypeName: "like",
	})
	if err != nil || f.inserted || f.botCalls != 0 {
		t.Fatalf("inserted=%v bot=%d err=%v", f.inserted, f.botCalls, err)
	}
}

func TestCreateWritesInboxAndBot(t *testing.T) {
	t.Parallel()
	f := &createFake{likeEnabled: true}
	h := NewCreateNotificationHandler(f, f, f, f, f, slog.Default())
	err := h.Handle(context.Background(), CreateInput{
		RecipientID: uuid.New(), ActorID: uuid.New(), TypeName: "like", ReviewTitle: "X",
	})
	if err != nil || !f.inserted || f.botCalls != 1 {
		t.Fatalf("inserted=%v bot=%d err=%v", f.inserted, f.botCalls, err)
	}
}

func TestCreateMuteSkipsBot(t *testing.T) {
	t.Parallel()
	f := &createFake{likeEnabled: true, muted: true}
	h := NewCreateNotificationHandler(f, f, f, f, f, slog.Default())
	_ = h.Handle(context.Background(), CreateInput{
		RecipientID: uuid.New(), ActorID: uuid.New(), TypeName: "like",
	})
	if !f.inserted || f.botCalls != 0 {
		t.Fatalf("inserted=%v bot=%d", f.inserted, f.botCalls)
	}
}
