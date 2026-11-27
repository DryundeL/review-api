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
	LikesCount     int64
	DislikesCount  int64
	ViewsCount     int64
	// ViewerIsLike is filled by GetResultHandler for a signed-in viewer.
	ViewerIsLike *bool
}

type ResultReader interface {
	// Result returns a result that is not deleted; contract.ErrNotFound otherwise.
	Result(ctx context.Context, id uuid.UUID) (ResultView, error)
}

// ResultPageReader reads what the result page shows: the result and the viewer's vote.
type ResultPageReader interface {
	ResultReader
	// ViewerVote is the viewer's vote on the result: true like, false dislike, nil none.
	ViewerVote(ctx context.Context, resultID, viewerID uuid.UUID) (*bool, error)
}

// ReactionWriter stores votes, views and comments of results. Methods must run in the
// caller's transaction.
type ReactionWriter interface {
	// LockOpenResult locks a result that others may see and react to: published, not deleted,
	// neither it nor its template hidden by a moderator. Unlisted results are open too.
	// exclusive takes the row lock that counter updates need; otherwise the lock only keeps the
	// result from being unpublished, deleted or hidden meanwhile. It returns the result's author,
	// or contract.ErrNotFound.
	LockOpenResult(ctx context.Context, id uuid.UUID, exclusive bool) (uuid.UUID, error)
	// UpsertVote sets the user's vote and adjusts the counters. The result must be locked exclusively.
	UpsertVote(ctx context.Context, resultID, userID uuid.UUID, isLike bool) error
	// InsertView records the first view of the user; later views change nothing.
	InsertView(ctx context.Context, resultID, userID uuid.UUID) error
	// CommentExists reports whether the comment belongs to the result.
	CommentExists(ctx context.Context, resultID, commentID uuid.UUID) (bool, error)
	InsertComment(ctx context.Context, c domain.Comment) error
}

// Comment is a comment with its author for the read side.
type Comment struct {
	domain.Comment
	Author     AuthorRef
	HasReplies bool
}

// CommentPosition is a keyset position in a thread, oldest first.
type CommentPosition struct {
	CreatedAt time.Time `json:"t"`
	ID        uuid.UUID `json:"id"`
}

type CommentReader interface {
	// Comment returns a comment of an open result; contract.ErrNotFound otherwise.
	Comment(ctx context.Context, resultID, commentID uuid.UUID) (Comment, error)
	// ListComments lists top-level comments (parentID nil) or the replies to one, oldest first.
	// contract.ErrNotFound when the result is not open or the parent is not its comment.
	ListComments(ctx context.Context, resultID uuid.UUID, parentID *uuid.UUID, after *CommentPosition, limit int) ([]Comment, error)
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

// MineStatus filters the author's own list. Moderation wins over status and visibility:
// a hidden object is only in MineHidden.
type MineStatus string

const (
	MineAll       MineStatus = ""
	MineDraft     MineStatus = "draft"
	MinePublished MineStatus = "published"
	MineUnlisted  MineStatus = "unlisted"
	MineHidden    MineStatus = "hidden"
)

// PageCursor is a keyset position: the sort time (first publication or last update) and the id.
type PageCursor struct {
	T  time.Time
	ID uuid.UUID
}

// ListQuery selects one page of templates or results.
// Without Mine it lists published public objects that are not hidden, newest first publication first;
// AuthorID then narrows it to a profile. With Mine it lists every live object of AuthorID by last update.
type ListQuery struct {
	Mine     bool
	AuthorID uuid.UUID
	Status   MineStatus    // Mine only
	Search   string        // plain text, matched as a substring of the title
	TypeCode string        // empty: any
	Format   domain.Format // empty: any
	After    *PageCursor
	Limit    int
}

// ListCard is one row of a list. Results take type, format and template title from their snapshot.
type ListCard struct {
	ID               uuid.UUID
	Title            string
	TypeCode         string
	Format           domain.Format
	CoverURL         string
	Author           AuthorRef
	Status           domain.Status
	Visibility       domain.Visibility
	Moderated        bool // the object or, for a result, its template is hidden by a moderator
	FirstPublishedAt *time.Time
	UpdatedAt        time.Time
	TemplateID       uuid.UUID // results only
	TemplateTitle    string    // results only
}

type ListReader interface {
	ListTemplates(ctx context.Context, q ListQuery) ([]ListCard, error)
	ListResults(ctx context.Context, q ListQuery) ([]ListCard, error)
}

type ReportTarget string

const (
	TargetTemplate ReportTarget = "template"
	TargetResult   ReportTarget = "result"
)

type ReportStatus string

const (
	ReportOpen     ReportStatus = "open"
	ReportResolved ReportStatus = "resolved"
)

type Report struct {
	ID         uuid.UUID
	Target     ReportTarget
	TargetID   uuid.UUID
	ReporterID uuid.UUID
	ReasonCode string
	Status     ReportStatus
	CreatedAt  time.Time
	ResolvedAt *time.Time
}

// ModerationWriter stores reports and moderator decisions.
type ModerationWriter interface {
	// InsertReport fails with contract.ErrAlreadyReported while the reporter has an open report on the target.
	InsertReport(ctx context.Context, r Report) error
	// ResolveReport closes an open report: contract.ErrNotFound if missing, contract.ErrConflict if already closed.
	ResolveReport(ctx context.Context, id uuid.UUID) (Report, error)
	// SetHidden sets or clears the moderation mark, deleted objects included (a deleted template still hides
	// its results). Hiding keeps the first hiding time. contract.ErrNotFound if the object does not exist.
	SetHidden(ctx context.Context, target ReportTarget, id uuid.UUID, hidden bool) error
}

// ReportCard is an open report for the moderator, with the current state of its target.
type ReportCard struct {
	Report
	TargetTitle    string
	TargetAuthorID uuid.UUID
	TargetHidden   bool
	TargetDeleted  bool
}

type ReportReader interface {
	// OpenReports lists open reports oldest first, after the cursor when it is set.
	OpenReports(ctx context.Context, after *PageCursor, limit int) ([]ReportCard, error)
}

// ReportReasons checks that a report reason is active. contract.FieldError for an unknown or inactive one.
type ReportReasons interface {
	RequireActive(ctx context.Context, code string) error
}
