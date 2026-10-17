package query

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/feed/contract"
	"review-api/internal/modules/feed/internal/application/ports"
)

type cursorJSON struct {
	Sort string    `json:"s"`
	V    int64     `json:"v"`
	T    time.Time `json:"t"`
	ID   string    `json:"id"`
}

func EncodeCursor(c ports.Cursor) (string, error) {
	if c.ID == uuid.Nil || c.T.IsZero() {
		return "", contract.FieldError{Field: "cursor"}
	}
	raw, err := json.Marshal(cursorJSON{
		Sort: c.Sort,
		V:    c.V,
		T:    c.T.UTC(),
		ID:   c.ID.String(),
	})
	if err != nil {
		return "", contract.FieldError{Field: "cursor"}
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeCursor(raw, expectedSort string) (*ports.Cursor, error) {
	if raw == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, contract.FieldError{Field: "cursor"}
	}
	var payload cursorJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, contract.FieldError{Field: "cursor"}
	}
	if payload.Sort != expectedSort {
		return nil, contract.FieldError{Field: "cursor"}
	}
	id, err := uuid.Parse(payload.ID)
	if err != nil || id == uuid.Nil || payload.T.IsZero() {
		return nil, contract.FieldError{Field: "cursor"}
	}
	return &ports.Cursor{
		Sort: payload.Sort,
		V:    payload.V,
		T:    payload.T.UTC(),
		ID:   id,
	}, nil
}

func cursorFromItem(sort string, item ports.Item) ports.Cursor {
	v := int64(0)
	switch sort {
	case ports.SortViews:
		v = item.ViewsCount
	case ports.SortLikes:
		v = item.LikesCount
	}
	return ports.Cursor{
		Sort: sort,
		V:    v,
		T:    item.CreatedAt.UTC(),
		ID:   item.ID,
	}
}
