package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/notifications/internal/application/command"
)

type FollowedHandler struct {
	create *command.CreateNotificationHandler
}

func NewFollowedHandler(create *command.CreateNotificationHandler) *FollowedHandler {
	return &FollowedHandler{create: create}
}

func (h *FollowedHandler) Handle(ctx context.Context, _ string, payload []byte) error {
	var body struct {
		FollowerID string `json:"followerId"`
		FolloweeID string `json:"followeeId"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return fmt.Errorf("followed payload: %w", err)
	}
	followerID, err := uuid.Parse(body.FollowerID)
	if err != nil {
		return nil
	}
	followeeID, err := uuid.Parse(body.FolloweeID)
	if err != nil {
		return nil
	}
	return h.create.Handle(ctx, command.CreateInput{
		RecipientID: followeeID,
		ActorID:     followerID,
		TypeName:    "follow",
	})
}
