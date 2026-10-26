package domain

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
)

const (
	maxReviewTitleLen = 256
	maxBlockTitleLen  = 50
	maxBlocks         = 10
	minRating         = 1
	maxRating         = 10
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Block is one section of a review body.
type Block struct {
	ID       uuid.UUID
	Position int16
	Title    string
	Body     string
}

// Review is the write-side aggregate.
type Review struct {
	ID            uuid.UUID
	AuthorID      uuid.UUID
	MediaObjectID uuid.UUID
	Title         string
	Rating        int16
	Status        string
	Blocks        []Block
}

// Draft is the validated input for create/update.
type Draft struct {
	MediaObjectID uuid.UUID
	Title         string
	Rating        int16
	Status        string
	Blocks        []BlockDraft
}

// BlockDraft is one block from the API body.
type BlockDraft struct {
	Title string
	Body  string
}

func NewReview(authorID uuid.UUID, draft Draft) (Review, error) {
	if authorID == uuid.Nil {
		return Review{}, contract.FieldError{Field: "authorId"}
	}
	if draft.MediaObjectID == uuid.Nil {
		return Review{}, contract.FieldError{Field: "mediaObjectId"}
	}
	title, err := parseReviewTitle(draft.Title)
	if err != nil {
		return Review{}, err
	}
	if err := parseRating(draft.Rating); err != nil {
		return Review{}, err
	}
	status, err := parseStatus(draft.Status)
	if err != nil {
		return Review{}, err
	}
	blocks, err := parseBlocks(draft.Blocks)
	if err != nil {
		return Review{}, err
	}
	if status == StatusPublished && len(blocks) == 0 {
		return Review{}, contract.FieldError{Field: "blocks"}
	}
	return Review{
		ID:            uuid.New(),
		AuthorID:      authorID,
		MediaObjectID: draft.MediaObjectID,
		Title:         title,
		Rating:        draft.Rating,
		Status:        status,
		Blocks:        blocks,
	}, nil
}

func (r *Review) Apply(draft Draft) error {
	if r == nil {
		return contract.ErrInvalid
	}
	title, err := parseReviewTitle(draft.Title)
	if err != nil {
		return err
	}
	if err := parseRating(draft.Rating); err != nil {
		return err
	}
	status := r.Status
	if strings.TrimSpace(draft.Status) != "" {
		status, err = parseStatus(draft.Status)
		if err != nil {
			return err
		}
	}
	blocks, err := parseBlocks(draft.Blocks)
	if err != nil {
		return err
	}
	if status == StatusPublished && len(blocks) == 0 {
		return contract.FieldError{Field: "blocks"}
	}
	// mediaObjectId is fixed after create
	r.Title = title
	r.Rating = draft.Rating
	r.Status = status
	r.Blocks = blocks
	return nil
}

func parseReviewTitle(value string) (string, error) {
	value = sanitizeText(value)
	if value == "" || utf8.RuneCountInString(value) > maxReviewTitleLen {
		return "", contract.FieldError{Field: "title"}
	}
	return value, nil
}

func parseRating(value int16) error {
	if value < minRating || value > maxRating {
		return contract.FieldError{Field: "rating"}
	}
	return nil
}

func parseStatus(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return StatusDraft, nil
	}
	switch value {
	case StatusDraft, StatusPublished:
		return value, nil
	default:
		return "", contract.FieldError{Field: "status"}
	}
}

func parseBlocks(items []BlockDraft) ([]Block, error) {
	if len(items) > maxBlocks {
		return nil, contract.FieldError{Field: "blocks"}
	}
	out := make([]Block, 0, len(items))
	for i, item := range items {
		title, err := parseBlockTitle(item.Title)
		if err != nil {
			return nil, err
		}
		body, err := parseBlockBody(item.Body)
		if err != nil {
			return nil, err
		}
		out = append(out, Block{
			ID:       uuid.New(),
			Position: int16(i + 1),
			Title:    title,
			Body:     body,
		})
	}
	return out, nil
}

func parseBlockTitle(value string) (string, error) {
	value = sanitizeText(value)
	if value == "" || utf8.RuneCountInString(value) > maxBlockTitleLen {
		return "", contract.FieldError{Field: "blocks"}
	}
	return value, nil
}

func parseBlockBody(value string) (string, error) {
	value = sanitizeText(value)
	if value == "" {
		return "", contract.FieldError{Field: "blocks"}
	}
	return value, nil
}

// sanitizeText trims, drops controls, rejects markup angle brackets (text/emoji/entities only).
func sanitizeText(value string) string {
	value = strings.TrimSpace(strings.Map(dropControl, value))
	if strings.ContainsAny(value, "<>") {
		return ""
	}
	return value
}
