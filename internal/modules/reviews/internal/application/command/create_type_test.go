package command

import (
	"context"
	"errors"
	"testing"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/domain"
)

type fakeWriter struct {
	item    domain.ReviewType
	next    int
	nextErr error
	err     error
}

func (f *fakeWriter) Create(_ context.Context, item domain.ReviewType) error {
	if f.err != nil {
		return f.err
	}
	f.item = item
	return nil
}

func (f *fakeWriter) Update(context.Context, string, *string, *int, *bool) (domain.ReviewType, error) {
	return domain.ReviewType{}, errors.New("not used")
}

func (f *fakeWriter) NextSort(context.Context) (int, error) {
	if f.nextErr != nil {
		return 0, f.nextErr
	}
	return f.next, nil
}

func TestCreateTypeUsesNextSort(t *testing.T) {
	t.Parallel()
	repo := &fakeWriter{next: 140}
	item, err := NewCreateTypeHandler(repo).Handle(context.Background(), "game", "Игра", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if item.SortOrder != 140 || item.Code != "game" || !item.IsActive {
		t.Fatalf("item = %+v", item)
	}
}

func TestCreateTypeRejectsBeforeWrite(t *testing.T) {
	t.Parallel()
	repo := &fakeWriter{}
	_, err := NewCreateTypeHandler(repo).Handle(context.Background(), "Bad", "Игра", nil, nil)
	if !errors.As(err, &contract.FieldError{}) {
		t.Fatal(err)
	}
	if repo.item.Code != "" {
		t.Fatal("writer was called")
	}
}

func TestUpdateTypeRequiresAField(t *testing.T) {
	t.Parallel()
	_, err := NewUpdateTypeHandler(&fakeWriter{}).Handle(context.Background(), "manga", nil, nil, nil)
	var field contract.FieldError
	if !errors.As(err, &field) || field.Field != "body" {
		t.Fatal(err)
	}
}
