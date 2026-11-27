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

type stubTemplates map[uuid.UUID]ports.TemplateView

func (s stubTemplates) Template(_ context.Context, id uuid.UUID) (ports.TemplateView, error) {
	if v, ok := s[id]; ok {
		return v, nil
	}
	return ports.TemplateView{}, contract.ErrNotFound
}

type stubResults map[uuid.UUID]ports.ResultView

func (s stubResults) Result(_ context.Context, id uuid.UUID) (ports.ResultView, error) {
	if v, ok := s[id]; ok {
		return v, nil
	}
	return ports.ResultView{}, contract.ErrNotFound
}

type memReports struct{ inserted []ports.Report }

func (m *memReports) InsertReport(_ context.Context, r ports.Report) error {
	m.inserted = append(m.inserted, r)
	return nil
}
func (m *memReports) ResolveReport(context.Context, uuid.UUID) (ports.Report, error) {
	return ports.Report{}, contract.ErrNotFound
}
func (m *memReports) SetHidden(context.Context, ports.ReportTarget, uuid.UUID, bool) error {
	return nil
}

type fakeReasons map[string]bool

func (f fakeReasons) RequireActive(_ context.Context, code string) error {
	if !f[code] {
		return contract.Field("reasonCode", contract.ReasonInvalid)
	}
	return nil
}

func TestCreateReport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	author, reporter := uuid.New(), uuid.New()
	published := uuid.New()
	tpl := ports.TemplateView{Template: domain.Template{ID: published, AuthorID: author, Status: domain.StatusPublished}}
	resOnHidden := uuid.New()
	res := ports.ResultView{Result: domain.Result{ID: resOnHidden, AuthorID: author, Status: domain.StatusPublished}, TemplateModerated: true}
	repo := &memReports{}
	h := NewReportHandler(stubTemplates{published: tpl}, stubResults{resOnHidden: res}, repo, fakeReasons{"spam": true})

	_, err := h.Create(ctx, ports.TargetTemplate, published, reporter, "  ")
	wantField(t, err, "reasonCode", contract.ReasonRequired)
	_, err = h.Create(ctx, ports.TargetTemplate, published, reporter, "nope")
	wantField(t, err, "reasonCode", contract.ReasonInvalid)
	if _, err := h.Create(ctx, "review", published, reporter, "spam"); !errors.Is(err, contract.ErrInvalid) {
		t.Fatalf("unknown target: %v", err)
	}
	if _, err := h.Create(ctx, ports.TargetResult, resOnHidden, reporter, "spam"); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("result of a hidden template: %v", err)
	}
	rep, err := h.Create(ctx, ports.TargetTemplate, published, reporter, " spam ")
	if err != nil || rep.ReasonCode != "spam" || rep.Status != ports.ReportOpen || len(repo.inserted) != 1 {
		t.Fatalf("report = %+v %v", rep, err)
	}
	if err := NewModerationHandler(repo).Hide(ctx, "review", published); !errors.Is(err, contract.ErrInvalid) {
		t.Fatalf("hide unknown target: %v", err)
	}
}
