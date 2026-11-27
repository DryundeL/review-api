package write_test

import (
	"context"
	"slices"
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

func TestListQueriesIntegration(t *testing.T) {
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
	halo, doom, dune := uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO media_objects (id, title, type, info, poster_url) VALUES
		($1, 'Halo', 'game', '{"releaseStatus":"completed"}', 'https://cdn.example/halo.jpg'),
		($2, 'Doom', 'game', '{"releaseStatus":"completed"}', 'https://cdn.example/doom.jpg'),
		($3, 'Dune', 'movie', '{"releaseStatus":"completed"}', 'https://cdn.example/dune.jpg')`, halo, doom, dune)

	repo := write.NewRepository(pool)
	tx := transaction.NewPgxManager(pool)
	catalog := dbCatalog{
		halo: {ID: halo, Type: "game", Title: "Halo", PosterURL: "https://cdn.example/halo.jpg"},
		doom: {ID: doom, Type: "game", Title: "Doom", PosterURL: "https://cdn.example/doom.jpg"},
		dune: {ID: dune, Type: "movie", Title: "Dune", PosterURL: "https://cdn.example/dune.jpg"},
	}
	createTpl := command.NewCreateTemplateHandler(repo, anyType{}, tx, 20)
	saveTpl := command.NewSaveTemplateHandler(repo, catalog, realImagesStub{}, anyType{}, tx)
	stateTpl := command.NewTemplateStateHandler(repo, catalog, tx)
	results := command.NewResultHandler(repo, tx, 20)
	lists := query.NewListHandler(read.NewRepository(pool))

	template := func(title, typeCode, format string, works []uuid.UUID, visibility string) domain.Template {
		t.Helper()
		tpl, err := createTpl.Handle(ctx, author, command.CreateTemplateInput{TypeCode: typeCode, Format: format, Title: title})
		if err != nil {
			t.Fatal(err)
		}
		in := command.SaveTemplateInput{Revision: 1, Title: title}
		for _, tier := range tpl.Tiers {
			in.Tiers = append(in.Tiers, command.TierInput{ID: &tier.ID, Name: tier.Name, Color: tier.Color})
		}
		for _, w := range works {
			in.Items = append(in.Items, command.ItemInput{MediaObjectID: &w})
		}
		if tpl, err = saveTpl.Handle(ctx, tpl.ID, author, in); err != nil {
			t.Fatal(err)
		}
		if visibility == "" {
			return tpl
		}
		if tpl, err = stateTpl.Publish(ctx, tpl.ID, author, tpl.Revision, visibility); err != nil {
			t.Fatal(err)
		}
		return tpl
	}
	// result places the template items in reverse order: in tiers the last item goes to the top tier.
	result := func(tpl domain.Template, title, visibility string) domain.Result {
		t.Helper()
		res, err := results.Create(ctx, tpl.ID, fan)
		if err != nil {
			t.Fatal(err)
		}
		in := command.SaveResultInput{Revision: 1, Title: title}
		for i := range tpl.Items {
			item := tpl.Items[len(tpl.Items)-1-i]
			e := command.EntryInput{ItemID: item.ID, Position: i + 1}
			if tpl.Format == domain.FormatTiers {
				e.TierID, e.Position = &tpl.Tiers[i].ID, 1
			}
			in.Entries = append(in.Entries, e)
		}
		if res, err = results.Save(ctx, res.ID, fan, in); err != nil {
			t.Fatal(err)
		}
		if visibility == "" {
			return res
		}
		if res, err = results.Publish(ctx, res.ID, fan, res.Revision, visibility); err != nil {
			t.Fatal(err)
		}
		return res
	}

	games := template("Игры", "game", "tiers", []uuid.UUID{halo, doom}, "public")
	hits := template("100% хиты_", "movie", "ranked", []uuid.UUID{dune}, "public")
	secret := template("Секрет", "game", "collection", []uuid.UUID{halo}, "unlisted")
	draft := template("Черновик", "game", "tiers", []uuid.UUID{halo}, "")
	banned := template("Бан", "game", "ranked", []uuid.UUID{doom}, "public")
	gone := template("Удалён", "game", "collection", []uuid.UUID{doom, halo}, "public")

	rPublic := result(games, "Мой топ", "public")
	rUnlisted := result(secret, "Тайный", "unlisted")
	rDraft := result(hits, "Набросок", "")
	rBanned := result(banned, "Под баном", "public")
	rGone := result(gone, "Сирота", "public")

	exec(`UPDATE list_templates SET moderation_hidden_at = NOW() WHERE id = $1`, banned.ID)
	if err := stateTpl.Delete(ctx, gone.ID, author); err != nil {
		t.Fatal(err)
	}

	ids := func(page query.ListPage, err error) []uuid.UUID {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		out := make([]uuid.UUID, len(page.Items))
		for i, c := range page.Items {
			out[i] = c.ID
		}
		return out
	}
	want := func(name string, got []uuid.UUID, expected ...uuid.UUID) {
		t.Helper()
		if !slices.Equal(got, expected) {
			t.Fatalf("%s: got %v, want %v", name, got, expected)
		}
	}

	// Section: only published public, not hidden and not deleted; newest first publication first.
	want("templates", ids(lists.Templates(ctx, query.ListInput{})), hits.ID, games.ID)
	want("type filter", ids(lists.Templates(ctx, query.ListInput{Type: "movie"})), hits.ID)
	want("format filter", ids(lists.Templates(ctx, query.ListInput{Format: "tiers"})), games.ID)
	want("search is case-insensitive", ids(lists.Templates(ctx, query.ListInput{Search: "ИГР"})), games.ID)
	want("percent is literal", ids(lists.Templates(ctx, query.ListInput{Search: "%"})), hits.ID)
	want("underscore is literal", ids(lists.Templates(ctx, query.ListInput{Search: "_"})), hits.ID)

	// A deleted or unpublished template does not hide its results; a moderated one does.
	want("results", ids(lists.Results(ctx, query.ListInput{})), rGone.ID, rPublic.ID)
	want("result format from snapshot", ids(lists.Results(ctx, query.ListInput{Format: "collection"})), rGone.ID)
	want("result type from snapshot", ids(lists.Results(ctx, query.ListInput{Type: "movie"})), nil...)
	want("result search", ids(lists.Results(ctx, query.ListInput{Search: "сирота"})), rGone.ID)

	page, err := lists.Results(ctx, query.ListInput{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	card := page.Items[0]
	if card.TemplateTitle != "Удалён" || card.TemplateID != gone.ID || card.Author.Username != "fan" ||
		card.CoverURL != "https://cdn.example/halo.jpg" || card.Format != domain.FormatCollection || card.TypeCode != "game" {
		t.Fatalf("result card = %+v", card)
	}
	want("second page", ids(lists.Results(ctx, query.ListInput{Limit: 1, Cursor: page.NextCursor})), rPublic.ID)
	tiersPage, err := lists.Results(ctx, query.ListInput{Format: "tiers"})
	if err != nil || tiersPage.Items[0].CoverURL != "https://cdn.example/doom.jpg" {
		t.Fatalf("tiers cover must come from the top tier: %+v %v", tiersPage.Items, err)
	}
	tplPage, err := lists.Templates(ctx, query.ListInput{Format: "tiers"})
	if err != nil || tplPage.Items[0].CoverURL != "https://cdn.example/halo.jpg" || tplPage.Items[0].Author.Username != "ann" {
		t.Fatalf("template card = %+v %v", tplPage.Items, err)
	}

	// Profiles: the same rules, narrowed to the author.
	want("profile templates", ids(lists.ProfileTemplates(ctx, author, query.ListInput{})), hits.ID, games.ID)
	want("profile results of the template author", ids(lists.ProfileResults(ctx, author, query.ListInput{})), nil...)
	want("profile results", ids(lists.ProfileResults(ctx, fan, query.ListInput{})), rGone.ID, rPublic.ID)

	// The author's own list: every live object, by last update; moderation wins over status.
	want("mine draft", ids(lists.MyResults(ctx, fan, query.MineInput{Status: "draft"})), rDraft.ID)
	want("mine published", ids(lists.MyResults(ctx, fan, query.MineInput{Status: "published"})), rGone.ID, rPublic.ID)
	want("mine unlisted", ids(lists.MyResults(ctx, fan, query.MineInput{Status: "unlisted"})), rUnlisted.ID)
	want("mine hidden", ids(lists.MyResults(ctx, fan, query.MineInput{Status: "hidden"})), rBanned.ID)
	all := ids(lists.MyResults(ctx, fan, query.MineInput{}))
	if len(all) != 5 {
		t.Fatalf("mine all = %v", all)
	}
	want("mine templates hidden", ids(lists.MyTemplates(ctx, author, query.MineInput{Status: "hidden"})), banned.ID)
	want("mine templates draft", ids(lists.MyTemplates(ctx, author, query.MineInput{Status: "draft"})), draft.ID)
	want("mine templates of another user", ids(lists.MyTemplates(ctx, fan, query.MineInput{})), nil...)

	// Editing bumps a result to the top of the author's list, but not of the section.
	if _, err := results.Save(ctx, rPublic.ID, fan, command.SaveResultInput{
		Revision: rPublic.Revision, Title: "Мой топ 2026",
		Entries: []command.EntryInput{{ItemID: games.Items[0].ID, TierID: &games.Tiers[0].ID, Position: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	mine, err := lists.MyResults(ctx, fan, query.MineInput{Limit: 1})
	if err != nil || mine.Items[0].ID != rPublic.ID || mine.Items[0].Status != domain.StatusPublished {
		t.Fatalf("mine after edit: %+v %v", mine.Items, err)
	}
	want("section order is by first publication", ids(lists.Results(ctx, query.ListInput{})), rGone.ID, rPublic.ID)
	var seen []uuid.UUID
	for cursor := ""; ; {
		p, err := lists.MyResults(ctx, fan, query.MineInput{Limit: 2, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range p.Items {
			seen = append(seen, c.ID)
		}
		if p.NextCursor == "" {
			break
		}
		cursor = p.NextCursor
	}
	if len(seen) != 5 || len(slices.Compact(slices.SortedFunc(slices.Values(seen), func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) }))) != 5 {
		t.Fatalf("paging mine = %v", seen)
	}
}
