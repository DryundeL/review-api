package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/internal/application/command"
)

type FollowedHandler struct {
	grant   *command.GrantPointsHandler
	advance *command.AdvanceMetricHandler
	points  PointsConfig
}

func NewFollowedHandler(grant *command.GrantPointsHandler, advance *command.AdvanceMetricHandler, points PointsConfig) *FollowedHandler {
	return &FollowedHandler{grant: grant, advance: advance, points: points}
}

func (h *FollowedHandler) Handle(ctx context.Context, _ string, payload []byte) error {
	if h == nil || h.grant == nil || h.points.Follower < 1 {
		return nil
	}
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
	key := followerID.String() + ":" + followeeID.String()
	if err := h.grant.Handle(ctx, followeeID, h.points.Follower, "follower", "follower:"+key); err != nil {
		return err
	}
	if h.advance != nil {
		if _, _, err := h.advance.Handle(ctx, followeeID, "followers", 1, "ach:follow:"+key); err != nil {
			return err
		}
	}
	return nil
}
