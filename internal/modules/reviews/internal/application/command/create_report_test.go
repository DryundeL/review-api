package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
)

type reportFakeTx struct{}

func (reportFakeTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type reportFakeReasons struct {
	active map[string]bool
}

func (f reportFakeReasons) ListActive(context.Context) ([]ports.Reason, error) {
	return nil, nil
}

func (f reportFakeReasons) IsActive(_ context.Context, code string) (bool, error) {
	return f.active[code], nil
}

type reportFakePublisher struct {
	events []string
}

func (p *reportFakePublisher) Publish(_ context.Context, eventName, _ string, _ any) error {
	p.events = append(p.events, eventName)
	return nil
}

type reportFakeRepo struct {
	review      domain.Review
	reports     []ports.Report
	insertErr   error
	unpublished bool
}

func (r *reportFakeRepo) Create(context.Context, domain.Review) error  { return nil }
func (r *reportFakeRepo) Replace(context.Context, domain.Review) error { return nil }
func (r *reportFakeRepo) Delete(context.Context, uuid.UUID) error      { return nil }
func (r *reportFakeRepo) Get(context.Context, uuid.UUID) (domain.Review, error) {
	return r.review, nil
}
func (r *reportFakeRepo) GetForUpdate(context.Context, uuid.UUID) (domain.Review, error) {
	if r.review.ID == uuid.Nil {
		return domain.Review{}, contract.ErrNotFound
	}
	return r.review, nil
}
func (r *reportFakeRepo) UpsertVote(context.Context, uuid.UUID, uuid.UUID, bool) error { return nil }
func (r *reportFakeRepo) InsertView(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (r *reportFakeRepo) InsertReport(_ context.Context, report ports.Report) error {
	if r.insertErr != nil {
		return r.insertErr
	}
	for _, existing := range r.reports {
		if existing.ReviewID == report.ReviewID && existing.ReporterID == report.ReporterID {
			return contract.ErrAlreadyReported
		}
	}
	r.reports = append(r.reports, report)
	return nil
}
func (r *reportFakeRepo) CountDistinctReporters(_ context.Context, reviewID uuid.UUID) (int, error) {
	seen := map[uuid.UUID]struct{}{}
	for _, report := range r.reports {
		if report.ReviewID == reviewID {
			seen[report.ReporterID] = struct{}{}
		}
	}
	return len(seen), nil
}
func (r *reportFakeRepo) Unpublish(_ context.Context, reviewID uuid.UUID) error {
	if r.review.ID != reviewID || r.review.Status != domain.StatusPublished {
		return contract.ErrNotFound
	}
	r.review.Status = domain.StatusDraft
	r.unpublished = true
	return nil
}

func newReportHandler(repo *reportFakeRepo, threshold int, pub *reportFakePublisher) *CreateReportHandler {
	return NewCreateReportHandler(
		repo,
		reportFakeReasons{active: map[string]bool{"spam": true}},
		reportFakeTx{},
		pub,
		threshold,
	)
}

func TestCreateReportUnpublishesAtThreshold(t *testing.T) {
	t.Parallel()
	authorID := uuid.New()
	reviewID := uuid.New()
	repo := &reportFakeRepo{review: domain.Review{
		ID: reviewID, AuthorID: authorID, MediaObjectID: uuid.New(), Status: domain.StatusPublished,
	}}
	pub := &reportFakePublisher{}
	h := newReportHandler(repo, 2, pub)

	if _, err := h.Handle(context.Background(), reviewID, uuid.New(), "spam"); err != nil {
		t.Fatal(err)
	}
	if repo.unpublished || len(pub.events) != 0 {
		t.Fatal("should not unpublish below threshold")
	}
	if _, err := h.Handle(context.Background(), reviewID, uuid.New(), "spam"); err != nil {
		t.Fatal(err)
	}
	if !repo.unpublished || len(pub.events) != 1 || pub.events[0] != contract.EventUnpublished {
		t.Fatalf("unpublished=%v events=%v", repo.unpublished, pub.events)
	}
}

func TestCreateReportRejectsDuplicate(t *testing.T) {
	t.Parallel()
	reviewID := uuid.New()
	reporterID := uuid.New()
	repo := &reportFakeRepo{review: domain.Review{
		ID: reviewID, AuthorID: uuid.New(), Status: domain.StatusPublished,
	}}
	h := newReportHandler(repo, 10, &reportFakePublisher{})
	if _, err := h.Handle(context.Background(), reviewID, reporterID, "spam"); err != nil {
		t.Fatal(err)
	}
	_, err := h.Handle(context.Background(), reviewID, reporterID, "spam")
	if !errors.Is(err, contract.ErrAlreadyReported) {
		t.Fatal(err)
	}
}

func TestCreateReportRejectsOwnReview(t *testing.T) {
	t.Parallel()
	authorID := uuid.New()
	reviewID := uuid.New()
	repo := &reportFakeRepo{review: domain.Review{
		ID: reviewID, AuthorID: authorID, Status: domain.StatusPublished,
	}}
	h := newReportHandler(repo, 10, &reportFakePublisher{})
	_, err := h.Handle(context.Background(), reviewID, authorID, "spam")
	if !errors.Is(err, contract.ErrForbidden) {
		t.Fatal(err)
	}
}

func TestCreateReportRejectsDraft(t *testing.T) {
	t.Parallel()
	reviewID := uuid.New()
	repo := &reportFakeRepo{review: domain.Review{
		ID: reviewID, AuthorID: uuid.New(), Status: domain.StatusDraft,
	}}
	h := newReportHandler(repo, 10, &reportFakePublisher{})
	_, err := h.Handle(context.Background(), reviewID, uuid.New(), "spam")
	if !errors.Is(err, contract.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestCreateReportRejectsInactiveReason(t *testing.T) {
	t.Parallel()
	reviewID := uuid.New()
	repo := &reportFakeRepo{review: domain.Review{
		ID: reviewID, AuthorID: uuid.New(), Status: domain.StatusPublished,
	}}
	h := newReportHandler(repo, 10, &reportFakePublisher{})
	_, err := h.Handle(context.Background(), reviewID, uuid.New(), "unknown")
	var field contract.FieldError
	if !errors.As(err, &field) || field.Field != "reasonCode" {
		t.Fatal(err)
	}
	if len(repo.reports) != 0 {
		t.Fatal("inserted before validation")
	}
}

func TestCreateReportSetsCreatedAt(t *testing.T) {
	t.Parallel()
	reviewID := uuid.New()
	repo := &reportFakeRepo{review: domain.Review{
		ID: reviewID, AuthorID: uuid.New(), Status: domain.StatusPublished,
	}}
	h := newReportHandler(repo, 10, &reportFakePublisher{})
	before := time.Now().UTC().Add(-time.Second)
	got, err := h.Handle(context.Background(), reviewID, uuid.New(), "spam")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == uuid.Nil || got.CreatedAt.Before(before) {
		t.Fatalf("report = %+v", got)
	}
}
