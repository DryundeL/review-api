package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/internal/application/command"
)

type PointsConfig struct {
	ReviewPerBlock int
	LikeGiven      int
	LikeReceived   int
	Follower       int
}

type PublishedHandler struct {
	grant  *command.GrantPointsHandler
	points PointsConfig
}

func NewPublishedHandler(grant *command.GrantPointsHandler, points PointsConfig) *PublishedHandler {
	return &PublishedHandler{grant: grant, points: points}
}

func (h *PublishedHandler) Handle(ctx context.Context, _ string, payload []byte) error {
	if h == nil || h.grant == nil || h.points.ReviewPerBlock < 1 {
		return nil
	}
	var body struct {
		ID         string `json:"id"`
		AuthorID   string `json:"authorId"`
		BlockCount string `json:"blockCount"`
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
	blocks, err := strconv.Atoi(body.BlockCount)
	if err != nil || blocks < 1 {
		return nil
	}
	amount := h.points.ReviewPerBlock * blocks
	return h.grant.Handle(ctx, authorID, amount, "review_publish", "review_publish:"+reviewID.String())
}
