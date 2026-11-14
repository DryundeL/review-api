package contract

import "github.com/google/uuid"

const EventCommentCreated = "review.comment_created"

type CommentCreated struct {
	CommentID uuid.UUID  `json:"commentId"`
	ReviewID  uuid.UUID  `json:"reviewId"`
	AuthorID  uuid.UUID  `json:"authorId"`
	ParentID  *uuid.UUID `json:"parentId"`
}

// CommentNotificationRef contains only published discussion metadata.
type CommentNotificationRef struct {
	ReviewID    uuid.UUID
	ReviewTitle string
	AuthorID    uuid.UUID
	ParentID    *uuid.UUID
	RecipientID uuid.UUID
}
