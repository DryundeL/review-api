package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
)

type inlineTx struct{}

func (inlineTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// memTemplates keeps templates by value; a failed command must not leave partial writes,
// so writes happen only through InsertTemplate/UpdateTemplate.
type memTemplates struct {
	items   map[uuid.UUID]domain.Template
	deleted map[uuid.UUID]bool
	locks   int
}

func newMem() *memTemplates {
	return &memTemplates{items: map[uuid.UUID]domain.Template{}, deleted: map[uuid.UUID]bool{}}
}

func (m *memTemplates) LockAuthor(context.Context, uuid.UUID) error { m.locks++; return nil }

func (m *memTemplates) CountTemplates(_ context.Context, authorID uuid.UUID) (int, error) {
	n := 0
	for id, t := range m.items {
		if t.AuthorID == authorID && !m.deleted[id] {
			n++
		}
	}
	return n, nil
}

func (m *memTemplates) InsertTemplate(_ context.Context, t domain.Template) error {
	m.items[t.ID] = t
	return nil
}

func (m *memTemplates) GetTemplateForUpdate(_ context.Context, id uuid.UUID) (domain.Template, error) {
	t, ok := m.items[id]
	if !ok || m.deleted[id] {
		return domain.Template{}, contract.ErrNotFound
	}
	return t, nil
}

func (m *memTemplates) UpdateTemplate(_ context.Context, t domain.Template, _ bool) error {
	m.items[t.ID] = t
	return nil
}

func (m *memTemplates) SoftDeleteTemplate(_ context.Context, id uuid.UUID) error {
	m.deleted[id] = true
	return nil
}

type fakeCatalog map[uuid.UUID]ports.CatalogWork

func (f fakeCatalog) Work(_ context.Context, id uuid.UUID) (ports.CatalogWork, error) {
	w, ok := f[id]
	if !ok {
		return ports.CatalogWork{}, contract.ErrNotFound
	}
	return w, nil
}

// fakeImages owns uploads by author and records every attach call.
type fakeImages struct {
	owner    map[uuid.UUID]uuid.UUID
	attached [][]uuid.UUID
}

func (f *fakeImages) Attach(_ context.Context, ownerID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.attached = append(f.attached, ids)
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if f.owner[id] != ownerID {
			return nil, contract.ErrNotFound
		}
		out[id] = "/storage/images/" + id.String() + ".png"
	}
	return out, nil
}

type fakeTypes map[string]bool

func (f fakeTypes) RequireActive(_ context.Context, code string) error {
	if !f[code] {
		return contract.Field("typeCode", contract.ReasonInvalid)
	}
	return nil
}

type fixture struct {
	repo    *memTemplates
	catalog fakeCatalog
	images  *fakeImages
	create  *CreateTemplateHandler
	save    *SaveTemplateHandler
	state   *TemplateStateHandler
	copy    *CopyTemplateHandler
	author  uuid.UUID
	halo    uuid.UUID // a game
	dune    uuid.UUID // a movie
}

func newFixture(quota int) *fixture {
	f := &fixture{
		repo: newMem(), author: uuid.New(), halo: uuid.New(), dune: uuid.New(),
		images: &fakeImages{owner: map[uuid.UUID]uuid.UUID{}},
	}
	f.catalog = fakeCatalog{
		f.halo: {ID: f.halo, Type: "game", Title: "Halo", PosterURL: "https://cdn.example/halo.jpg"},
		f.dune: {ID: f.dune, Type: "movie", Title: "Dune", PosterURL: "https://cdn.example/dune.jpg"},
	}
	types := fakeTypes{"game": true, "movie": true}
	f.create = NewCreateTemplateHandler(f.repo, types, inlineTx{}, quota)
	f.save = NewSaveTemplateHandler(f.repo, f.catalog, f.images, types, inlineTx{})
	f.state = NewTemplateStateHandler(f.repo, f.catalog, inlineTx{})
	f.copy = NewCopyTemplateHandler(f.repo, inlineTx{}, quota)
	return f
}

func (f *fixture) upload(owner uuid.UUID) uuid.UUID {
	id := uuid.New()
	f.images.owner[id] = owner
	return id
}

func (f *fixture) draft(t *testing.T, format string) domain.Template {
	t.Helper()
	tpl, err := f.create.Handle(context.Background(), f.author, CreateTemplateInput{TypeCode: "game", Format: format, Title: "Игры"})
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func wantField(t *testing.T, err error, field, reason string) {
	t.Helper()
	fe, ok := errors.AsType[contract.FieldError](err)
	if !ok || fe.Field != field || fe.Reason != reason {
		t.Fatalf("err = %v, want %s/%s", err, field, reason)
	}
}

func ptr[T any](v T) *T { return &v }

func TestCreateTemplate(t *testing.T) {
	t.Parallel()
	f := newFixture(2)
	ctx := context.Background()
	tpl := f.draft(t, "")
	if tpl.Format != domain.FormatTiers || len(tpl.Tiers) != 5 || tpl.Revision != 1 {
		t.Fatalf("draft = %+v", tpl)
	}
	_, err := f.create.Handle(ctx, f.author, CreateTemplateInput{TypeCode: "anime"})
	wantField(t, err, "typeCode", contract.ReasonInvalid)
	_, err = f.create.Handle(ctx, f.author, CreateTemplateInput{TypeCode: "game", Format: "pyramid"})
	wantField(t, err, "format", contract.ReasonInvalid)

	f.draft(t, "ranked")
	if _, err := f.create.Handle(ctx, f.author, CreateTemplateInput{TypeCode: "game"}); !errors.Is(err, contract.ErrQuotaExceeded) {
		t.Fatalf("third template: %v", err)
	}
	if _, err := f.copy.Handle(ctx, tpl.ID, f.author); !errors.Is(err, contract.ErrQuotaExceeded) {
		t.Fatalf("copy over quota: %v", err)
	}
	// Deleted templates free the quota.
	if err := f.state.Delete(ctx, tpl.ID, f.author); err != nil {
		t.Fatal(err)
	}
	f.draft(t, "collection")
}

func TestSaveTemplateItems(t *testing.T) {
	t.Parallel()
	f := newFixture(0)
	ctx := context.Background()
	tpl := f.draft(t, "collection")
	img := f.upload(f.author)

	saved, err := f.save.Handle(ctx, tpl.ID, f.author, SaveTemplateInput{
		Revision: 1, Title: "Игры",
		Items: []ItemInput{
			{MediaObjectID: &f.halo},            // title and poster from the catalog
			{Title: "Своя игра", ImageID: &img}, // manual item with an upload
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 2 || len(saved.Items) != 2 {
		t.Fatalf("saved = %+v", saved)
	}
	halo, manual := saved.Items[0], saved.Items[1]
	if halo.Title != "Halo" || halo.ImageURL != "https://cdn.example/halo.jpg" || halo.ImageID != nil || *halo.MediaObjectID != f.halo {
		t.Fatalf("catalog item = %+v", halo)
	}
	if manual.ImageID == nil || *manual.ImageID != img || manual.ImageURL == "" {
		t.Fatalf("manual item = %+v", manual)
	}

	// Re-saving with the same image does not attach it again; the catalog link stays.
	f.images.attached = nil
	saved, err = f.save.Handle(ctx, tpl.ID, f.author, SaveTemplateInput{
		Revision: 2, Title: "Игры",
		Items: []ItemInput{{ID: &manual.ID, Title: "Своя", ImageID: &img}, {ID: &halo.ID, Title: "Halo 1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.images.attached) != 0 {
		t.Fatalf("attached again: %v", f.images.attached)
	}
	if saved.Items[0].ID != manual.ID || saved.Items[1].Title != "Halo 1" || saved.Items[1].MediaObjectID == nil {
		t.Fatalf("reordered = %+v", saved.Items)
	}

	cases := map[string]struct {
		items  []ItemInput
		field  string
		reason string
	}{
		"unknown item id":     {[]ItemInput{{ID: ptr(uuid.New()), Title: "x"}}, "items.id", contract.ReasonInvalid},
		"relink catalog item": {[]ItemInput{{ID: &halo.ID, MediaObjectID: &f.dune}}, "items.mediaObjectId", contract.ReasonImmutable},
		"unknown work":        {[]ItemInput{{MediaObjectID: ptr(uuid.New())}}, "items.mediaObjectId", contract.ReasonInvalid},
		"work of other type":  {[]ItemInput{{MediaObjectID: &f.dune}}, "items.mediaObjectId", contract.ReasonMismatch},
		"same work twice":     {[]ItemInput{{ID: &halo.ID}, {MediaObjectID: &f.halo}}, "items.mediaObjectId", contract.ReasonDuplicate},
		"foreign upload":      {[]ItemInput{{Title: "x", ImageID: ptr(f.upload(uuid.New()))}}, "items.imageId", contract.ReasonInvalid},
		"nil upload id":       {[]ItemInput{{Title: "x", ImageID: &uuid.Nil}}, "items.imageId", contract.ReasonInvalid},
		"too many items":      {make([]ItemInput, domain.MaxItems+1), "items", contract.ReasonTooMany},
	}
	for name, tc := range cases {
		_, err := f.save.Handle(ctx, tpl.ID, f.author, SaveTemplateInput{Revision: 3, Items: tc.items})
		if err == nil {
			t.Fatalf("%s: accepted", name)
		}
		wantField(t, err, tc.field, tc.reason)
	}
	if got := f.repo.items[tpl.ID]; got.Revision != 3 || len(got.Items) != 2 {
		t.Fatalf("rejected saves changed the template: %+v", got)
	}
}

func TestSaveTemplateRevisionAndOwner(t *testing.T) {
	t.Parallel()
	f := newFixture(0)
	ctx := context.Background()
	tpl := f.draft(t, "ranked")
	if _, err := f.save.Handle(ctx, tpl.ID, f.author, SaveTemplateInput{Revision: 2}); !errors.Is(err, contract.ErrRevisionConflict) {
		t.Fatalf("stale revision: %v", err)
	}
	if _, err := f.save.Handle(ctx, tpl.ID, uuid.New(), SaveTemplateInput{Revision: 1}); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("foreign template: %v", err)
	}
	for _, fn := range []func() error{
		func() error { _, err := f.state.Unpublish(ctx, tpl.ID, uuid.New()); return err },
		func() error { return f.state.Delete(ctx, tpl.ID, uuid.New()) },
		func() error { _, err := f.copy.Handle(ctx, tpl.ID, uuid.New()); return err },
	} {
		if err := fn(); !errors.Is(err, contract.ErrNotFound) {
			t.Fatalf("foreign action: %v", err)
		}
	}
}

func TestSaveTemplateTiersAndType(t *testing.T) {
	t.Parallel()
	f := newFixture(0)
	ctx := context.Background()
	tpl := f.draft(t, "tiers")
	s := tpl.Tiers[0]
	saved, err := f.save.Handle(ctx, tpl.ID, f.author, SaveTemplateInput{
		Revision: 1,
		Tiers:    []TierInput{{ID: &s.ID, Name: "Топ", Color: s.Color}, {Name: "Новый", Color: "#000000"}},
		Items:    []ItemInput{{MediaObjectID: &f.halo}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Tiers) != 2 || saved.Tiers[0].ID != s.ID || saved.Tiers[1].ID == uuid.Nil {
		t.Fatalf("tiers = %+v", saved.Tiers)
	}
	_, err = f.save.Handle(ctx, tpl.ID, f.author, SaveTemplateInput{Revision: 2, Tiers: []TierInput{{ID: ptr(uuid.New()), Name: "x", Color: "#000000"}}})
	wantField(t, err, "tiers.id", contract.ReasonInvalid)

	// Changing the type re-checks catalog items already in the template.
	in := SaveTemplateInput{Revision: 2, TypeCode: ptr("movie"), Tiers: []TierInput{{ID: &s.ID, Name: "S", Color: "#ffffff"}},
		Items: []ItemInput{{ID: &saved.Items[0].ID}}}
	_, err = f.save.Handle(ctx, tpl.ID, f.author, in)
	wantField(t, err, "items.mediaObjectId", contract.ReasonMismatch)
	in.TypeCode = ptr("anime")
	_, err = f.save.Handle(ctx, tpl.ID, f.author, in)
	wantField(t, err, "typeCode", contract.ReasonInvalid)
	in.TypeCode, in.Items = ptr("movie"), nil
	if saved, err = f.save.Handle(ctx, tpl.ID, f.author, in); err != nil || saved.TypeCode != "movie" {
		t.Fatalf("type change of a draft: %v", err)
	}
}

func TestTemplateStateAndRefresh(t *testing.T) {
	t.Parallel()
	f := newFixture(0)
	ctx := context.Background()
	tpl := f.draft(t, "ranked")
	tpl, err := f.save.Handle(ctx, tpl.ID, f.author, SaveTemplateInput{Revision: 1, Title: "Игры", Items: []ItemInput{{MediaObjectID: &f.halo, Title: "Мой Halo"}}})
	if err != nil {
		t.Fatal(err)
	}
	item := tpl.Items[0]
	if item.Title != "Мой Halo" {
		t.Fatalf("client title must override the catalog: %q", item.Title)
	}
	if _, err := f.state.Publish(ctx, tpl.ID, f.author, 1, ""); !errors.Is(err, contract.ErrRevisionConflict) {
		t.Fatalf("publish stale revision: %v", err)
	}
	if tpl, err = f.state.Publish(ctx, tpl.ID, f.author, 2, "unlisted"); err != nil || tpl.Status != domain.StatusPublished || tpl.Revision != 2 {
		t.Fatalf("publish: %+v %v", tpl, err)
	}

	f.catalog[f.halo] = ports.CatalogWork{ID: f.halo, Type: "game", Title: "Halo: CE", PosterURL: "https://cdn.example/halo2.jpg"}
	tpl, err = f.state.RefreshItem(ctx, tpl.ID, item.ID, f.author, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := tpl.Items[0]; got.Title != "Halo: CE" || got.ImageURL != "https://cdn.example/halo2.jpg" || tpl.Revision != 3 {
		t.Fatalf("refreshed = %+v rev %d", got, tpl.Revision)
	}
	f.catalog[f.halo] = ports.CatalogWork{ID: f.halo, Type: "game", Title: "Halo"}
	if tpl, err = f.state.RefreshItem(ctx, tpl.ID, item.ID, f.author, 3); err != nil || tpl.Items[0].ImageURL != "https://cdn.example/halo2.jpg" {
		t.Fatalf("missing poster must keep the image: %v", err)
	}
	if _, err := f.state.RefreshItem(ctx, tpl.ID, uuid.New(), f.author, 4); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("unknown item: %v", err)
	}

	if _, err := f.state.SetVisibility(ctx, tpl.ID, f.author, ""); err == nil {
		t.Fatal("empty visibility accepted")
	}
	if tpl, err = f.state.Unpublish(ctx, tpl.ID, f.author); err != nil || tpl.Status != domain.StatusDraft {
		t.Fatalf("unpublish: %v", err)
	}
	if _, err := f.state.SetVisibility(ctx, tpl.ID, f.author, "public"); !errors.Is(err, contract.ErrConflict) {
		t.Fatalf("visibility of a draft: %v", err)
	}

	// A moderated template can be neither republished nor copied.
	hidden := tpl
	hidden.ModerationHiddenAt = hidden.FirstPublishedAt
	f.repo.items[tpl.ID] = hidden
	if _, err := f.state.Publish(ctx, tpl.ID, f.author, 4, ""); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("publish moderated: %v", err)
	}
	if _, err := f.copy.Handle(ctx, tpl.ID, f.author); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("copy moderated: %v", err)
	}
}

func TestMixedTemplate(t *testing.T) {
	t.Parallel()
	f := newFixture(5)
	ctx := context.Background()
	// "mixed" is not in the type dictionary and must not be looked up there.
	tpl, err := f.create.Handle(ctx, f.author, CreateTemplateInput{TypeCode: domain.TypeMixed, Format: "ranked", Title: "Всё подряд"})
	if err != nil || tpl.TypeCode != domain.TypeMixed {
		t.Fatalf("create mixed = %+v, %v", tpl, err)
	}
	items := []ItemInput{{MediaObjectID: &f.halo}, {MediaObjectID: &f.dune}, {Title: "Пицца", ImageID: ptr(f.upload(f.author))}}
	saved, err := f.save.Handle(ctx, tpl.ID, f.author, SaveTemplateInput{Revision: tpl.Revision, Title: "Всё подряд", Items: items})
	if err != nil || len(saved.Items) != 3 {
		t.Fatalf("save mixed = %+v, %v", saved, err)
	}
	// Leaving "mixed" re-checks every catalog work against the new type.
	keep := []ItemInput{{ID: &saved.Items[0].ID}, {ID: &saved.Items[1].ID}, {ID: &saved.Items[2].ID}}
	_, err = f.save.Handle(ctx, tpl.ID, f.author, SaveTemplateInput{Revision: saved.Revision, TypeCode: ptr("game"), Title: "Игры", Items: keep})
	wantField(t, err, "items.mediaObjectId", contract.ReasonMismatch)
	if _, err := f.state.Publish(ctx, tpl.ID, f.author, saved.Revision, ""); err != nil {
		t.Fatalf("publish mixed: %v", err)
	}

	// A typed draft may become mixed and then take a work of another type.
	game := f.draft(t, "ranked")
	if _, err := f.save.Handle(ctx, game.ID, f.author, SaveTemplateInput{
		Revision: game.Revision, TypeCode: ptr(domain.TypeMixed), Title: "Смесь", Items: []ItemInput{{MediaObjectID: &f.dune}},
	}); err != nil {
		t.Fatalf("typed → mixed: %v", err)
	}
}
