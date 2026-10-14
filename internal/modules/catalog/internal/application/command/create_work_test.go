package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

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
	saved domain.Work
}

func (f *fakeWriter) Save(_ context.Context, work domain.Work) error {
	f.saved = work
	return nil
}

type fakeTypes struct{ err error }

func (f fakeTypes) RequireActive(context.Context, []string) error { return f.err }

type fakeAuthors struct {
	id  uuid.UUID
	err error
}

func (f fakeAuthors) UserID(context.Context, int64) (uuid.UUID, error) {
	return f.id, f.err
}

type fakePublisher struct{ called bool }

func (f *fakePublisher) Publish(context.Context, string, string, any) error {
	f.called = true
	return nil
}

func TestCreateWorkPublishesInTransaction(t *testing.T) {
	t.Parallel()
	author := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	writer := &fakeWriter{}
	tx := &fakeTx{}
	pub := &fakePublisher{}
	h := NewCreateWorkHandler(writer, tx, fakeTypes{}, fakeAuthors{id: author}, pub)
	work, err := h.Handle(context.Background(), 42, domain.Draft{
		Title:         "Матрица",
		PrimaryType:   "movie",
		ReleaseStatus: "completed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !tx.called || !pub.called || writer.saved.CreatedBy != author || work.Origin != "user" {
		t.Fatalf("work = %+v saved = %+v", work, writer.saved)
	}
}

func TestCreateWorkStopsWhenTypeIsInactive(t *testing.T) {
	t.Parallel()
	tx := &fakeTx{}
	h := NewCreateWorkHandler(&fakeWriter{}, tx, fakeTypes{err: contract.FieldError{Field: "type"}}, fakeAuthors{
		id: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
	}, &fakePublisher{})
	_, err := h.Handle(context.Background(), 42, domain.Draft{
		Title:         "Матрица",
		PrimaryType:   "movie",
		ReleaseStatus: "completed",
	})
	if !errors.As(err, &contract.FieldError{}) || tx.called {
		t.Fatal(err)
	}
}
