package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/internal/application/command"
	reviewscontract "review-api/internal/modules/reviews/contract"
)

type ReviewLookup interface {
	PublishedReviewRef(ctx context.Context, reviewID uuid.UUID) (reviewscontract.ReviewRef, error)
}

type VotedHandler struct {
	grant   *command.GrantPointsHandler
	reviews ReviewLookup
	points  PointsConfig
}

func NewVotedHandler(grant *command.GrantPointsHandler, reviews ReviewLookup, points PointsConfig) *VotedHandler {
	return &VotedHandler{grant: grant, reviews: reviews, points: points}
}

func (h *VotedHandler) Handle(ctx context.Context, _ string, payload []byte) error {
	if h == nil || h.grant == nil || h.reviews == nil {
		return nil
	}
	var body struct {
		ReviewID string `json:"reviewId"`
		UserID   string `json:"userId"`
		IsLike   string `json:"isLike"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return fmt.Errorf("voted payload: %w", err)
	}
	if body.IsLike != "true" {
		return nil
	}
	reviewID, err := uuid.Parse(body.ReviewID)
	if err != nil {
		return nil
	}
	actorID, err := uuid.Parse(body.UserID)
	if err != nil {
		return nil
	}
	ref, err := h.reviews.PublishedReviewRef(ctx, reviewID)
	if err != nil {
		return fmt.Errorf("voted review: %w", err)
	}
	if actorID == ref.AuthorID {
		return nil
	}
	keyBase := reviewID.String() + ":" + actorID.String()
	if h.points.LikeGiven > 0 {
		if err := h.grant.Handle(ctx, actorID, h.points.LikeGiven, "like_given", "like_given:"+keyBase); err != nil {
			return err
		}
	}
	if h.points.LikeReceived > 0 {
		if err := h.grant.Handle(ctx, ref.AuthorID, h.points.LikeReceived, "like_received", "like_received:"+keyBase); err != nil {
			return err
		}
	}
	return nil
}
