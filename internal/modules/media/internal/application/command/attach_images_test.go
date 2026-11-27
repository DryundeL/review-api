package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/media/contract"
	"review-api/internal/modules/media/internal/application/ports"
)

func TestAttachImages(t *testing.T) {
	t.Parallel()
	owner := uuid.New()
	a := ports.ImageRecord{ID: uuid.New(), StorageKey: "images/a.png", Width: 3, Height: 4}
	b := ports.ImageRecord{ID: uuid.New(), StorageKey: "images/b.webp", Width: 5, Height: 6}
	h := NewAttachImagesHandler(&gcRepo{items: []ports.ImageRecord{a, b}}, "/storage/")

	refs, err := h.Handle(context.Background(), owner, []uuid.UUID{b.ID, a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[0].ID != b.ID || refs[0].URL != "/storage/images/b.webp" || refs[1].ID != a.ID || refs[1].Width != 3 {
		t.Fatalf("refs = %+v", refs)
	}

	if refs, err := h.Handle(context.Background(), owner, nil); err != nil || len(refs) != 0 {
		t.Fatalf("empty: %v %v", refs, err)
	}
	for name, ids := range map[string][]uuid.UUID{
		"unknown": {a.ID, uuid.New()},
		"nil id":  {uuid.Nil},
	} {
		if _, err := h.Handle(context.Background(), owner, ids); !errors.Is(err, contract.ErrNotFound) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	tooMany := make([]uuid.UUID, contract.MaxAttachBatch+1)
	for i := range tooMany {
		tooMany[i] = uuid.New()
	}
	if _, err := h.Handle(context.Background(), owner, tooMany); !errors.Is(err, contract.ErrInvalid) {
		t.Fatalf("too many: %v", err)
	}
}
