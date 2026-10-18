package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/notifications/internal/application/command"
	reviewscontract "review-api/internal/modules/reviews/contract"
)

type ReviewLookup interface {
	PublishedReviewRef(ctx context.Context, reviewID uuid.UUID) (reviewscontract.ReviewRef, error)
}

type VotedHandler struct {
	create  *command.CreateNotificationHandler
	reviews ReviewLookup
}

func NewVotedHandler(create *command.CreateNotificationHandler, reviews ReviewLookup) *VotedHandler {
	return &VotedHandler{create: create, reviews: reviews}
}

func (h *VotedHandler) Handle(ctx context.Context, _ string, payload []byte) error {
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
	rid := ref.ID
	return h.create.Handle(ctx, command.CreateInput{
		RecipientID: ref.AuthorID,
		ActorID:     actorID,
		TypeName:    "like",
		ReviewID:    &rid,
		ReviewTitle: ref.Title,
	})
}
