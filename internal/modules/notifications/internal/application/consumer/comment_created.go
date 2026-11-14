package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"review-api/internal/modules/notifications/internal/application/command"
	reviewscontract "review-api/internal/modules/reviews/contract"
)

type CommentLookup interface {
	CommentNotificationRef(context.Context, uuid.UUID) (reviewscontract.CommentNotificationRef, error)
}

type CommentCreatedHandler struct {
	create   *command.CreateNotificationHandler
	comments CommentLookup
}

func NewCommentCreatedHandler(create *command.CreateNotificationHandler, comments CommentLookup) *CommentCreatedHandler {
	return &CommentCreatedHandler{create: create, comments: comments}
}

func (h *CommentCreatedHandler) Handle(ctx context.Context, _ string, payload []byte) error {
	var event reviewscontract.CommentCreated
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil
	}
	if event.CommentID == uuid.Nil || event.ReviewID == uuid.Nil || event.AuthorID == uuid.Nil {
		return nil
	}
	ref, err := h.comments.CommentNotificationRef(ctx, event.CommentID)
	if errors.Is(err, reviewscontract.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if ref.ReviewID != event.ReviewID || ref.AuthorID != event.AuthorID {
		return nil
	}
	typeName := "comment"
	if ref.ParentID != nil {
		typeName = "comment_reply"
	}
	id := uuid.NewHash(sha256.New(), uuid.NameSpaceOID, []byte(event.CommentID.String()+":"+ref.RecipientID.String()+":"+typeName), 8)
	return h.create.Handle(ctx, command.CreateInput{
		NotificationID: id, RecipientID: ref.RecipientID, ActorID: ref.AuthorID, TypeName: typeName,
		ReviewID: &ref.ReviewID, ReviewTitle: ref.ReviewTitle, CommentID: &event.CommentID, ParentCommentID: ref.ParentID,
	})
}
