package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"review-api/internal/modules/media/contract"
)

const key = "images/0b5f3c1e-8a1d-4c55-9e4f-2a7b6c8d9e01.webp"

func TestLocalStorePutAndDelete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewLocalStore(dir)
	ctx := context.Background()

	if err := s.Put(ctx, key, []byte("data")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(key)))
	if err != nil || string(got) != "data" {
		t.Fatalf("read back %q, %v", got, err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "images"))
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("second Delete must be a no-op: %v", err)
	}
}

func TestLocalStoreRejectsForeignKeys(t *testing.T) {
	t.Parallel()
	s := NewLocalStore(t.TempDir())
	for _, k := range []string{
		"../images/0b5f3c1e-8a1d-4c55-9e4f-2a7b6c8d9e01.webp",
		"images/../../etc/passwd",
		"/images/0b5f3c1e-8a1d-4c55-9e4f-2a7b6c8d9e01.webp",
		"images/0b5f3c1e-8a1d-4c55-9e4f-2a7b6c8d9e01.svg",
		"",
	} {
		if err := s.Put(context.Background(), k, []byte("x")); !errors.Is(err, contract.ErrInvalid) {
			t.Fatalf("Put(%q) = %v", k, err)
		}
		if err := s.Delete(context.Background(), k); !errors.Is(err, contract.ErrInvalid) {
			t.Fatalf("Delete(%q) = %v", k, err)
		}
	}
}
