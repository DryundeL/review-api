package domain

import (
	"cmp"
	"slices"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
)

// Snapshot is the immutable template version a result is built on.
type Snapshot struct {
	VersionID  uuid.UUID
	TemplateID uuid.UUID
	Format     Format
	Title      string
	Items      []Item // in template order
	Tiers      []Tier // in template order, top tier first
}

func (s Snapshot) hasItem(id uuid.UUID) bool {
	return slices.ContainsFunc(s.Items, func(it Item) bool { return it.ID == id })
}

func (s Snapshot) tierIndex(id uuid.UUID) int {
	return slices.IndexFunc(s.Tiers, func(t Tier) bool { return t.ID == id })
}

func (s Snapshot) item(id uuid.UUID) (Item, bool) {
	i := slices.IndexFunc(s.Items, func(it Item) bool { return it.ID == id })
	if i < 0 {
		return Item{}, false
	}
	return s.Items[i], true
}

// Entry places one template item. Position is 1-based: the place in a ranked list,
// the order inside a tier, or the display order of a collection.
type Entry struct {
	ItemID   uuid.UUID
	TierID   *uuid.UUID
	Position int
}

type Result struct {
	ID                 uuid.UUID
	AuthorID           uuid.UUID
	TemplateID         uuid.UUID
	VersionID          uuid.UUID
	Format             Format
	Title              string
	Description        string
	Status             Status
	Visibility         Visibility
	Revision           int
	FirstPublishedAt   *time.Time
	ModerationHiddenAt *time.Time
	CreatedAt          time.Time
	Entries            []Entry
}

// NewResult creates an empty draft named after the template.
func NewResult(id, authorID uuid.UUID, snap Snapshot, now time.Time) (Result, error) {
	if id == uuid.Nil || authorID == uuid.Nil || snap.VersionID == uuid.Nil || snap.TemplateID == uuid.Nil {
		return Result{}, contract.ErrInvalid
	}
	r := Result{
		ID: id, AuthorID: authorID, TemplateID: snap.TemplateID, VersionID: snap.VersionID,
		Format: snap.Format, Status: StatusDraft, Visibility: VisibilityPublic, Revision: 1,
		CreatedAt: now.UTC(),
	}
	if err := r.Replace(snap, snap.Title, "", nil); err != nil {
		return Result{}, err
	}
	return r, nil
}

// Replace validates and applies a full save against the result's snapshot.
// A published result must keep at least one placed item.
func (r *Result) Replace(snap Snapshot, title, description string, entries []Entry) error {
	if snap.VersionID != r.VersionID {
		return contract.ErrInvalid
	}
	cleanTitle, err := cleanField(title, MaxTitleLen, false, "title", -1)
	if err != nil {
		return err
	}
	cleanDescription, err := cleanField(description, MaxDescriptionLen, true, "description", -1)
	if err != nil {
		return err
	}
	cleanEntries, err := validateEntries(snap, entries)
	if err != nil {
		return err
	}
	if r.Status == StatusPublished {
		if err := checkResultPublishable(cleanTitle, cleanEntries); err != nil {
			return err
		}
	}
	r.Title, r.Description, r.Entries = cleanTitle, cleanDescription, cleanEntries
	return nil
}

// validateEntries checks membership, duplicates and the placement rules of the format,
// and returns entries in canonical order (tier order, then position).
func validateEntries(snap Snapshot, in []Entry) ([]Entry, error) {
	if len(in) > len(snap.Items) {
		return nil, contract.Field("entries", contract.ReasonTooMany)
	}
	seen := make(map[uuid.UUID]struct{}, len(in))
	// positions per group: tier id, or uuid.Nil for formats without tiers
	groups := make(map[uuid.UUID][]int)
	out := make([]Entry, 0, len(in))
	for i, e := range in {
		if !snap.hasItem(e.ItemID) {
			return nil, contract.FieldAt("entries.itemId", i, contract.ReasonInvalid)
		}
		if _, dup := seen[e.ItemID]; dup {
			return nil, contract.FieldAt("entries.itemId", i, contract.ReasonDuplicate)
		}
		seen[e.ItemID] = struct{}{}
		group := uuid.Nil
		if snap.Format == FormatTiers {
			if e.TierID == nil {
				return nil, contract.FieldAt("entries.tierId", i, contract.ReasonRequired)
			}
			if snap.tierIndex(*e.TierID) < 0 {
				return nil, contract.FieldAt("entries.tierId", i, contract.ReasonInvalid)
			}
			group = *e.TierID
		} else if e.TierID != nil {
			return nil, contract.FieldAt("entries.tierId", i, contract.ReasonInvalid)
		}
		if e.Position < 1 || e.Position > len(snap.Items) {
			return nil, contract.FieldAt("entries.position", i, contract.ReasonInvalid)
		}
		groups[group] = append(groups[group], e.Position)
		out = append(out, Entry{ItemID: e.ItemID, TierID: cloneID(e.TierID), Position: e.Position})
	}
	// Positions in each group must be exactly 1..n: no gaps, no shared places.
	for _, positions := range groups {
		slices.Sort(positions)
		for i, p := range positions {
			if p != i+1 {
				return nil, contract.Field("entries.position", contract.ReasonInvalid)
			}
		}
	}
	slices.SortFunc(out, func(a, b Entry) int {
		ta, tb := -1, -1
		if a.TierID != nil {
			ta = snap.tierIndex(*a.TierID)
		}
		if b.TierID != nil {
			tb = snap.tierIndex(*b.TierID)
		}
		return cmp.Or(cmp.Compare(ta, tb), cmp.Compare(a.Position, b.Position))
	})
	return out, nil
}

func cloneID(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}
	v := *id
	return &v
}

func checkResultPublishable(title string, entries []Entry) error {
	if title == "" {
		return contract.Field("title", contract.ReasonRequired)
	}
	if len(entries) == 0 {
		return contract.Field("entries", contract.ReasonRequired)
	}
	return nil
}

// Publish makes the result visible. templateModerated blocks publication of results
// whose template is hidden by a moderator, including drafts started earlier.
func (r *Result) Publish(visibility Visibility, now time.Time, templateModerated bool) error {
	if r.ModerationHiddenAt != nil || templateModerated {
		return contract.ErrModerated
	}
	if err := checkResultPublishable(r.Title, r.Entries); err != nil {
		return err
	}
	if r.FirstPublishedAt == nil {
		at := now.UTC()
		r.FirstPublishedAt = &at
	}
	r.Status = StatusPublished
	r.Visibility = visibility
	return nil
}

func (r *Result) Unpublish() {
	r.Status = StatusDraft
}

func (r *Result) SetVisibility(v Visibility) error {
	if r.Status != StatusPublished {
		return contract.ErrConflict
	}
	r.Visibility = v
	return nil
}

// Copy keeps the snapshot and the placement. It is blocked when the result or its template is moderated.
func (r Result) Copy(id uuid.UUID, now time.Time, templateModerated bool) (Result, error) {
	if r.ModerationHiddenAt != nil || templateModerated {
		return Result{}, contract.ErrModerated
	}
	if id == uuid.Nil {
		return Result{}, contract.ErrInvalid
	}
	entries := make([]Entry, len(r.Entries))
	for i, e := range r.Entries {
		entries[i] = Entry{ItemID: e.ItemID, TierID: cloneID(e.TierID), Position: e.Position}
	}
	return Result{
		ID: id, AuthorID: r.AuthorID, TemplateID: r.TemplateID, VersionID: r.VersionID, Format: r.Format,
		Title: r.Title, Description: r.Description, Status: StatusDraft, Visibility: VisibilityPublic,
		Revision: 1, CreatedAt: now.UTC(), Entries: entries,
	}, nil
}

// CoverURL is the image of the first placed item: place 1 of a ranked list, the first item
// of the top non-empty tier, or the first item of a collection. Entries are in canonical order.
func (r Result) CoverURL(snap Snapshot) string {
	if len(r.Entries) == 0 {
		return ""
	}
	item, ok := snap.item(r.Entries[0].ItemID)
	if !ok {
		return ""
	}
	return item.ImageURL
}
