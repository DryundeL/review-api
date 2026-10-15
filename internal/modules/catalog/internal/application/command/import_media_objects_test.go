package command

import (
	"context"
	"errors"
	"testing"

	"review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/catalog/internal/domain"
)

type fakeTx struct {
	called bool
}

func (f *fakeTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	f.called = true
	return fn(ctx)
}

type fakeWriter struct {
	saved []domain.MediaObject
}

func (f *fakeWriter) Save(_ context.Context, item domain.MediaObject) error {
	f.saved = append(f.saved, item)
	return nil
}

type fakeTypes struct{ err error }

func (f fakeTypes) RequireActive(context.Context, []string) error { return f.err }

type fakePublisher struct{ count int }

func (f *fakePublisher) Publish(context.Context, string, string, any) error {
	f.count++
	return nil
}

func TestImportPublishesInTransaction(t *testing.T) {
	t.Parallel()
	writer := &fakeWriter{}
	tx := &fakeTx{}
	pub := &fakePublisher{}
	h := NewImportMediaObjectsHandler(writer, tx, fakeTypes{}, pub)
	items, err := h.Handle(context.Background(), []domain.Draft{{
		Title:         "Матрица",
		Type:          "movie",
		ReleaseStatus: "completed",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !tx.called || pub.count != 1 || len(writer.saved) != 1 || len(items) != 1 {
		t.Fatalf("items=%+v saved=%+v pub=%d", items, writer.saved, pub.count)
	}
	if writer.saved[0].Type != "movie" || writer.saved[0].CreatedBy != nil {
		t.Fatalf("saved = %+v", writer.saved[0])
	}
}

func TestImportStopsWhenTypeIsInactive(t *testing.T) {
	t.Parallel()
	tx := &fakeTx{}
	h := NewImportMediaObjectsHandler(&fakeWriter{}, tx, fakeTypes{err: contract.FieldError{Field: "type"}}, &fakePublisher{})
	_, err := h.Handle(context.Background(), []domain.Draft{{
		Title:         "Матрица",
		Type:          "movie",
		ReleaseStatus: "completed",
	}})
	var field contract.FieldError
	if !errors.As(err, &field) || field.Field != "type" || tx.called {
		t.Fatal(err)
	}
}

func TestImportRejectsEmptyAndOversizedBatch(t *testing.T) {
	t.Parallel()
	h := NewImportMediaObjectsHandler(&fakeWriter{}, &fakeTx{}, fakeTypes{}, &fakePublisher{})
	_, err := h.Handle(context.Background(), nil)
	var field contract.FieldError
	if !errors.As(err, &field) || field.Field != "items" {
		t.Fatal(err)
	}
	drafts := make([]domain.Draft, domain.MaxImportBatch()+1)
	for i := range drafts {
		drafts[i] = domain.Draft{Title: "A", Type: "movie", ReleaseStatus: "completed"}
	}
	_, err = h.Handle(context.Background(), drafts)
	if !errors.As(err, &field) || field.Field != "items" {
		t.Fatal(err)
	}
}
