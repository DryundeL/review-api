package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/domain"
)

type snapKey struct {
	template uuid.UUID
	revision int
}

// memResults stores results and snapshots by value, like memTemplates.
type memResults struct {
	templates map[uuid.UUID]domain.Template
	deletedT  map[uuid.UUID]bool
	versions  map[snapKey]domain.Snapshot
	results   map[uuid.UUID]domain.Result
	deletedR  map[uuid.UUID]bool
}

func newMemResults() *memResults {
	return &memResults{
		templates: map[uuid.UUID]domain.Template{}, deletedT: map[uuid.UUID]bool{},
		versions: map[snapKey]domain.Snapshot{}, results: map[uuid.UUID]domain.Result{}, deletedR: map[uuid.UUID]bool{},
	}
}

func (m *memResults) LockAuthor(context.Context, uuid.UUID) error { return nil }

func (m *memResults) CountResults(_ context.Context, authorID uuid.UUID) (int, error) {
	n := 0
	for id, r := range m.results {
		if r.AuthorID == authorID && !m.deletedR[id] {
			n++
		}
	}
	return n, nil
}

func (m *memResults) GetTemplateForShare(_ context.Context, id uuid.UUID) (domain.Template, error) {
	t, ok := m.templates[id]
	if !ok || m.deletedT[id] {
		return domain.Template{}, contract.ErrNotFound
	}
	return t, nil
}

func (m *memResults) TemplateModerated(_ context.Context, id uuid.UUID) (bool, error) {
	t, ok := m.templates[id]
	if !ok {
		return false, contract.ErrNotFound
	}
	return t.ModerationHiddenAt != nil, nil
}

func (m *memResults) EnsureSnapshot(_ context.Context, t domain.Template) (domain.Snapshot, error) {
	key := snapKey{t.ID, t.Revision}
	if s, ok := m.versions[key]; ok {
		return s, nil
	}
	s := domain.Snapshot{
		VersionID: uuid.New(), TemplateID: t.ID, Format: t.Format, Title: t.Title,
		Items: append([]domain.Item(nil), t.Items...), Tiers: append([]domain.Tier(nil), t.Tiers...),
	}
	m.versions[key] = s
	return s, nil
}

func (m *memResults) Snapshot(_ context.Context, versionID uuid.UUID) (domain.Snapshot, error) {
	for _, s := range m.versions {
		if s.VersionID == versionID {
			return s, nil
		}
	}
	return domain.Snapshot{}, contract.ErrNotFound
}

func (m *memResults) InsertResult(_ context.Context, r domain.Result) error {
	m.results[r.ID] = r
	return nil
}

func (m *memResults) GetResultForUpdate(_ context.Context, id uuid.UUID) (domain.Result, error) {
	r, ok := m.results[id]
	if !ok || m.deletedR[id] {
		return domain.Result{}, contract.ErrNotFound
	}
	return r, nil
}

func (m *memResults) UpdateResult(_ context.Context, r domain.Result, _ bool) error {
	m.results[r.ID] = r
	return nil
}

func (m *memResults) SoftDeleteResult(_ context.Context, id uuid.UUID) error {
	m.deletedR[id] = true
	return nil
}

type resultFixture struct {
	repo    *memResults
	h       *ResultHandler
	owner   uuid.UUID // template author
	visitor uuid.UUID
}

func newResultFixture(quota int) *resultFixture {
	f := &resultFixture{repo: newMemResults(), owner: uuid.New(), visitor: uuid.New()}
	f.h = NewResultHandler(f.repo, inlineTx{}, quota)
	return f
}

// published adds a published template of the owner with three items in the given format.
func (f *resultFixture) published(t *testing.T, format domain.Format) domain.Template {
	t.Helper()
	tpl, err := domain.NewTemplate(uuid.New(), f.owner, "game", format, "Игры", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c := tpl.Content()
	for _, title := range []string{"Halo", "Doom", "Quake"} {
		c.Items = append(c.Items, domain.Item{ID: uuid.New(), Title: title, ImageURL: "https://cdn.example/" + title + ".jpg"})
	}
	if err := tpl.Replace(c); err != nil {
		t.Fatal(err)
	}
	if err := tpl.Publish(domain.VisibilityUnlisted, time.Now()); err != nil {
		t.Fatal(err)
	}
	f.repo.templates[tpl.ID] = tpl
	return tpl
}

func TestCreateResult(t *testing.T) {
	t.Parallel()
	f := newResultFixture(2)
	ctx := context.Background()
	tpl := f.published(t, domain.FormatRanked)

	// Unlisted templates accept results from their own page.
	res, err := f.h.Create(ctx, tpl.ID, f.visitor)
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "Игры" || res.Status != domain.StatusDraft || res.Format != domain.FormatRanked || res.Revision != 1 || len(res.Entries) != 0 {
		t.Fatalf("result = %+v", res)
	}
	// The same revision reuses one snapshot.
	again, err := f.h.Create(ctx, tpl.ID, f.visitor)
	if err != nil || again.VersionID != res.VersionID {
		t.Fatalf("second result: %v, version %v != %v", err, again.VersionID, res.VersionID)
	}
	if _, err := f.h.Create(ctx, tpl.ID, f.visitor); !errors.Is(err, contract.ErrQuotaExceeded) {
		t.Fatalf("over quota: %v", err)
	}
	if err := f.h.Delete(ctx, again.ID, f.visitor); err != nil {
		t.Fatal(err)
	}
	if _, err := f.h.Create(ctx, tpl.ID, f.visitor); err != nil {
		t.Fatalf("deleted results free the quota: %v", err)
	}

	draft := tpl
	draft.Unpublish()
	f.repo.templates[tpl.ID] = draft
	if _, err := f.h.Create(ctx, tpl.ID, uuid.New()); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("foreign draft template: %v", err)
	}
	if _, err := f.h.Create(ctx, tpl.ID, f.owner); !errors.Is(err, contract.ErrConflict) {
		t.Fatalf("own draft template: %v", err)
	}
	hidden := tpl
	hidden.ModerationHiddenAt = tpl.FirstPublishedAt
	f.repo.templates[tpl.ID] = hidden
	if _, err := f.h.Create(ctx, tpl.ID, uuid.New()); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("foreign moderated template: %v", err)
	}
	if _, err := f.h.Create(ctx, tpl.ID, f.owner); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("own moderated template: %v", err)
	}
	f.repo.deletedT[tpl.ID] = true
	if _, err := f.h.Create(ctx, tpl.ID, f.owner); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("deleted template: %v", err)
	}
}

func TestSaveResultTiers(t *testing.T) {
	t.Parallel()
	f := newResultFixture(0)
	ctx := context.Background()
	tpl := f.published(t, domain.FormatTiers)
	res, err := f.h.Create(ctx, tpl.ID, f.visitor)
	if err != nil {
		t.Fatal(err)
	}
	s, a := tpl.Tiers[0].ID, tpl.Tiers[1].ID
	halo, doom := tpl.Items[0].ID, tpl.Items[1].ID

	saved, err := f.h.Save(ctx, res.ID, f.visitor, SaveResultInput{
		Revision: 1, Title: " Мой топ ",
		Entries: []EntryInput{{ItemID: doom, TierID: &a, Position: 1}, {ItemID: halo, TierID: &s, Position: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 2 || saved.Title != "Мой топ" || saved.Entries[0].ItemID != halo {
		t.Fatalf("saved = %+v", saved)
	}

	cases := map[string]struct {
		entries []EntryInput
		field   string
		reason  string
	}{
		"foreign item":   {[]EntryInput{{ItemID: uuid.New(), TierID: &s, Position: 1}}, "entries.itemId", contract.ReasonInvalid},
		"duplicate item": {[]EntryInput{{ItemID: halo, TierID: &s, Position: 1}, {ItemID: halo, TierID: &a, Position: 1}}, "entries.itemId", contract.ReasonDuplicate},
		"foreign tier":   {[]EntryInput{{ItemID: halo, TierID: new(uuid.New()), Position: 1}}, "entries.tierId", contract.ReasonInvalid},
		"missing tier":   {[]EntryInput{{ItemID: halo, Position: 1}}, "entries.tierId", contract.ReasonRequired},
		"gap":            {[]EntryInput{{ItemID: halo, TierID: &s, Position: 2}}, "entries.position", contract.ReasonInvalid},
		"too many":       {make([]EntryInput, domain.MaxItems+1), "entries", contract.ReasonTooMany},
	}
	for name, tc := range cases {
		_, err := f.h.Save(ctx, res.ID, f.visitor, SaveResultInput{Revision: 2, Entries: tc.entries})
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}
		wantField(t, err, tc.field, tc.reason)
	}
	if got := f.repo.results[res.ID]; got.Revision != 2 || len(got.Entries) != 2 {
		t.Fatalf("rejected saves changed the result: %+v", got)
	}
	if _, err := f.h.Save(ctx, res.ID, f.visitor, SaveResultInput{Revision: 1}); !errors.Is(err, contract.ErrRevisionConflict) {
		t.Fatalf("stale revision: %v", err)
	}
	if _, err := f.h.Save(ctx, res.ID, f.owner, SaveResultInput{Revision: 2}); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("foreign result: %v", err)
	}
}

func TestSaveResultIgnoresLaterTemplateEdits(t *testing.T) {
	t.Parallel()
	f := newResultFixture(0)
	ctx := context.Background()
	tpl := f.published(t, domain.FormatCollection)
	res, err := f.h.Create(ctx, tpl.ID, f.visitor)
	if err != nil {
		t.Fatal(err)
	}
	// The template drops an item and gets a new revision; the result keeps its snapshot.
	edited := tpl
	c := edited.Content()
	c.Items = c.Items[1:]
	if err := edited.Replace(c); err != nil {
		t.Fatal(err)
	}
	edited.Revision++
	f.repo.templates[tpl.ID] = edited
	if _, err := f.h.Save(ctx, res.ID, f.visitor, SaveResultInput{
		Revision: 1, Title: "Подборка", Entries: []EntryInput{{ItemID: tpl.Items[0].ID, Position: 1}},
	}); err != nil {
		t.Fatalf("item of the snapshot: %v", err)
	}
	next, err := f.h.Create(ctx, tpl.ID, f.visitor)
	if err != nil || next.VersionID == res.VersionID {
		t.Fatalf("new revision must get a new snapshot: %v", err)
	}
}

func TestResultPublishAndCopy(t *testing.T) {
	t.Parallel()
	f := newResultFixture(0)
	ctx := context.Background()
	tpl := f.published(t, domain.FormatRanked)
	res, err := f.h.Create(ctx, tpl.ID, f.visitor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.h.Publish(ctx, res.ID, f.visitor, 1, ""); err == nil {
		t.Fatal("empty result published")
	}
	if _, err := f.h.Save(ctx, res.ID, f.visitor, SaveResultInput{
		Revision: 1, Title: "Топ", Entries: []EntryInput{{ItemID: tpl.Items[2].ID, Position: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.h.Publish(ctx, res.ID, f.visitor, 1, ""); !errors.Is(err, contract.ErrRevisionConflict) {
		t.Fatalf("publish stale revision: %v", err)
	}

	// An unpublished and deleted template does not block publication.
	draft := tpl
	draft.Unpublish()
	f.repo.templates[tpl.ID] = draft
	f.repo.deletedT[tpl.ID] = true
	pub, err := f.h.Publish(ctx, res.ID, f.visitor, 2, "public")
	if err != nil || pub.Status != domain.StatusPublished || pub.FirstPublishedAt == nil || pub.Revision != 2 {
		t.Fatalf("publish: %+v %v", pub, err)
	}
	if _, err := f.h.SetVisibility(ctx, res.ID, f.visitor, "unlisted"); err != nil {
		t.Fatal(err)
	}
	cp, err := f.h.Copy(ctx, res.ID, f.visitor)
	if err != nil {
		t.Fatal(err)
	}
	if cp.VersionID != res.VersionID || cp.Status != domain.StatusDraft || len(cp.Entries) != 1 || cp.FirstPublishedAt != nil {
		t.Fatalf("copy = %+v", cp)
	}
	if _, err := f.h.Copy(ctx, res.ID, f.owner); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("copy of a foreign result: %v", err)
	}
	if unp, err := f.h.Unpublish(ctx, res.ID, f.visitor); err != nil || unp.Status != domain.StatusDraft {
		t.Fatalf("unpublish: %v", err)
	}
	if _, err := f.h.SetVisibility(ctx, res.ID, f.visitor, "public"); !errors.Is(err, contract.ErrConflict) {
		t.Fatalf("visibility of a draft: %v", err)
	}

	// A template hidden by a moderator blocks publication and copies of every result on it.
	hidden := draft
	hidden.ModerationHiddenAt = tpl.FirstPublishedAt
	f.repo.templates[tpl.ID] = hidden
	if _, err := f.h.Publish(ctx, res.ID, f.visitor, 2, ""); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("publish with moderated template: %v", err)
	}
	if _, err := f.h.Copy(ctx, res.ID, f.visitor); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("copy with moderated template: %v", err)
	}
	// Saving stays possible: the author may still edit a draft that nobody else sees.
	if _, err := f.h.Save(ctx, res.ID, f.visitor, SaveResultInput{Revision: 2, Title: "Топ"}); err != nil {
		t.Fatalf("save with moderated template: %v", err)
	}
}
