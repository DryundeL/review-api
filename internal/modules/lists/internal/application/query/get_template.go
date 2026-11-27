package query

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
)

type GetTemplateHandler struct {
	repo ports.TemplateReader
}

func NewGetTemplateHandler(repo ports.TemplateReader) *GetTemplateHandler {
	return &GetTemplateHandler{repo: repo}
}

// Handle returns a template the viewer may open and whether the viewer is its author.
// viewerID is uuid.Nil for guests. Unavailable templates are indistinguishable from missing ones.
func (h *GetTemplateHandler) Handle(ctx context.Context, id, viewerID uuid.UUID) (ports.TemplateView, bool, error) {
	if h == nil || h.repo == nil {
		return ports.TemplateView{}, false, contract.ErrInvalid
	}
	v, err := h.repo.Template(ctx, id)
	if err != nil {
		return ports.TemplateView{}, false, err
	}
	isOwner := viewerID != uuid.Nil && v.AuthorID == viewerID
	if !CanViewTemplate(v.Template, isOwner) {
		return ports.TemplateView{}, false, contract.ErrNotFound
	}
	return v, isOwner, nil
}

// CanViewTemplate: the author always sees their template (a moderated one is marked by
// ModerationHiddenAt); others see it published, by direct link even when unlisted, unless hidden.
func CanViewTemplate(t domain.Template, isOwner bool) bool {
	if isOwner {
		return true
	}
	return t.Status == domain.StatusPublished && t.ModerationHiddenAt == nil
}
