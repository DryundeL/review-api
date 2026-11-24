package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
)

func snapshot(format Format, items, tiers int) Snapshot {
	s := Snapshot{VersionID: uuid.New(), TemplateID: uuid.New(), Format: format, Title: "Игры 2026"}
	for range items {
		s.Items = append(s.Items, manual("item"))
	}
	if format == FormatTiers {
		for range tiers {
			s.Tiers = append(s.Tiers, Tier{ID: uuid.New(), Name: "T", Color: "#ffffff"})
		}
	}
	return s
}

func newResult(t *testing.T, snap Snapshot) Result {
	t.Helper()
	r, err := NewResult(uuid.New(), uuid.New(), snap, now)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func at(item Item, pos int) Entry { return Entry{ItemID: item.ID, Position: pos} }

func inTier(item Item, tier Tier, pos int) Entry {
	id := tier.ID
	return Entry{ItemID: item.ID, TierID: &id, Position: pos}
}

func TestNewResultDefaults(t *testing.T) {
	t.Parallel()
	snap := snapshot(FormatRanked, 3, 0)
	r := newResult(t, snap)
	if r.Title != snap.Title || r.Status != StatusDraft || r.Format != FormatRanked || len(r.Entries) != 0 {
		t.Fatalf("result = %+v", r)
	}
	// An empty result may be saved as a draft but not published.
	wantField(t, r.Publish(VisibilityPublic, now, false), "entries", contract.ReasonRequired)
}

func TestRankedEntries(t *testing.T) {
	t.Parallel()
	snap := snapshot(FormatRanked, 4, 0)
	it := snap.Items
	r := newResult(t, snap)
	if err := r.Replace(snap, "Мой топ", "", []Entry{at(it[2], 2), at(it[0], 1)}); err != nil {
		t.Fatalf("partial ranking: %v", err)
	}
	if r.Entries[0].ItemID != it[0].ID || r.CoverURL(snap) != it[0].ImageURL {
		t.Fatal("place 1 must come first and be the cover")
	}
	cases := map[string][]Entry{
		"gap":           {at(it[0], 1), at(it[1], 3)},
		"shared place":  {at(it[0], 1), at(it[1], 1)},
		"starts at 0":   {at(it[0], 0)},
		"beyond items":  {at(it[0], 5)},
		"with tier":     {inTier(it[0], Tier{ID: uuid.New()}, 1)},
		"foreign item":  {{ItemID: uuid.New(), Position: 1}},
		"duplicate":     {at(it[0], 1), at(it[0], 2)},
		"more than all": {at(it[0], 1), at(it[1], 2), at(it[2], 3), at(it[3], 4), {ItemID: uuid.New(), Position: 5}},
	}
	for name, entries := range cases {
		if err := r.Replace(snap, "Мой топ", "", entries); !errors.Is(err, contract.ErrInvalid) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}

func TestTierEntries(t *testing.T) {
	t.Parallel()
	snap := snapshot(FormatTiers, 4, 3)
	it, tiers := snap.Items, snap.Tiers
	r := newResult(t, snap)
	// Top tier is empty: the cover is the first item of the next non-empty tier.
	entries := []Entry{inTier(it[3], tiers[2], 1), inTier(it[1], tiers[1], 2), inTier(it[2], tiers[1], 1)}
	if err := r.Replace(snap, "T", "", entries); err != nil {
		t.Fatal(err)
	}
	if r.CoverURL(snap) != it[2].ImageURL {
		t.Fatal("cover must be the first item of the top non-empty tier")
	}
	if r.Entries[0].ItemID != it[2].ID || r.Entries[1].ItemID != it[1].ID || r.Entries[2].ItemID != it[3].ID {
		t.Fatalf("entries are not in canonical order: %+v", r.Entries)
	}
	cases := map[string][]Entry{
		"no tier":           {at(it[0], 1)},
		"foreign tier":      {inTier(it[0], Tier{ID: uuid.New()}, 1)},
		"gap inside tier":   {inTier(it[0], tiers[0], 1), inTier(it[1], tiers[0], 3)},
		"shared inside tie": {inTier(it[0], tiers[0], 1), inTier(it[1], tiers[0], 1)},
	}
	for name, entries := range cases {
		if err := r.Replace(snap, "T", "", entries); !errors.Is(err, contract.ErrInvalid) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	// Equal positions in different tiers are fine.
	if err := r.Replace(snap, "T", "", []Entry{inTier(it[0], tiers[0], 1), inTier(it[1], tiers[1], 1)}); err != nil {
		t.Fatal(err)
	}
}

func TestCollectionEntries(t *testing.T) {
	t.Parallel()
	snap := snapshot(FormatCollection, 3, 0)
	it := snap.Items
	r := newResult(t, snap)
	if err := r.Replace(snap, "Подборка", "", []Entry{at(it[2], 1), at(it[0], 2)}); err != nil {
		t.Fatal(err)
	}
	if r.CoverURL(snap) != it[2].ImageURL {
		t.Fatal("cover must be the first item of the collection")
	}
	if err := r.Replace(snap, "Подборка", "", []Entry{at(it[0], 2)}); !errors.Is(err, contract.ErrInvalid) {
		t.Fatalf("display order must start at 1: %v", err)
	}
}

func TestResultSnapshotMismatch(t *testing.T) {
	t.Parallel()
	r := newResult(t, snapshot(FormatRanked, 2, 0))
	other := snapshot(FormatRanked, 2, 0)
	if err := r.Replace(other, "T", "", nil); !errors.Is(err, contract.ErrInvalid) {
		t.Fatalf("foreign snapshot: %v", err)
	}
}

func TestResultLifecycle(t *testing.T) {
	t.Parallel()
	snap := snapshot(FormatRanked, 2, 0)
	r := newResult(t, snap)
	if err := r.Replace(snap, "Мой топ", "desc", []Entry{at(snap.Items[0], 1)}); err != nil {
		t.Fatal(err)
	}
	if err := r.Publish(VisibilityPublic, now, true); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("publish on moderated template: %v", err)
	}
	if err := r.Publish(VisibilityPublic, now, false); err != nil {
		t.Fatal(err)
	}
	// A published result cannot be emptied or lose its title.
	wantField(t, r.Replace(snap, "Мой топ", "", nil), "entries", contract.ReasonRequired)
	wantField(t, r.Replace(snap, " ", "", []Entry{at(snap.Items[0], 1)}), "title", contract.ReasonRequired)

	cp, err := r.Copy(uuid.New(), now, false)
	if err != nil {
		t.Fatal(err)
	}
	if cp.VersionID != r.VersionID || cp.Status != StatusDraft || cp.FirstPublishedAt != nil || len(cp.Entries) != 1 {
		t.Fatalf("copy = %+v", cp)
	}
	if _, err := r.Copy(uuid.New(), now, true); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("copy on moderated template: %v", err)
	}
	r.Unpublish()
	if err := r.SetVisibility(VisibilityUnlisted); !errors.Is(err, contract.ErrConflict) {
		t.Fatalf("visibility of draft: %v", err)
	}
	if err := r.Publish(VisibilityUnlisted, now.Add(time.Hour), false); err != nil {
		t.Fatal(err)
	}
	if !r.FirstPublishedAt.Equal(now) || r.Visibility != VisibilityUnlisted {
		t.Fatalf("republish = %+v", r)
	}
}
