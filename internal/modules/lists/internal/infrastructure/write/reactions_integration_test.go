package write_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/command"
	"review-api/internal/modules/lists/internal/application/query"
	"review-api/internal/modules/lists/internal/infrastructure/read"
	"review-api/internal/modules/lists/internal/infrastructure/write"
	"review-api/internal/platform/transaction"
)

// publishedResult builds a published ranked template of one catalog work and a published result of fan on it.
func publishedResult(t *testing.T, ctx context.Context, pool *pgxpool.Pool, author, fan uuid.UUID) (templateID, resultID uuid.UUID) {
	t.Helper()
	work := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO media_objects (id, title, type, info, poster_url)
		VALUES ($1, 'Halo', 'game', '{"releaseStatus":"completed"}', 'https://cdn.example/halo.jpg')`, work); err != nil {
		t.Fatal(err)
	}
	repo := write.NewRepository(pool)
	tx := transaction.NewPgxManager(pool)
	catalog := dbCatalog{work: {ID: work, Type: "game", Title: "Halo", PosterURL: "https://cdn.example/halo.jpg"}}
	tpl, err := command.NewCreateTemplateHandler(repo, anyType{}, tx, 10).
		Handle(ctx, author, command.CreateTemplateInput{TypeCode: "game", Format: "ranked", Title: "Игры"})
	if err != nil {
		t.Fatal(err)
	}
	tpl, err = command.NewSaveTemplateHandler(repo, catalog, realImagesStub{}, anyType{}, tx).Handle(ctx, tpl.ID, author,
		command.SaveTemplateInput{Revision: 1, Title: "Игры", Tiers: []command.TierInput{}, Items: []command.ItemInput{{MediaObjectID: &work}}})
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
	saved, err := results.Save(ctx, res.ID, fan, command.SaveResultInput{
		Revision: 1, Title: "Мой топ", Entries: []command.EntryInput{{ItemID: tpl.Items[0].ID, Position: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := results.Publish(ctx, res.ID, fan, saved.Revision, ""); err != nil {
		t.Fatal(err)
	}
	return tpl.ID, res.ID
}

func TestResultReactionsIntegration(t *testing.T) {
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
	exec(`INSERT INTO users (id, telegram_id, first_name, username) VALUES ($1, 1, 'Ann', 'ann'), ($2, 2, 'Fan', 'fan'), ($3, 3, 'Guest', 'guest')`,
		author, fan, guest)
	tplID, resID := publishedResult(t, ctx, pool, author, fan)

	reader := read.NewRepository(pool)
	reactions := command.NewReactionHandler(write.NewRepository(pool), reader, transaction.NewPgxManager(pool))
	comments := query.NewCommentsHandler(reader)
	get := query.NewGetResultHandler(reader)
	counters := func() (likes, dislikes, views int64) {
		t.Helper()
		v, _, err := get.Handle(ctx, resID, uuid.Nil)
		if err != nil {
			t.Fatal(err)
		}
		return v.LikesCount, v.DislikesCount, v.ViewsCount
	}

	// Votes: a repeat changes nothing, a switch moves the vote between counters.
	if err := reactions.Vote(ctx, resID, guest, true); err != nil {
		t.Fatal(err)
	}
	if err := reactions.Vote(ctx, resID, guest, true); err != nil {
		t.Fatal(err)
	}
	if l, d, _ := counters(); l != 1 || d != 0 {
		t.Fatalf("after like: likes %d dislikes %d", l, d)
	}
	if err := reactions.Vote(ctx, resID, guest, false); err != nil {
		t.Fatal(err)
	}
	if l, d, _ := counters(); l != 0 || d != 1 {
		t.Fatalf("after switch: likes %d dislikes %d", l, d)
	}
	if v, _, err := get.Handle(ctx, resID, guest); err != nil || v.ViewerIsLike == nil || *v.ViewerIsLike {
		t.Fatalf("viewer vote = %v, %v", v.ViewerIsLike, err)
	}

	// Concurrent votes of one user keep the counters consistent with the stored vote.
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			if err := reactions.Vote(ctx, resID, author, i%2 == 0); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	var stored, actualLikes, actualDislikes int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE is_like), COUNT(*) FILTER (WHERE NOT is_like)
		FROM list_result_votes WHERE result_id = $1`, resID).Scan(&stored, &actualLikes, &actualDislikes); err != nil {
		t.Fatal(err)
	}
	if l, d, _ := counters(); stored != 2 || l != actualLikes || d != actualDislikes {
		t.Fatalf("counters %d/%d, stored %d/%d of %d", l, d, actualLikes, actualDislikes, stored)
	}

	// Views: the author's own view is not counted, others count once.
	for _, u := range []uuid.UUID{fan, guest, guest, author} {
		if err := reactions.View(ctx, resID, u); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, views := counters(); views != 2 {
		t.Fatalf("views = %d", views)
	}

	// Comments and replies, paged oldest first.
	first, err := reactions.Comment(ctx, resID, guest, nil, "  Отличный топ  ")
	if err != nil {
		t.Fatal(err)
	}
	if first.Body != "Отличный топ" || first.Author.Username != "guest" || first.ParentID != nil {
		t.Fatalf("comment = %+v", first)
	}
	if _, err := reactions.Comment(ctx, resID, fan, &first.ID, "Спасибо"); err != nil {
		t.Fatal(err)
	}
	if _, err := reactions.Comment(ctx, resID, author, nil, "Второй"); err != nil {
		t.Fatal(err)
	}
	if _, err := reactions.Comment(ctx, resID, fan, new(uuid.New()), "мимо"); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("reply to a missing comment: %v", err)
	}
	if _, err := reactions.Comment(ctx, resID, fan, nil, " \n "); err == nil {
		t.Fatal("empty comment accepted")
	}
	page, err := comments.List(ctx, resID, nil, "", 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != first.ID || !page.Items[0].HasReplies || page.NextCursor == "" {
		t.Fatalf("first page = %+v %v", page, err)
	}
	page, err = comments.List(ctx, resID, nil, page.NextCursor, 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].Body != "Второй" || page.NextCursor != "" {
		t.Fatalf("second page = %+v %v", page, err)
	}
	if _, err := comments.List(ctx, resID, &first.ID, page.NextCursor, 1); err != nil {
		t.Fatal(err)
	}
	replies, err := comments.List(ctx, resID, &first.ID, "", 10)
	if err != nil || len(replies.Items) != 1 || replies.Items[0].Body != "Спасибо" {
		t.Fatalf("replies = %+v %v", replies, err)
	}

	// Hidden or unpublished results take no reactions and show no comments.
	exec(`UPDATE list_templates SET moderation_hidden_at = NOW() WHERE id = $1`, tplID)
	if err := reactions.Vote(ctx, resID, guest, true); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("vote on a hidden template's result: %v", err)
	}
	if _, err := comments.List(ctx, resID, nil, "", 10); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("comments of a hidden result: %v", err)
	}
	exec(`UPDATE list_templates SET moderation_hidden_at = NULL WHERE id = $1`, tplID)
	exec(`UPDATE list_results SET status = 'draft' WHERE id = $1`, resID)
	if err := reactions.View(ctx, resID, guest); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("view of a draft: %v", err)
	}
	if _, err := reactions.Comment(ctx, resID, guest, nil, "черновик"); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("comment on a draft: %v", err)
	}
}
