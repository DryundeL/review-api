package contract

import "github.com/google/uuid"

// MediaObjectRef is the catalog snapshot other modules copy into their own data.
type MediaObjectRef struct {
	ID        uuid.UUID
	Type      string
	Title     string
	PosterURL string // empty when the work has no poster
}
