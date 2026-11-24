package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"

	"review-api/internal/modules/media/contract"
)

// keyPattern mirrors media_images_storage_key_chk; keys are server-generated, this is defence in depth.
var keyPattern = regexp.MustCompile(`^images/[0-9a-f-]{36}\.(jpg|png|webp)$`)

// LocalStore writes files below dir through os.Root, so no key can escape the directory.
type LocalStore struct {
	dir string
}

func NewLocalStore(dir string) *LocalStore {
	return &LocalStore{dir: dir}
}

// Put writes atomically: a temp file in the target directory is synced and renamed into place.
func (s *LocalStore) Put(_ context.Context, key string, data []byte) (err error) {
	if s == nil || s.dir == "" || !keyPattern.MatchString(key) {
		return contract.ErrInvalid
	}
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer root.Close()

	dir := path.Dir(key)
	if err := root.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create storage dir: %w", err)
	}
	tmp, err := tempName(dir)
	if err != nil {
		return err
	}
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = root.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("write image: %w", err)
	}
	if err = f.Sync(); err != nil {
		return fmt.Errorf("sync image: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("close image: %w", err)
	}
	if err = root.Rename(tmp, key); err != nil {
		return fmt.Errorf("rename image: %w", err)
	}
	return nil
}

// Delete is idempotent: a missing file is not an error.
func (s *LocalStore) Delete(_ context.Context, key string) error {
	if s == nil || s.dir == "" || !keyPattern.MatchString(key) {
		return contract.ErrInvalid
	}
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer root.Close()
	if err := root.Remove(key); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete image: %w", err)
	}
	return nil
}

func tempName(dir string) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("temp name: %w", err)
	}
	return path.Join(dir, ".upload-"+hex.EncodeToString(b[:])+".tmp"), nil
}
