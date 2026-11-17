package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/internal/application/command"
	"review-api/internal/modules/gamification/internal/domain"
)

type PublishedHandler struct {
	grant   *command.GrantPointsHandler
	advance *command.AdvanceMetricHandler
	points  PointsConfig
}

func NewPublishedHandler(grant *command.GrantPointsHandler, advance *command.AdvanceMetricHandler, points PointsConfig) *PublishedHandler {
	return &PublishedHandler{grant: grant, advance: advance, points: points}
}

func (h *PublishedHandler) Handle(ctx context.Context, _ string, payload []byte) error {
	if h == nil || h.grant == nil || h.points.ReviewPublish < 1 {
		return nil
	}
	var body struct {
		ID        string `json:"id"`
		AuthorID  string `json:"authorId"`
		MediaType string `json:"mediaType"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return fmt.Errorf("published payload: %w", err)
	}
	authorID, err := uuid.Parse(body.AuthorID)
	if err != nil {
		return nil
	}
	reviewID, err := uuid.Parse(body.ID)
	if err != nil {
		return nil
	}
	// Fixed amount per review: blocks are gone, so "blocks_written" no longer advances.
	if err := h.grant.Handle(ctx, authorID, h.points.ReviewPublish, "review_publish", "review_publish:"+reviewID.String()); err != nil {
		return err
	}
	if h.advance == nil {
		return nil
	}
	rid := reviewID.String()
	if _, _, err := h.advance.Handle(ctx, authorID, "reviews_published", 1, "ach:pub:"+rid+":reviews"); err != nil {
		return err
	}
	mediaType := strings.ToLower(strings.TrimSpace(body.MediaType))
	typeMetric := domain.TypeMetric(mediaType)
	if typeMetric == "" {
		return nil
	}
	applied, newVal, err := h.advance.Handle(ctx, authorID, typeMetric, 1, "ach:pub:"+rid+":type")
	if err != nil {
		return err
	}
	if applied && newVal == 1 {
		if _, _, err := h.advance.Handle(
			ctx, authorID, "types_explored", 1, "ach:type_first:"+authorID.String()+":"+mediaType,
		); err != nil {
			return err
		}
	}
	return nil
}
