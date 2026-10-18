package contract

import "github.com/google/uuid"

// ReviewRef is a minimal published-review snapshot for other BCs.
type ReviewRef struct {
	ID       uuid.UUID
	AuthorID uuid.UUID
	Title    string
}
