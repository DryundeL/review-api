package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
)

// editorFakeRepo keeps one review in memory; publish copies the working version like the SQL does.
type editorFakeRepo struct {
	reportFakeRepo
	public    domain.Review
	publishes int
}

func (r *editorFakeRepo) GetForUpdate(context.Context, uuid.UUID) (domain.Review, error) {
	return r.review, nil
}

func (r *editorFakeRepo) SaveDraft(_ context.Context, item domain.Review) error {
	r.review = item
	return nil
}

func (r *editorFakeRepo) Publish(_ context.Context, item domain.Review) error {
	r.review, r.public = item, item
	r.publishes++
	return nil
}

type editorFakeReader struct{ ports.ReviewReader }

func (editorFakeReader) Editor(context.Context, uuid.UUID, uuid.UUID) (ports.EditorReview, error) {
	return ports.EditorReview{}, nil
}

func ptr(v int16) *int16 { return &v }

func TestEditorLifecycle(t *testing.T) {
	ctx := context.Background()
	author := uuid.New()
	item, err := domain.NewReview(author, uuid.New(), domain.Draft{})
	if err != nil {
		t.Fatal(err)
	}
	repo := &editorFakeRepo{}
	repo.review = item
	pub := &reportFakePublisher{}
	save := NewSaveDraftHandler(repo, editorFakeReader{}, reportFakeTx{})
	publish := NewPublishReviewHandler(repo, editorFakeReader{}, reportFakeTx{}, pub)

	if _, err := save.Handle(ctx, item.ID, uuid.New(), 1, domain.Draft{}); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("foreign review must be 404: %v", err)
	}
	if _, err := publish.Handle(ctx, item.ID, uuid.New(), 1); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("foreign publish must be 404: %v", err)
	}
	if _, err := publish.Handle(ctx, item.ID, author, 1); err == nil {
		t.Fatal("empty draft published")
	}

	draft := domain.Draft{Rating: ptr(8), Body: "<p>текст</p>"}
	if _, err := save.Handle(ctx, item.ID, author, 1, draft); err != nil {
		t.Fatal(err)
	}
	if _, err := save.Handle(ctx, item.ID, author, 1, domain.Draft{}); !errors.Is(err, contract.ErrRevisionConflict) {
		t.Fatalf("stale revision must conflict: %v", err)
	}
	if repo.review.Body != "<p>текст</p>" || repo.review.Revision != 2 {
		t.Fatalf("stale save overwrote: %+v", repo.review)
	}
	if _, err := publish.Handle(ctx, item.ID, author, 1); !errors.Is(err, contract.ErrRevisionConflict) {
		t.Fatalf("old revision published: %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := publish.Handle(ctx, item.ID, author, 2); err != nil {
			t.Fatal(err)
		}
	}
	if repo.publishes != 1 || len(pub.events) != 1 || pub.events[0] != contract.EventPublished {
		t.Fatalf("repeat publish must be a no-op: publishes=%d events=%v", repo.publishes, pub.events)
	}

	if _, err := save.Handle(ctx, item.ID, author, 2, domain.Draft{Rating: ptr(3), Body: "<p>правка</p>"}); err != nil {
		t.Fatal(err)
	}
	if repo.public.Body != "<p>текст</p>" || *repo.public.Rating != 8 {
		t.Fatalf("autosave changed the public version: %+v", repo.public)
	}
	if _, err := publish.Handle(ctx, item.ID, author, 3); err != nil {
		t.Fatal(err)
	}
	if repo.public.Body != "<p>правка</p>" || len(pub.events) != 1 {
		t.Fatalf("update must replace content without new events: %+v %v", repo.public, pub.events)
	}
}
