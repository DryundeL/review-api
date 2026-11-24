package write_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/command"
	"review-api/internal/modules/lists/internal/application/query"
	"review-api/internal/modules/lists/internal/domain"
	"review-api/internal/modules/lists/internal/infrastructure/read"
	"review-api/internal/modules/lists/internal/infrastructure/write"
	"review-api/internal/platform/transaction"
)

func TestResultCommandsIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := testPool(t, ctx)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}

	author, fan, guest := uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO users (id, telegram_id, first_name, username) VALUES ($1, 1, 'Ann', 'ann'), ($2, 2, 'Fan', 'fan'), ($3, 3, 'Guest', NULL)`, author, fan, guest)
	halo, doom := uuid.New(), uuid.New()
	exec(`INSERT INTO media_objects (id, title, type, info, poster_url) VALUES
		($1, 'Halo', 'game', '{"releaseStatus":"completed"}', 'https://cdn.example/halo.jpg'),
		($2, 'Doom', 'game', '{"releaseStatus":"completed"}', 'https://cdn.example/doom.jpg')`, halo, doom)

	repo := write.NewRepository(pool)
	tx := transaction.NewPgxManager(pool)
	catalog := dbCatalog{
		halo: {ID: halo, Type: "game", Title: "Halo", PosterURL: "https://cdn.example/halo.jpg"},
		doom: {ID: doom, Type: "game", Title: "Doom", PosterURL: "https://cdn.example/doom.jpg"},
	}
	createTpl := command.NewCreateTemplateHandler(repo, anyType{}, tx, 10)
	saveTpl := command.NewSaveTemplateHandler(repo, catalog, realImagesStub{}, anyType{}, tx)
	stateTpl := command.NewTemplateStateHandler(repo, catalog, tx)
	results := command.NewResultHandler(repo, tx, 10)
	get := query.NewGetResultHandler(read.NewRepository(pool))

	tpl, err := createTpl.Handle(ctx, author, command.CreateTemplateInput{TypeCode: "game", Title: "Игры"})
	if err != nil {
		t.Fatal(err)
	}
	tiers := make([]command.TierInput, len(tpl.Tiers))
	for i, tier := range tpl.Tiers {
		tiers[i] = command.TierInput{ID: &tier.ID, Name: tier.Name, Color: tier.Color}
	}
	tpl, err = saveTpl.Handle(ctx, tpl.ID, author, command.SaveTemplateInput{
		Revision: 1, Title: "Игры", Tiers: tiers[:2],
		Items: []command.ItemInput{{MediaObjectID: &halo}, {MediaObjectID: &doom}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := results.Create(ctx, tpl.ID, fan); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("result on a draft template: %v", err)
	}
	if tpl, err = stateTpl.Publish(ctx, tpl.ID, author, tpl.Revision, ""); err != nil {
		t.Fatal(err)
	}

	// Concurrent creates on one revision share a single snapshot.
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		ids []uuid.UUID
	)
	for range 4 {
		wg.Go(func() {
			res, err := results.Create(ctx, tpl.ID, fan)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				t.Error(err)
				return
			}
			ids = append(ids, res.ID)
		})
	}
	wg.Wait()
	var versions int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM list_template_versions WHERE template_id = $1`, tpl.ID).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 4 || versions != 1 {
		t.Fatalf("results %d, versions %d", len(ids), versions)
	}
	resID := ids[0]
	s, a := tpl.Tiers[0].ID, tpl.Tiers[1].ID
	haloItem, doomItem := tpl.Items[0].ID, tpl.Items[1].ID

	saved, err := results.Save(ctx, resID, fan, command.SaveResultInput{
		Revision: 1, Title: "Мой топ",
		Entries: []command.EntryInput{{ItemID: doomItem, TierID: &a, Position: 1}, {ItemID: haloItem, TierID: &s, Position: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Re-saving with swapped tiers rewrites positions without unique conflicts.
	saved, err = results.Save(ctx, resID, fan, command.SaveResultInput{
		Revision: saved.Revision, Title: "Мой топ",
		Entries: []command.EntryInput{{ItemID: doomItem, TierID: &s, Position: 1}, {ItemID: haloItem, TierID: &s, Position: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// The template changes: items renamed, one removed, a tier dropped. The result keeps its snapshot.
	if _, err := saveTpl.Handle(ctx, tpl.ID, author, command.SaveTemplateInput{
		Revision: tpl.Revision, Title: "Игры v2", Tiers: tiers[1:2],
		Items: []command.ItemInput{{ID: &haloItem, Title: "Halo CE"}},
	}); err != nil {
		t.Fatal(err)
	}
	view, isOwner, err := get.Handle(ctx, resID, fan)
	if err != nil || !isOwner {
		t.Fatalf("owner view: %v", err)
	}
	if view.Snapshot.Title != "Игры" || len(view.Snapshot.Items) != 2 || view.Snapshot.Items[0].Title != "Halo" || len(view.Snapshot.Tiers) != 2 {
		t.Fatalf("snapshot changed: %+v", view.Snapshot)
	}
	if len(view.Entries) != 2 || view.Entries[0].ItemID != doomItem || view.Entries[1].Position != 2 || view.Revision != 3 {
		t.Fatalf("entries = %+v", view.Entries)
	}
	if view.TypeCode != "game" || view.TemplateAuthor.Username != "ann" || view.Author.Username != "fan" || !view.TemplatePublic {
		t.Fatalf("template data = %+v", view)
	}
	if got := view.CoverURL(view.Snapshot); got != "https://cdn.example/doom.jpg" {
		t.Fatalf("cover = %q", got)
	}
	if _, _, err := get.Handle(ctx, resID, guest); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("draft visible to others: %v", err)
	}

	// The foreign keys reject items and tiers outside the snapshot even past the domain.
	forge := func(e domain.Entry) error {
		forged := view.Result
		forged.Entries = []domain.Entry{e}
		return tx.WithinTransaction(ctx, func(txCtx context.Context) error { return repo.UpdateResult(txCtx, forged, true) })
	}
	if fe, ok := errors.AsType[contract.FieldError](forge(domain.Entry{ItemID: uuid.New(), TierID: &s, Position: 1})); !ok || fe.Field != "entries.itemId" {
		t.Fatal("entry outside the snapshot stored")
	}
	if fe, ok := errors.AsType[contract.FieldError](forge(domain.Entry{ItemID: haloItem, TierID: new(uuid.New()), Position: 1})); !ok || fe.Field != "entries.tierId" {
		t.Fatal("tier outside the snapshot stored")
	}

	// Removing a work from the catalog keeps the snapshot data.
	exec(`DELETE FROM media_objects WHERE id = $1`, doom)
	if view, _, err = get.Handle(ctx, resID, fan); err != nil || view.Snapshot.Items[1].Title != "Doom" || view.Snapshot.Items[1].MediaObjectID != nil {
		t.Fatalf("after catalog delete: %+v %v", view.Snapshot.Items, err)
	}

	// The template is unpublished and deleted: the draft can still be published and is public.
	if _, err := stateTpl.Unpublish(ctx, tpl.ID, author); err != nil {
		t.Fatal(err)
	}
	if err := stateTpl.Delete(ctx, tpl.ID, author); err != nil {
		t.Fatal(err)
	}
	if _, err := results.Create(ctx, tpl.ID, fan); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("result on a deleted template: %v", err)
	}
	if _, err := results.Publish(ctx, resID, fan, saved.Revision, "unlisted"); err != nil {
		t.Fatal(err)
	}
	if view, _, err = get.Handle(ctx, resID, uuid.Nil); err != nil || view.TemplatePublic || view.Snapshot.Title != "Игры" {
		t.Fatalf("guest view: %+v %v", view, err)
	}
	cp, err := results.Copy(ctx, resID, fan)
	if err != nil {
		t.Fatal(err)
	}
	if view, _, err = get.Handle(ctx, cp.ID, fan); err != nil || len(view.Entries) != 2 || view.VersionID != saved.VersionID || view.Status != domain.StatusDraft {
		t.Fatalf("copy = %+v %v", view, err)
	}

	// Hiding the template hides every result on it, including published ones; unhide restores them.
	exec(`UPDATE list_templates SET moderation_hidden_at = NOW() WHERE id = $1`, tpl.ID)
	if _, _, err := get.Handle(ctx, resID, guest); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("result of a hidden template visible: %v", err)
	}
	if view, _, err = get.Handle(ctx, resID, fan); err != nil || !view.TemplateModerated {
		t.Fatalf("author must see the moderation mark: %v", err)
	}
	if _, err := results.Copy(ctx, resID, fan); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("copy under moderation: %v", err)
	}
	if _, err := results.Publish(ctx, cp.ID, fan, cp.Revision, ""); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("publish under moderation: %v", err)
	}
	exec(`UPDATE list_templates SET moderation_hidden_at = NULL WHERE id = $1`, tpl.ID)
	if view, _, err = get.Handle(ctx, resID, guest); err != nil || view.Visibility != domain.VisibilityUnlisted {
		t.Fatalf("unhide must restore the result: %v", err)
	}

	// Soft delete: gone for everyone, the author included.
	if err := results.Delete(ctx, resID, guest); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}
	if err := results.Delete(ctx, resID, fan); err != nil {
		t.Fatal(err)
	}
	if _, _, err := get.Handle(ctx, resID, fan); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("deleted result visible: %v", err)
	}
}

// realImagesStub rejects every upload: the result test uses catalog posters only.
type realImagesStub struct{}

func (realImagesStub) Attach(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]string, error) {
	return nil, contract.ErrNotFound
}

//go:fix inline
func ptrID(id uuid.UUID) *uuid.UUID { return new(id) }
