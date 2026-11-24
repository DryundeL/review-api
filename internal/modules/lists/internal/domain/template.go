package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
)

var colorPattern = regexp.MustCompile(`^#[0-9a-f]{6}$`)

type Tier struct {
	ID    uuid.UUID
	Name  string
	Color string // #rrggbb, lower case
}

// Item is a template element. Catalog items keep MediaObjectID; uploaded images keep ImageID.
// ImageURL is resolved by the application layer (upload URL or copied catalog poster).
type Item struct {
	ID            uuid.UUID
	Title         string
	MediaObjectID *uuid.UUID
	ImageID       *uuid.UUID
	ImageURL      string
}

type Template struct {
	ID                 uuid.UUID
	AuthorID           uuid.UUID
	TypeCode           string
	Format             Format
	Title              string
	Description        string
	Status             Status
	Visibility         Visibility
	Revision           int
	FirstPublishedAt   *time.Time
	ModerationHiddenAt *time.Time
	CreatedAt          time.Time
	Tiers              []Tier
	Items              []Item
}

// TemplateContent is everything the author edits in one save.
type TemplateContent struct {
	TypeCode    string
	Format      Format
	Title       string
	Description string
	Tiers       []Tier
	Items       []Item
}

// DefaultTiers are the TierMaker-style S/A/B/C/D rows with fresh ids.
func DefaultTiers() []Tier {
	names := []string{"S", "A", "B", "C", "D"}
	colors := []string{"#ff7f7f", "#ffbf7f", "#ffdf7f", "#ffff7f", "#bfff7f"}
	out := make([]Tier, len(names))
	for i := range names {
		out[i] = Tier{ID: uuid.New(), Name: names[i], Color: colors[i]}
	}
	return out
}

// NewTemplate creates a draft. Tiers format gets the default S..D rows.
func NewTemplate(id, authorID uuid.UUID, typeCode string, format Format, title string, now time.Time) (Template, error) {
	if id == uuid.Nil || authorID == uuid.Nil {
		return Template{}, contract.ErrInvalid
	}
	t := Template{
		ID: id, AuthorID: authorID, Status: StatusDraft, Visibility: VisibilityPublic,
		Revision: 1, CreatedAt: now.UTC(),
	}
	content := TemplateContent{TypeCode: typeCode, Format: format, Title: title}
	if format == FormatTiers {
		content.Tiers = DefaultTiers()
	}
	if err := t.Replace(content); err != nil {
		return Template{}, err
	}
	return t, nil
}

// Replace validates and applies a full save. A published template must stay publishable,
// because a save of a published template is visible immediately.
func (t *Template) Replace(c TemplateContent) error {
	typeCode := strings.TrimSpace(c.TypeCode)
	if typeCode == "" {
		return contract.Field("typeCode", contract.ReasonRequired)
	}
	switch c.Format {
	case FormatTiers, FormatRanked, FormatCollection:
	default:
		return contract.Field("format", contract.ReasonInvalid)
	}
	if t.FirstPublishedAt != nil {
		if typeCode != t.TypeCode {
			return contract.Field("typeCode", contract.ReasonImmutable)
		}
		if c.Format != t.Format {
			return contract.Field("format", contract.ReasonImmutable)
		}
	}
	title, err := cleanField(c.Title, MaxTitleLen, false, "title", -1)
	if err != nil {
		return err
	}
	description, err := cleanField(c.Description, MaxDescriptionLen, true, "description", -1)
	if err != nil {
		return err
	}
	tiers, err := cleanTiers(c.Format, c.Tiers)
	if err != nil {
		return err
	}
	items, err := cleanItems(c.Items)
	if err != nil {
		return err
	}
	next := TemplateContent{TypeCode: typeCode, Format: c.Format, Title: title, Description: description, Tiers: tiers, Items: items}
	if t.Status == StatusPublished {
		if err := checkPublishable(next); err != nil {
			return err
		}
	}
	t.TypeCode, t.Format, t.Title, t.Description = typeCode, c.Format, title, description
	t.Tiers, t.Items = tiers, items
	return nil
}

func cleanTiers(format Format, in []Tier) ([]Tier, error) {
	if format != FormatTiers {
		if len(in) > 0 {
			return nil, contract.Field("tiers", contract.ReasonInvalid)
		}
		return nil, nil
	}
	if len(in) < MinTiers {
		return nil, contract.Field("tiers", contract.ReasonRequired)
	}
	if len(in) > MaxTiers {
		return nil, contract.Field("tiers", contract.ReasonTooMany)
	}
	out := make([]Tier, 0, len(in))
	seen := make(map[uuid.UUID]struct{}, len(in))
	for i, tier := range in {
		if tier.ID == uuid.Nil {
			return nil, contract.FieldAt("tiers.id", i, contract.ReasonRequired)
		}
		if _, dup := seen[tier.ID]; dup {
			return nil, contract.FieldAt("tiers.id", i, contract.ReasonDuplicate)
		}
		seen[tier.ID] = struct{}{}
		name, err := cleanField(tier.Name, MaxTierNameLen, false, "tiers.name", i)
		if err != nil {
			return nil, err
		}
		color := strings.ToLower(strings.TrimSpace(tier.Color))
		if !colorPattern.MatchString(color) {
			return nil, contract.FieldAt("tiers.color", i, contract.ReasonInvalid)
		}
		out = append(out, Tier{ID: tier.ID, Name: name, Color: color})
	}
	return out, nil
}

func cleanItems(in []Item) ([]Item, error) {
	if len(in) > MaxItems {
		return nil, contract.Field("items", contract.ReasonTooMany)
	}
	out := make([]Item, 0, len(in))
	ids := make(map[uuid.UUID]struct{}, len(in))
	works := make(map[uuid.UUID]struct{}, len(in))
	for i, item := range in {
		if item.ID == uuid.Nil {
			return nil, contract.FieldAt("items.id", i, contract.ReasonRequired)
		}
		if _, dup := ids[item.ID]; dup {
			return nil, contract.FieldAt("items.id", i, contract.ReasonDuplicate)
		}
		ids[item.ID] = struct{}{}
		if item.MediaObjectID != nil {
			if *item.MediaObjectID == uuid.Nil {
				return nil, contract.FieldAt("items.mediaObjectId", i, contract.ReasonInvalid)
			}
			if _, dup := works[*item.MediaObjectID]; dup {
				return nil, contract.FieldAt("items.mediaObjectId", i, contract.ReasonDuplicate)
			}
			works[*item.MediaObjectID] = struct{}{}
		}
		title, err := cleanField(item.Title, MaxItemTitleLen, false, "items.title", i)
		if err != nil {
			return nil, err
		}
		if item.ImageID != nil && (*item.ImageID == uuid.Nil || item.ImageURL == "") {
			return nil, contract.FieldAt("items.imageId", i, contract.ReasonInvalid)
		}
		item.Title = title
		out = append(out, item)
	}
	return out, nil
}

func checkPublishable(c TemplateContent) error {
	if c.Title == "" {
		return contract.Field("title", contract.ReasonRequired)
	}
	if len(c.Items) == 0 {
		return contract.Field("items", contract.ReasonRequired)
	}
	for i, item := range c.Items {
		if item.Title == "" {
			return contract.FieldAt("items.title", i, contract.ReasonRequired)
		}
		if item.ImageURL == "" {
			return contract.FieldAt("items.image", i, contract.ReasonRequired)
		}
	}
	for i, tier := range c.Tiers {
		if tier.Name == "" {
			return contract.FieldAt("tiers.name", i, contract.ReasonRequired)
		}
	}
	return nil
}

// Content is the editable part of the template.
func (t *Template) Content() TemplateContent {
	return TemplateContent{TypeCode: t.TypeCode, Format: t.Format, Title: t.Title, Description: t.Description, Tiers: t.Tiers, Items: t.Items}
}

// Publish makes the template visible. The first publication date is set once and kept forever.
func (t *Template) Publish(visibility Visibility, now time.Time) error {
	if t.ModerationHiddenAt != nil {
		return contract.ErrModerated
	}
	if err := checkPublishable(t.Content()); err != nil {
		return err
	}
	if t.FirstPublishedAt == nil {
		at := now.UTC()
		t.FirstPublishedAt = &at
	}
	t.Status = StatusPublished
	t.Visibility = visibility
	return nil
}

// Unpublish returns the template to draft. Type and format stay locked.
func (t *Template) Unpublish() {
	t.Status = StatusDraft
}

func (t *Template) SetVisibility(v Visibility) error {
	if t.Status != StatusPublished {
		return contract.ErrConflict
	}
	t.Visibility = v
	return nil
}

// Copy is a new never-published draft with the same content. A moderated template cannot be copied.
func (t Template) Copy(id uuid.UUID, now time.Time) (Template, error) {
	if t.ModerationHiddenAt != nil {
		return Template{}, contract.ErrModerated
	}
	if id == uuid.Nil {
		return Template{}, contract.ErrInvalid
	}
	return Template{
		ID: id, AuthorID: t.AuthorID, TypeCode: t.TypeCode, Format: t.Format,
		Title: t.Title, Description: t.Description, Status: StatusDraft, Visibility: VisibilityPublic,
		Revision: 1, CreatedAt: now.UTC(),
		Tiers: append([]Tier(nil), t.Tiers...), Items: append([]Item(nil), t.Items...),
	}, nil
}

// CoverURL is the image of the first item, empty if there is none.
func (t Template) CoverURL() string {
	if len(t.Items) == 0 {
		return ""
	}
	return t.Items[0].ImageURL
}

// AcceptsResults reports whether a new result may be created from the template right now.
func (t Template) AcceptsResults() bool {
	return t.Status == StatusPublished && t.ModerationHiddenAt == nil
}
