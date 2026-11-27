package listshttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
)

// maxCommentBodyBytes fits 2000 code points of 4 bytes each with JSON escaping.
const maxCommentBodyBytes = 32 << 10

type voteBody struct {
	IsLike *bool `json:"isLike"`
}

type commentJSON struct {
	ID         string     `json:"id"`
	ResultID   string     `json:"resultId"`
	ParentID   *string    `json:"parentId"`
	Author     authorJSON `json:"author"`
	Body       string     `json:"body"`
	CreatedAt  string     `json:"createdAt"`
	HasReplies bool       `json:"hasReplies"`
}

type commentPageJSON struct {
	Items      []commentJSON `json:"items"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

func commentResponse(c ports.Comment) commentJSON {
	return commentJSON{
		ID: c.ID.String(), ResultID: c.ResultID.String(), ParentID: idPtrJSON(c.ParentID), Author: authorOf(c.Author),
		Body: c.Body, CreatedAt: timeJSON(c.CreatedAt), HasReplies: c.HasReplies,
	}
}

func (h *Handler) VoteResult(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var body voteBody
	if err := bind(c, &body); err != nil {
		return err
	}
	if body.IsLike == nil {
		return mapError(contract.Field("isLike", contract.ReasonRequired))
	}
	if err := h.d.Reactions.Vote(c.Request().Context(), id, userID, *body.IsLike); err != nil {
		return mapError(err)
	}
	noStore(c)
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ViewResult(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.d.Reactions.View(c.Request().Context(), id, userID); err != nil {
		return mapError(err)
	}
	noStore(c)
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) CreateResultComment(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	// Read the raw body: JSON decoding would silently replace invalid UTF-8.
	data, err := io.ReadAll(http.MaxBytesReader(c.Response(), c.Request().Body, maxCommentBodyBytes))
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return httperr.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body is too large")
		}
		return httperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}
	if !utf8.Valid(data) {
		return mapError(contract.Field("body", contract.ReasonInvalid))
	}
	var body struct {
		Body     string     `json:"body"`
		ParentID *uuid.UUID `json:"parentId"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return httperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}
	item, err := h.d.Reactions.Comment(c.Request().Context(), id, userID, body.ParentID, body.Body)
	if err != nil {
		return mapError(err)
	}
	noStore(c)
	return c.JSON(http.StatusCreated, commentResponse(item))
}

func (h *Handler) ListResultComments(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	limit, err := parseLimit(c)
	if err != nil {
		return err
	}
	var parentID *uuid.UUID
	if raw := c.QueryParam("parentId"); raw != "" {
		p, err := uuid.Parse(raw)
		if err != nil || p == uuid.Nil {
			return mapError(contract.Field("parentId", contract.ReasonInvalid))
		}
		parentID = &p
	}
	page, err := h.d.Comments.List(c.Request().Context(), id, parentID, c.QueryParam("cursor"), limit)
	if err != nil {
		return mapError(err)
	}
	out := commentPageJSON{Items: make([]commentJSON, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, item := range page.Items {
		out.Items = append(out.Items, commentResponse(item))
	}
	noStore(c)
	return c.JSON(http.StatusOK, out)
}
