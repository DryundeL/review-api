package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type PrefItem struct {
	Name    string
	Title   string
	Text    string
	Enabled bool
}

type PrefPatch struct {
	Name    string
	Enabled bool
}

type PrefRepository interface {
	EnsureDefaults(ctx context.Context, userID uuid.UUID) error
	List(ctx context.Context, userID uuid.UUID) ([]PrefItem, error)
	Patch(ctx context.Context, userID uuid.UUID, like, follow, followRequest *bool) error
	TypeEnabled(ctx context.Context, userID uuid.UUID, name string) (bool, error)
	TypeMeta(ctx context.Context, name string) (TypeMeta, error)
}

type TypeMeta struct {
	Name  string
	Title string
	Text  string
}

type Notification struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TypeName  string
	Title     string
	Body      string
	ActorID   *uuid.UUID
	ReviewID  *uuid.UUID
	ReadAt    *time.Time
	CreatedAt time.Time
}

type InboxRepository interface {
	// Insert returns false when the notification ID already exists.
	Insert(ctx context.Context, item Notification) (bool, error)
	ListInbox(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Notification, error)
	UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)
	MarkRead(ctx context.Context, userID, id uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
}
