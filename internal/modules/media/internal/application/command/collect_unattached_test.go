package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/media/internal/application/ports"
)

type gcRepo struct {
	items   []ports.ImageRecord
	cutoff  time.Time
	deleted []uuid.UUID
}

func (r *gcRepo) Insert(context.Context, ports.ImageRecord) error { return nil }

func (r *gcRepo) LockUnattached(_ context.Context, cutoff time.Time, limit int) ([]ports.ImageRecord, error) {
	r.cutoff = cutoff
	if len(r.items) > limit {
		return r.items[:limit], nil
	}
	return r.items, nil
}

func (r *gcRepo) DeleteByIDs(_ context.Context, ids []uuid.UUID) error {
	r.deleted = append(r.deleted, ids...)
	return nil
}

type gcFiles struct {
	failKey string
	deleted []string
}

func (f *gcFiles) Put(context.Context, string, []byte) error { return nil }

func (f *gcFiles) Delete(_ context.Context, key string) error {
	if key == f.failKey {
		return errors.New("io error")
	}
	f.deleted = append(f.deleted, key)
	return nil
}

type inlineTx struct{}

func (inlineTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func TestCollectUnattachedKeepsRowsWhoseFileWasNotDeleted(t *testing.T) {
	t.Parallel()
	a := ports.ImageRecord{ID: uuid.New(), StorageKey: "images/a.png"}
	b := ports.ImageRecord{ID: uuid.New(), StorageKey: "images/b.png"}
	repo := &gcRepo{items: []ports.ImageRecord{a, b}}
	files := &gcFiles{failKey: b.StorageKey}
	h := NewCollectUnattachedHandler(repo, files, inlineTx{}, 24*time.Hour)
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	h.now = func() time.Time { return now }

	n, err := h.Handle(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(repo.deleted) != 1 || repo.deleted[0] != a.ID {
		t.Fatalf("removed %d, rows %v", n, repo.deleted)
	}
	if !repo.cutoff.Equal(now.Add(-24 * time.Hour)) {
		t.Fatalf("cutoff = %v", repo.cutoff)
	}
}

func TestCollectUnattachedRejectsBadArgs(t *testing.T) {
	t.Parallel()
	h := NewCollectUnattachedHandler(&gcRepo{}, &gcFiles{}, inlineTx{}, 0)
	if _, err := h.Handle(context.Background(), 10); err == nil {
		t.Fatal("zero ttl must be rejected")
	}
	h = NewCollectUnattachedHandler(&gcRepo{}, &gcFiles{}, inlineTx{}, time.Hour)
	if _, err := h.Handle(context.Background(), 0); err == nil {
		t.Fatal("zero batch must be rejected")
	}
}
