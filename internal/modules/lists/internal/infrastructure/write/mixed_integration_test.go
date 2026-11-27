package write_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/internal/application/command"
	"review-api/internal/modules/lists/internal/application/query"
	"review-api/internal/modules/lists/internal/domain"
	"review-api/internal/modules/lists/internal/infrastructure/read"
	"review-api/internal/modules/lists/internal/infrastructure/write"
	"review-api/internal/platform/transaction"
)

// TestMixedTemplateIntegration: a mixed template is stored with a NULL type, read back as "mixed",
// found by type=mixed only, and its items carry the type of their catalog work.
func TestMixedTemplateIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := testPool(t, ctx)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	author, fan := uuid.New(), uuid.New()
	exec(`INSERT INTO users (id, telegram_id, first_name, username) VALUES ($1, 1, 'Ann', 'ann'), ($2, 2, 'Fan', 'fan')`, author, fan)
	halo, dune := uuid.New(), uuid.New()
	exec(`INSERT INTO media_objects (id, title, type, info, poster_url) VALUES
		($1, 'Halo', 'game', '{"releaseStatus":"completed"}', 'https://cdn.example/halo.jpg'),
		($2, 'Dune', 'movie', '{"releaseStatus":"completed"}', 'https://cdn.example/dune.jpg')`, halo, dune)

	repo := write.NewRepository(pool)
	reader := read.NewRepository(pool)
	tx := transaction.NewPgxManager(pool)
	catalog := dbCatalog{
		halo: {ID: halo, Type: "game", Title: "Halo", PosterURL: "https://cdn.example/halo.jpg"},
		dune: {ID: dune, Type: "movie", Title: "Dune", PosterURL: "https://cdn.example/dune.jpg"},
	}
	tpl, err := command.NewCreateTemplateHandler(repo, anyType{}, tx, 10).
		Handle(ctx, author, command.CreateTemplateInput{TypeCode: domain.TypeMixed, Format: "ranked", Title: "Всё подряд"})
	if err != nil {
		t.Fatal(err)
	}
	tpl, err = command.NewSaveTemplateHandler(repo, catalog, realImagesStub{}, anyType{}, tx).Handle(ctx, tpl.ID, author,
		command.SaveTemplateInput{Revision: 1, Title: "Всё подряд", Items: []command.ItemInput{{MediaObjectID: &halo}, {MediaObjectID: &dune}}})
	if err != nil {
		t.Fatal(err)
	}
	if tpl, err = command.NewTemplateStateHandler(repo, catalog, tx).Publish(ctx, tpl.ID, author, tpl.Revision, ""); err != nil {
		t.Fatal(err)
	}
	results := command.NewResultHandler(repo, tx, 10)
	res, err := results.Create(ctx, tpl.ID, fan)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := results.Save(ctx, res.ID, fan, command.SaveResultInput{Revision: 1, Title: "Мой микс", Entries: []command.EntryInput{
		{ItemID: tpl.Items[1].ID, Position: 1}, {ItemID: tpl.Items[0].ID, Position: 2},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := results.Publish(ctx, res.ID, fan, saved.Revision, ""); err != nil {
		t.Fatal(err)
	}

	// Storage keeps NULL; every reader says "mixed".
	var nullTemplates, nullVersions int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM list_templates WHERE id = $1 AND type_code IS NULL),
		(SELECT COUNT(*) FROM list_template_versions WHERE template_id = $1 AND type_code IS NULL)`, tpl.ID).
		Scan(&nullTemplates, &nullVersions); err != nil {
		t.Fatal(err)
	}
	if nullTemplates != 1 || nullVersions != 1 {
		t.Fatalf("NULL type_code: templates %d, versions %d", nullTemplates, nullVersions)
	}
	tv, _, err := query.NewGetTemplateHandler(reader).Handle(ctx, tpl.ID, author)
	if err != nil || tv.TypeCode != domain.TypeMixed || len(tv.Items) != 2 || tv.Items[0].MediaType != "game" || tv.Items[1].MediaType != "movie" {
		t.Fatalf("template = %+v, %v", tv, err)
	}
	get := query.NewGetResultHandler(reader)
	rv, _, err := get.Handle(ctx, res.ID, uuid.Nil)
	if err != nil || rv.TypeCode != domain.TypeMixed || len(rv.Snapshot.Items) != 2 || rv.Snapshot.Items[1].MediaType != "movie" {
		t.Fatalf("result = %+v, %v", rv, err)
	}

	// type=mixed finds both; a concrete type finds neither.
	lists := query.NewListHandler(reader)
	for _, tc := range []struct {
		typ  string
		want int
	}{{"mixed", 1}, {"game", 0}} {
		tp, err := lists.Templates(ctx, query.ListInput{Type: tc.typ})
		if err != nil || len(tp.Items) != tc.want || (tc.want == 1 && (tp.Items[0].ID != tpl.ID || tp.Items[0].TypeCode != domain.TypeMixed)) {
			t.Fatalf("templates type=%s: %+v %v", tc.typ, tp, err)
		}
		rp, err := lists.Results(ctx, query.ListInput{Type: tc.typ})
		if err != nil || len(rp.Items) != tc.want || (tc.want == 1 && (rp.Items[0].ID != res.ID || rp.Items[0].TypeCode != domain.TypeMixed)) {
			t.Fatalf("results type=%s: %+v %v", tc.typ, rp, err)
		}
	}

	// A work removed from the catalog leaves the item without a media type.
	exec(`DELETE FROM media_objects WHERE id = $1`, dune)
	rv, _, err = get.Handle(ctx, res.ID, uuid.Nil)
	if err != nil || len(rv.Snapshot.Items) != 2 || rv.Snapshot.Items[1].MediaType != "" || rv.Snapshot.Items[0].MediaType != "game" {
		t.Fatalf("result after catalog delete = %+v, %v", rv.Snapshot.Items, err)
	}
}
