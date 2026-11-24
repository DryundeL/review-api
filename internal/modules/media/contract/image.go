package contract

import (
	"errors"

	"github.com/google/uuid"
)

// ErrNotFound: an image does not exist or belongs to someone else. The two cases are not distinguished.
var ErrNotFound = errors.New("image not found")

// MaxAttachBatch bounds one AttachImages call (a list template holds at most 50 items).
const MaxAttachBatch = 100

// ImageRef is what other modules store: the URL is stable because files are immutable.
type ImageRef struct {
	ID     uuid.UUID
	URL    string
	Width  int
	Height int
}
