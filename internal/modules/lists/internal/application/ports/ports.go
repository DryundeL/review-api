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

// ResultWriter persists results and template snapshots. Methods run in the caller's
// transaction when the context carries one.
type ResultWriter interface {
	// LockAuthor serializes quota checks of one author until the transaction ends.
	LockAuthor(ctx context.Context, authorID uuid.UUID) error
	CountResults(ctx context.Context, authorID uuid.UUID) (int, error)
	// GetTemplateForShare locks a template that is not deleted against saves; contract.ErrNotFound otherwise.
	GetTemplateForShare(ctx context.Context, id uuid.UUID) (domain.Template, error)
	// TemplateModerated reports whether the template, deleted or not, is hidden by a moderator.
	// It takes a shared lock on the template row, so a concurrent hide is ordered before or after.
	TemplateModerated(ctx context.Context, templateID uuid.UUID) (bool, error)
	// EnsureSnapshot returns the snapshot of the template's current revision, creating it once.
	// The template must be locked by the caller.
	EnsureSnapshot(ctx context.Context, t domain.Template) (domain.Snapshot, error)
	Snapshot(ctx context.Context, versionID uuid.UUID) (domain.Snapshot, error)
	InsertResult(ctx context.Context, r domain.Result) error
	// GetResultForUpdate locks a result that is not deleted; contract.ErrNotFound otherwise.
	GetResultForUpdate(ctx context.Context, id uuid.UUID) (domain.Result, error)
	// UpdateResult writes header fields and, when withEntries is set, replaces the entries.
	UpdateResult(ctx context.Context, r domain.Result, withEntries bool) error
	SoftDeleteResult(ctx context.Context, id uuid.UUID) error
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

// ResultView is a result with its snapshot and the public data of its template.
type ResultView struct {
	domain.Result
	Author    AuthorRef
	UpdatedAt time.Time
	Snapshot  domain.Snapshot
	// TypeCode and TemplateAuthor come from the snapshot, never from the working template.
	TypeCode       string
	TemplateAuthor AuthorRef
	// TemplateModerated: the template is hidden by a moderator; the result is hidden with it.
	TemplateModerated bool
	// TemplatePublic: the template is published, public, not deleted and not moderated,
	// so the result may link to it and offer to create one's own result.
	TemplatePublic bool
}

type ResultReader interface {
	// Result returns a result that is not deleted; contract.ErrNotFound otherwise.
	Result(ctx context.Context, id uuid.UUID) (ResultView, error)
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
