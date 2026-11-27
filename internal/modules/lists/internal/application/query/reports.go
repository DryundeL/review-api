package query

import (
	"context"
	"strings"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
)

type ReportPage struct {
	Items      []ports.ReportCard
	NextCursor string
}

type ReportQueueHandler struct {
	repo ports.ReportReader
}

func NewReportQueueHandler(repo ports.ReportReader) *ReportQueueHandler {
	return &ReportQueueHandler{repo: repo}
}

const reportQueueKind = "reports:open"

// Handle lists open reports for the moderator. Only the open queue exists: status is "" or "open".
func (h *ReportQueueHandler) Handle(ctx context.Context, status, cursor string, limit int) (ReportPage, error) {
	if h == nil || h.repo == nil {
		return ReportPage{}, contract.ErrInvalid
	}
	if s := strings.TrimSpace(status); s != "" && s != string(ports.ReportOpen) {
		return ReportPage{}, contract.Field("status", contract.ReasonInvalid)
	}
	switch {
	case limit == 0:
		limit = DefaultPageSize
	case limit < 1 || limit > MaxPageSize:
		return ReportPage{}, contract.Field("limit", contract.ReasonInvalid)
	}
	after, err := decodeCursor(cursor, reportQueueKind)
	if err != nil {
		return ReportPage{}, err
	}
	cards, err := h.repo.OpenReports(ctx, after, limit+1)
	if err != nil {
		return ReportPage{}, err
	}
	page := ReportPage{Items: cards}
	if len(cards) > limit {
		page.Items = cards[:limit]
		last := page.Items[limit-1]
		if page.NextCursor, err = encodeCursor(reportQueueKind, ports.PageCursor{T: last.CreatedAt, ID: last.ID}); err != nil {
			return ReportPage{}, err
		}
	}
	return page, nil
}
