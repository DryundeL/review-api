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
	"review-api/internal/platform/transaction"
)

// anyReason passes every code, so the foreign key to report_reasons is what rejects unknown ones.
type anyReason struct{}

func (anyReason) RequireActive(context.Context, string) error { return nil }

func TestModerationIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := testPool(t, ctx)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	author, fan, critic := uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO users (id, telegram_id, first_name, username) VALUES ($1, 1, 'Ann', 'ann'), ($2, 2, 'Fan', 'fan'), ($3, 3, 'Critic', 'critic')`, author, fan, critic)
	halo := uuid.New()
	exec(`INSERT INTO media_objects (id, title, type, info, poster_url) VALUES ($1, 'Halo', 'game', '{"releaseStatus":"completed"}', 'https://cdn.example/halo.jpg')`, halo)

	repo := write.NewRepository(pool)
	reader := read.NewRepository(pool)
	tx := transaction.NewPgxManager(pool)
	catalog := dbCatalog{halo: {ID: halo, Type: "game", Title: "Halo", PosterURL: "https://cdn.example/halo.jpg"}}
	createTpl := command.NewCreateTemplateHandler(repo, anyType{}, tx, 10)
	saveTpl := command.NewSaveTemplateHandler(repo, catalog, realImagesStub{}, anyType{}, tx)
	stateTpl := command.NewTemplateStateHandler(repo, catalog, tx)
	results := command.NewResultHandler(repo, tx, 10)
	reports := command.NewReportHandler(reader, reader, repo, anyReason{})
	moderation := command.NewModerationHandler(repo)
	queue := query.NewReportQueueHandler(reader)
	getTpl := query.NewGetTemplateHandler(reader)
	getRes := query.NewGetResultHandler(reader)
	lists := query.NewListHandler(reader)

	tpl, err := createTpl.Handle(ctx, author, command.CreateTemplateInput{TypeCode: "game", Format: "ranked", Title: "Игры"})
	if err != nil {
		t.Fatal(err)
	}
	if tpl, err = saveTpl.Handle(ctx, tpl.ID, author, command.SaveTemplateInput{Revision: 1, Title: "Игры", Items: []command.ItemInput{{MediaObjectID: &halo}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := reports.Create(ctx, ports.TargetTemplate, tpl.ID, fan, "spam"); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("report on a draft: %v", err)
	}
	if tpl, err = stateTpl.Publish(ctx, tpl.ID, author, tpl.Revision, "unlisted"); err != nil {
		t.Fatal(err)
	}
	res, err := results.Create(ctx, tpl.ID, fan)
	if err != nil {
		t.Fatal(err)
	}
	if res, err = results.Save(ctx, res.ID, fan, command.SaveResultInput{
		Revision: 1, Title: "Топ", Entries: []command.EntryInput{{ItemID: tpl.Items[0].ID, Position: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	if res, err = results.Publish(ctx, res.ID, fan, res.Revision, "public"); err != nil {
		t.Fatal(err)
	}

	// Reports: unlisted objects can be reported by link; one open report per reporter and object.
	if _, err := reports.Create(ctx, ports.TargetTemplate, tpl.ID, author, "spam"); !errors.Is(err, contract.ErrForbidden) {
		t.Fatalf("own template: %v", err)
	}
	if _, err := reports.Create(ctx, ports.TargetTemplate, tpl.ID, fan, "no_such_reason"); err == nil {
		t.Fatal("unknown reason stored")
	} else if fe, ok := errors.AsType[contract.FieldError](err); !ok || fe.Field != "reasonCode" {
		t.Fatalf("unknown reason: %v", err)
	}
	if _, err := reports.Create(ctx, ports.TargetTemplate, uuid.New(), fan, "spam"); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("missing template: %v", err)
	}
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		ok, dups int
	)
	for range 4 {
		wg.Go(func() {
			_, err := reports.Create(ctx, ports.TargetTemplate, tpl.ID, fan, "spam")
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, contract.ErrAlreadyReported):
				dups++
			default:
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if ok != 1 || dups != 3 {
		t.Fatalf("concurrent reports: ok %d, duplicates %d", ok, dups)
	}
	if _, err := reports.Create(ctx, ports.TargetTemplate, tpl.ID, critic, "abuse"); err != nil {
		t.Fatal(err)
	}
	if _, err := reports.Create(ctx, ports.TargetResult, res.ID, critic, "abuse"); err != nil {
		t.Fatal(err)
	}

	// The queue is oldest first and pages by cursor; reporting never hides anything by itself.
	page, err := queue.Handle(ctx, "open", "", 2)
	if err != nil || len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("queue page: %+v %v", page, err)
	}
	first := page.Items[0]
	if first.Target != ports.TargetTemplate || first.TargetID != tpl.ID || first.ReporterID != fan || first.TargetTitle != "Игры" || first.TargetAuthorID != author || first.TargetHidden {
		t.Fatalf("first report = %+v", first)
	}
	rest, err := queue.Handle(ctx, "", page.NextCursor, 2)
	if err != nil || len(rest.Items) != 1 || rest.Items[0].Target != ports.TargetResult || rest.NextCursor != "" {
		t.Fatalf("queue rest: %+v %v", rest, err)
	}
	if _, _, err := getTpl.Handle(ctx, tpl.ID, uuid.Nil); err != nil {
		t.Fatalf("reports must not hide: %v", err)
	}

	// Hiding the template hides it and every result on it; status and visibility are kept.
	if err := moderation.Hide(ctx, ports.TargetTemplate, tpl.ID); err != nil {
		t.Fatal(err)
	}
	var hiddenAt time.Time
	if err := pool.QueryRow(ctx, `SELECT moderation_hidden_at FROM list_templates WHERE id = $1`, tpl.ID).Scan(&hiddenAt); err != nil {
		t.Fatal(err)
	}
	if err := moderation.Hide(ctx, ports.TargetTemplate, tpl.ID); err != nil {
		t.Fatal(err)
	}
	var again time.Time
	if err := pool.QueryRow(ctx, `SELECT moderation_hidden_at FROM list_templates WHERE id = $1`, tpl.ID).Scan(&again); err != nil || !again.Equal(hiddenAt) {
		t.Fatalf("second hide must keep the first time: %v %v", again, err)
	}
	if _, _, err := getTpl.Handle(ctx, tpl.ID, uuid.Nil); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("hidden template visible: %v", err)
	}
	if _, _, err := getRes.Handle(ctx, res.ID, critic); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("result of a hidden template visible: %v", err)
	}
	if p, err := lists.Results(ctx, query.ListInput{}); err != nil || len(p.Items) != 0 {
		t.Fatalf("section shows a result of a hidden template: %+v %v", p.Items, err)
	}
	if _, err := reports.Create(ctx, ports.TargetResult, res.ID, author, "spam"); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("report on a hidden result: %v", err)
	}
	if err := moderation.Unhide(ctx, ports.TargetTemplate, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if v, _, err := getTpl.Handle(ctx, tpl.ID, uuid.Nil); err != nil || v.Status != domain.StatusPublished || v.Visibility != domain.VisibilityUnlisted {
		t.Fatalf("unhide must restore the template: %+v %v", v, err)
	}
	if p, err := lists.Results(ctx, query.ListInput{}); err != nil || len(p.Items) != 1 {
		t.Fatalf("unhide must restore the result in the section: %v", err)
	}

	// A result can be hidden on its own; a deleted template can still be hidden, hiding its results.
	if err := moderation.Hide(ctx, ports.TargetResult, res.ID); err != nil {
		t.Fatal(err)
	}
	if v, _, err := getRes.Handle(ctx, res.ID, fan); err != nil || v.ModerationHiddenAt == nil {
		t.Fatalf("author must see the moderation mark: %v", err)
	}
	if err := moderation.Unhide(ctx, ports.TargetResult, res.ID); err != nil {
		t.Fatal(err)
	}
	if err := stateTpl.Delete(ctx, tpl.ID, author); err != nil {
		t.Fatal(err)
	}
	if err := moderation.Hide(ctx, ports.TargetTemplate, tpl.ID); err != nil {
		t.Fatalf("hide a deleted template: %v", err)
	}
	if _, _, err := getRes.Handle(ctx, res.ID, critic); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("result of a deleted hidden template visible: %v", err)
	}
	if err := moderation.Hide(ctx, ports.TargetResult, uuid.New()); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("hide a missing result: %v", err)
	}
	if err := moderation.Unhide(ctx, ports.TargetTemplate, tpl.ID); err != nil {
		t.Fatal(err)
	}

	// Resolving closes the report once; afterwards the same reporter may report again.
	resolved, err := moderation.Resolve(ctx, first.ID)
	if err != nil || resolved.Status != ports.ReportResolved || resolved.ResolvedAt == nil || resolved.TargetID != tpl.ID {
		t.Fatalf("resolve: %+v %v", resolved, err)
	}
	if _, err := moderation.Resolve(ctx, first.ID); !errors.Is(err, contract.ErrConflict) {
		t.Fatalf("second resolve: %v", err)
	}
	if _, err := moderation.Resolve(ctx, uuid.New()); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("resolve a missing report: %v", err)
	}
	if _, err := reports.Create(ctx, ports.TargetResult, res.ID, fan, "spam"); !errors.Is(err, contract.ErrForbidden) {
		t.Fatalf("own result: %v", err)
	}
	if _, err := reports.Create(ctx, ports.TargetResult, res.ID, author, "spam"); err != nil {
		t.Fatalf("report a result of a deleted template: %v", err)
	}
	if _, err := reports.Create(ctx, ports.TargetResult, res.ID, critic, "spam"); !errors.Is(err, contract.ErrAlreadyReported) {
		t.Fatalf("open report repeated: %v", err)
	}
	if _, err := moderation.Resolve(ctx, rest.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reports.Create(ctx, ports.TargetResult, res.ID, critic, "spam"); err != nil {
		t.Fatalf("report again after resolve: %v", err)
	}
	if p, err := queue.Handle(ctx, "open", "", 0); err != nil || len(p.Items) != 3 {
		t.Fatalf("queue after resolve: %+v %v", p.Items, err)
	}
	if _, err := queue.Handle(ctx, "resolved", "", 0); err == nil {
		t.Fatal("resolved queue is not supported")
	}
}
