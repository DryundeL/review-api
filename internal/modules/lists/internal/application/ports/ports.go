package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/internal/domain"
)

// TemplateWriter persists the working state of templates. Methods run in the caller's
// transaction when the context carries one.
type TemplateWriter interface {
	// LockAuthor serializes quota checks of one author until the transaction ends.
	LockAuthor(ctx context.Context, authorID uuid.UUID) error
	CountTemplates(ctx context.Context, authorID uuid.UUID) (int, error)
	InsertTemplate(ctx context.Context, t domain.Template) error
	// GetTemplateForUpdate locks a template that is not deleted; contract.ErrNotFound otherwise.
	GetTemplateForUpdate(ctx context.Context, id uuid.UUID) (domain.Template, error)
	// UpdateTemplate writes header fields and, when withContent is set, replaces tiers and items.
	UpdateTemplate(ctx context.Context, t domain.Template, withContent bool) error
	SoftDeleteTemplate(ctx context.Context, id uuid.UUID) error
}

// AuthorRef is the public part of a user shown next to a template or result.
type AuthorRef struct {
	ID       uuid.UUID
	Username string
	PhotoURL string
}

// TemplateView is a template with its author for the read side.
type TemplateView struct {
	domain.Template
	Author    AuthorRef
	UpdatedAt time.Time
}

type TemplateReader interface {
	// Template returns a template that is not deleted; contract.ErrNotFound otherwise.
	Template(ctx context.Context, id uuid.UUID) (TemplateView, error)
}

// CatalogWork is the catalog data copied into a template item.
type CatalogWork struct {
	ID        uuid.UUID
	Type      string
	Title     string
	PosterURL string
}

// Catalog resolves catalog works. contract.ErrNotFound when the work does not exist.
type Catalog interface {
	Work(ctx context.Context, id uuid.UUID) (CatalogWork, error)
}

// Images attaches the author's uploads and returns their URLs by id. It must run in the
// caller's transaction. contract.ErrNotFound when any id is missing or foreign.
type Images interface {
	Attach(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// Types checks that a review type is active. contract.FieldError for an unknown or inactive type.
type Types interface {
	RequireActive(ctx context.Context, code string) error
}
