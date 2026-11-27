package query

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
)

type GetResultHandler struct {
	repo ports.ResultReader
}

func NewGetResultHandler(repo ports.ResultReader) *GetResultHandler {
	return &GetResultHandler{repo: repo}
}

// Handle returns a result the viewer may open and whether the viewer is its author.
// viewerID is uuid.Nil for guests. Unavailable results are indistinguishable from missing ones.
func (h *GetResultHandler) Handle(ctx context.Context, id, viewerID uuid.UUID) (ports.ResultView, bool, error) {
	if h == nil || h.repo == nil {
		return ports.ResultView{}, false, contract.ErrInvalid
	}
	v, err := h.repo.Result(ctx, id)
	if err != nil {
		return ports.ResultView{}, false, err
	}
	isOwner := viewerID != uuid.Nil && v.AuthorID == viewerID
	if !CanViewResult(v.Result, v.TemplateModerated, isOwner) {
		return ports.ResultView{}, false, contract.ErrNotFound
	}
	return v, isOwner, nil
}

// CanViewResult: the author always sees their result (moderation is marked by ModerationHiddenAt
// and TemplateModerated); others see it published, unlisted included, unless the result or its
// template is hidden by a moderator. A deleted or unpublished template does not hide results.
func CanViewResult(r domain.Result, templateModerated, isOwner bool) bool {
	if isOwner {
		return true
	}
	return r.Status == domain.StatusPublished && r.ModerationHiddenAt == nil && !templateModerated
}
