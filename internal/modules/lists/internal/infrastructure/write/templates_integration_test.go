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
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/application/query"
	"review-api/internal/modules/lists/internal/domain"
	"review-api/internal/modules/lists/internal/infrastructure/read"
	"review-api/internal/modules/lists/internal/infrastructure/write"
	"review-api/internal/modules/media"
	mediacontract "review-api/internal/modules/media/contract"
	"review-api/internal/platform/transaction"
)

type dbCatalog map[uuid.UUID]ports.CatalogWork

func (c dbCatalog) Work(_ context.Context, id uuid.UUID) (ports.CatalogWork, error) {
	if w, ok := c[id]; ok {
		return w, nil
	}
	return ports.CatalogWork{}, contract.ErrNotFound
}

// realImages goes through the media module, so attach shares the template transaction.
type realImages struct{ m *media.Module }

func (r realImages) Attach(ctx context.Context, owner uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	refs, err := r.m.AttachImages(ctx, owner, ids)
	if errors.Is(err, mediacontract.ErrNotFound) {
		return nil, contract.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]string{}
	for _, ref := range refs {
		out[ref.ID] = ref.URL
	}
	return out, nil
}

type anyType struct{}

func (anyType) RequireActive(context.Context, string) error { return nil }

func TestTemplateCommandsIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := testPool(t, ctx)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}

	author, other := uuid.New(), uuid.New()
	exec(`INSERT INTO users (id, telegram_id, first_name, username) VALUES ($1, 1, 'Ann', 'ann'), ($2, 2, 'Bob', NULL)`, author, other)
	halo := uuid.New()
	exec(`INSERT INTO media_objects (id, title, type, info, poster_url) VALUES ($1, 'Halo', 'game', '{"releaseStatus":"completed"}', 'https://cdn.example/halo.jpg')`, halo)
	own, foreign := uuid.New(), uuid.New()
	exec(`INSERT INTO media_images (id, owner_id, storage_key, content_type, size_bytes, width, height) VALUES
		($1::uuid, $3, 'images/' || $1::text || '.png', 'image/png', 10, 1, 1),
		($2::uuid, $4, 'images/' || $2::text || '.png', 'image/png', 10, 1, 1)`, own, foreign, author, other)

	repo := write.NewRepository(pool)
	tx := transaction.NewPgxManager(pool)
	catalog := dbCatalog{halo: {ID: halo, Type: "game", Title: "Halo", PosterURL: "https://cdn.example/halo.jpg"}}
	images := realImages{media.New(media.Dependencies{Pool: pool, PublicPrefix: "/storage"})}
	create := command.NewCreateTemplateHandler(repo, anyType{}, tx, 3)
	save := command.NewSaveTemplateHandler(repo, catalog, images, anyType{}, tx)
	state := command.NewTemplateStateHandler(repo, catalog, tx)
	cp := command.NewCopyTemplateHandler(repo, tx, 3)
	get := query.NewGetTemplateHandler(read.NewRepository(pool))

	tpl, err := create.Handle(ctx, author, command.CreateTemplateInput{TypeCode: "game", Title: "Игры 2026"})
	if err != nil {
		t.Fatal(err)
	}
	tiers := make([]command.TierInput, len(tpl.Tiers))
	for i, tier := range tpl.Tiers {
		tiers[i] = command.TierInput{ID: &tier.ID, Name: tier.Name, Color: tier.Color}
	}
	attachedAt := func(id uuid.UUID) *time.Time {
		var at *time.Time
		if err := pool.QueryRow(ctx, `SELECT attached_at FROM media_images WHERE id = $1`, id).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}

	// A foreign upload rejects the whole save; nothing is attached or written.
	_, err = save.Handle(ctx, tpl.ID, author, command.SaveTemplateInput{
		Revision: 1, Title: "Игры", Tiers: tiers,
		Items: []command.ItemInput{{Title: "Своя", ImageID: &own}, {Title: "Чужая", ImageID: &foreign}},
	})
	if fe, ok := errors.AsType[contract.FieldError](err); !ok || fe.Field != "items.imageId" {
		t.Fatalf("foreign upload: %v", err)
	}
	if attachedAt(own) != nil || attachedAt(foreign) != nil {
		t.Fatal("rejected save attached images")
	}

	saved, err := save.Handle(ctx, tpl.ID, author, command.SaveTemplateInput{
		Revision: 1, Title: "Игры", Description: "Лучшие\nигры", Tiers: tiers[:3],
		Items: []command.ItemInput{{MediaObjectID: &halo}, {Title: "Своя", ImageID: &own}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if attachedAt(own) == nil {
		t.Fatal("upload is not attached")
	}

	// The viewer sees exactly what was stored, in order.
	view, isOwner, err := get.Handle(ctx, tpl.ID, author)
	if err != nil || !isOwner {
		t.Fatalf("owner view: %v", err)
	}
	if view.Revision != 2 || view.Description != "Лучшие\nигры" || len(view.Tiers) != 3 || view.Tiers[0].Name != "S" || view.Author.Username != "ann" {
		t.Fatalf("view = %+v", view)
	}
	if len(view.Items) != 2 || view.Items[0].Title != "Halo" || *view.Items[0].MediaObjectID != halo || view.Items[0].ImageID != nil ||
		view.Items[1].ImageURL != "/storage/images/"+own.String()+".png" || *view.Items[1].ImageID != own {
		t.Fatalf("items = %+v", view.Items)
	}
	if _, _, err := get.Handle(ctx, tpl.ID, other); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("draft visible to others: %v", err)
	}

	// Reorder and save again: positions are rewritten without unique conflicts.
	_, err = save.Handle(ctx, tpl.ID, author, command.SaveTemplateInput{
		Revision: 2, Title: "Игры", Tiers: []command.TierInput{tiers[2], tiers[0]},
		Items: []command.ItemInput{{ID: &saved.Items[1].ID, Title: "Своя", ImageID: &own}, {ID: &saved.Items[0].ID, Title: "Halo"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := save.Handle(ctx, tpl.ID, author, command.SaveTemplateInput{Revision: 2}); !errors.Is(err, contract.ErrRevisionConflict) {
		t.Fatalf("stale revision: %v", err)
	}

	published, err := state.Publish(ctx, tpl.ID, author, 3, "unlisted")
	if err != nil {
		t.Fatal(err)
	}
	if view, _, err = get.Handle(ctx, tpl.ID, uuid.Nil); err != nil || view.Visibility != domain.VisibilityUnlisted || view.Items[0].Title != "Своя" || view.Tiers[0].ID != *tiers[2].ID {
		t.Fatalf("guest view of unlisted: %+v %v", view, err)
	}

	// Copy keeps items and uploads (already attached) and starts as a draft.
	dup, err := cp.Handle(ctx, tpl.ID, author)
	if err != nil {
		t.Fatal(err)
	}
	if view, _, err = get.Handle(ctx, dup.ID, author); err != nil || view.Status != domain.StatusDraft || len(view.Items) != 2 || *view.Items[0].ImageID != own {
		t.Fatalf("copy = %+v %v", view, err)
	}

	// Moderation hides the template from everyone but the author.
	exec(`UPDATE list_templates SET moderation_hidden_at = NOW() WHERE id = $1`, tpl.ID)
	if _, _, err := get.Handle(ctx, tpl.ID, other); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("hidden template visible: %v", err)
	}
	if view, _, err = get.Handle(ctx, tpl.ID, author); err != nil || view.ModerationHiddenAt == nil {
		t.Fatalf("author must see the moderation mark: %v", err)
	}
	if _, err := state.Unpublish(ctx, tpl.ID, author); err != nil {
		t.Fatal(err)
	}
	if _, err := state.Publish(ctx, tpl.ID, author, published.Revision, ""); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("republish of hidden: %v", err)
	}

	// Soft delete: gone for everyone, the author included, and the quota slot is free.
	if err := state.Delete(ctx, tpl.ID, author); err != nil {
		t.Fatal(err)
	}
	if _, _, err := get.Handle(ctx, tpl.ID, author); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("deleted template visible: %v", err)
	}
	if err := state.Delete(ctx, tpl.ID, author); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}

	// Concurrent creates cannot pass the quota together: only the copy is alive, so 2 slots are left.
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		created  int
		rejected int
	)
	for range 6 {
		wg.Go(func() {
			_, err := create.Handle(ctx, author, command.CreateTemplateInput{TypeCode: "game"})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				created++
			case errors.Is(err, contract.ErrQuotaExceeded):
				rejected++
			default:
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if created != 2 || rejected != 4 {
		t.Fatalf("quota 3 with 1 alive template: created %d, rejected %d", created, rejected)
	}
}
