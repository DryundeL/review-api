package domain

import (
	"html"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	htmlparser "golang.org/x/net/html"

	"review-api/internal/modules/reviews/contract"
)

const (
	maxReviewTitleLen = 256
	maxBodyTextLen    = 20000
	minRating         = 1
	maxRating         = 10
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Stable 422 reasons returned to the client.
const (
	ReasonRequired = "required"
	ReasonTooLong  = "too_long"
	ReasonInvalid  = "invalid"
)

// Review is the write-side aggregate. Title, Rating and Body are the working version.
type Review struct {
	ID                uuid.UUID
	AuthorID          uuid.UUID
	MediaObjectID     uuid.UUID
	Status            string
	Title             string
	Rating            *int16
	Body              string
	Revision          int64
	PublishedRevision *int64
}

// Draft is editor input; every field is optional until publication.
type Draft struct {
	Title  string
	Rating *int16
	Body   string
}

func NewReview(authorID, mediaObjectID uuid.UUID, draft Draft) (Review, error) {
	if authorID == uuid.Nil {
		return Review{}, contract.FieldError{Field: "authorId"}
	}
	if mediaObjectID == uuid.Nil {
		return Review{}, contract.FieldError{Field: "mediaObjectId"}
	}
	item := Review{
		ID:            uuid.New(),
		AuthorID:      authorID,
		MediaObjectID: mediaObjectID,
		Status:        StatusDraft,
	}
	if err := item.apply(draft); err != nil {
		return Review{}, err
	}
	item.Revision = 1
	return item, nil
}

// SaveDraft replaces the working version and bumps the revision. The public version is untouched.
func (r *Review) SaveDraft(draft Draft) error {
	if r == nil {
		return contract.ErrInvalid
	}
	if err := r.apply(draft); err != nil {
		return err
	}
	r.Revision++
	return nil
}

// Publish makes the current working revision public. It reports false when it is already public.
func (r *Review) Publish() (bool, error) {
	if r == nil {
		return false, contract.ErrInvalid
	}
	if r.Status == StatusPublished && r.PublishedRevision != nil && *r.PublishedRevision == r.Revision {
		return false, nil
	}
	if r.Rating == nil {
		return false, contract.FieldError{Field: "rating", Reason: ReasonRequired}
	}
	_, hasText, err := parseBody(r.Body)
	if err != nil {
		return false, err
	}
	if !hasText {
		return false, contract.FieldError{Field: "body", Reason: ReasonRequired}
	}
	rev := r.Revision
	r.PublishedRevision = &rev
	r.Status = StatusPublished
	return true, nil
}

func (r *Review) apply(draft Draft) error {
	title, err := parseReviewTitle(draft.Title)
	if err != nil {
		return err
	}
	if draft.Rating != nil && (*draft.Rating < minRating || *draft.Rating > maxRating) {
		return contract.FieldError{Field: "rating", Reason: ReasonInvalid}
	}
	body, _, err := parseBody(draft.Body)
	if err != nil {
		return err
	}
	// mediaObjectId is fixed after create
	r.Title = title
	r.Rating = draft.Rating
	r.Body = body
	return nil
}

func parseReviewTitle(value string) (string, error) {
	value = strings.TrimSpace(strings.Map(dropControl, value))
	if strings.ContainsAny(value, "<>") {
		return "", contract.FieldError{Field: "title", Reason: ReasonInvalid}
	}
	if utf8.RuneCountInString(value) > maxReviewTitleLen {
		return "", contract.FieldError{Field: "title", Reason: ReasonTooLong}
	}
	return value, nil
}

// parseBody normalizes whitelisted HTML. The limit counts code points of decoded text, tags excluded.
func parseBody(value string) (string, bool, error) {
	value = strings.TrimSpace(strings.Map(dropControl, value))
	invalid := contract.FieldError{Field: "body", Reason: ReasonInvalid}
	var out strings.Builder
	length, hasText := 0, false
	tokens := htmlparser.NewTokenizer(strings.NewReader(value))
	for {
		switch tokens.Next() {
		case htmlparser.ErrorToken:
			if tokens.Err() != io.EOF {
				return "", false, invalid
			}
			return out.String(), hasText, nil
		case htmlparser.TextToken:
			text := string(tokens.Text())
			length += utf8.RuneCountInString(text)
			if length > maxBodyTextLen {
				return "", false, contract.FieldError{Field: "body", Reason: ReasonTooLong}
			}
			if !hasText && strings.IndexFunc(text, func(r rune) bool { return !unicode.IsSpace(r) }) >= 0 {
				hasText = true
			}
			out.WriteString(html.EscapeString(text))
		case htmlparser.StartTagToken, htmlparser.EndTagToken, htmlparser.SelfClosingTagToken:
			token := tokens.Token()
			if len(token.Attr) != 0 {
				return "", false, invalid
			}
			switch token.Data {
			case "p", "br", "b", "strong", "i", "em", "u", "s", "ul", "ol", "li", "blockquote", "pre", "code", "span", "h3":
				out.WriteString(token.String())
			default:
				return "", false, invalid
			}
		default:
			return "", false, invalid
		}
	}
}
