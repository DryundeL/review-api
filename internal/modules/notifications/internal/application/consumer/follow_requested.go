package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"review-api/internal/modules/notifications/internal/application/command"
)

type FollowRequestedHandler struct {
	create *command.CreateNotificationHandler
}

func NewFollowRequestedHandler(create *command.CreateNotificationHandler) *FollowRequestedHandler {
	return &FollowRequestedHandler{create: create}
}
func (h *FollowRequestedHandler) Handle(ctx context.Context, _ string, payload []byte) error {
	var body struct {
		FollowerID     string `json:"followerId"`
		FolloweeID     string `json:"followeeId"`
		NotificationID string `json:"notificationId"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return fmt.Errorf("follow requested payload: %w", err)
	}
	actor, err := uuid.Parse(body.FollowerID)
	if err != nil {
		return nil
	}
	recipient, err := uuid.Parse(body.FolloweeID)
	if err != nil {
		return nil
	}
	id, err := uuid.Parse(body.NotificationID)
	if err != nil || id == uuid.Nil {
		return nil
	}
	return h.create.Handle(ctx, command.CreateInput{NotificationID: id, ActorID: actor, RecipientID: recipient, TypeName: "follow_request"})
}
