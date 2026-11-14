package command

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
	"review-api/internal/platform/transaction"
)

type CreateCommentHandler struct {
	writer    ports.CommentWriter
	reader    ports.CommentReader
	tx        transaction.Manager
	publisher ports.Publisher
}

func NewCreateCommentHandler(writer ports.CommentWriter, reader ports.CommentReader, tx transaction.Manager, publisher ports.Publisher) *CreateCommentHandler {
	return &CreateCommentHandler{writer: writer, reader: reader, tx: tx, publisher: publisher}
}

func (h *CreateCommentHandler) Handle(ctx context.Context, reviewID, authorID uuid.UUID, parentID *uuid.UUID, body string) (ports.Comment, error) {
	item, err := domain.NewComment(reviewID, authorID, parentID, body)
	if err != nil {
		return ports.Comment{}, err
	}
	var result ports.Comment
	err = h.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := h.writer.CreateComment(ctx, item); err != nil {
			return err
		}
		if err := h.publisher.Publish(ctx, contract.EventCommentCreated, reviewID.String(), contract.CommentCreated{
			CommentID: item.ID, ReviewID: reviewID, AuthorID: authorID, ParentID: item.ParentID,
		}); err != nil {
			return err
		}
		var err error
		result, err = h.reader.GetComment(ctx, reviewID, item.ID)
		return err
	})
	return result, err
}
