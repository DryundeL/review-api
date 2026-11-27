package listshttp

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/lists/internal/application/command"
)

type entryBody struct {
	ItemID   uuid.UUID  `json:"itemId"`
	TierID   *uuid.UUID `json:"tierId"`
	Position int        `json:"position"`
}

type saveResultBody struct {
	Revision    int         `json:"revision"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Entries     []entryBody `json:"entries"`
}

func (b saveResultBody) input() command.SaveResultInput {
	in := command.SaveResultInput{
		Revision: b.Revision, Title: b.Title, Description: b.Description,
		Entries: make([]command.EntryInput, 0, len(b.Entries)),
	}
	for _, e := range b.Entries {
		in.Entries = append(in.Entries, command.EntryInput{ItemID: e.ItemID, TierID: e.TierID, Position: e.Position})
	}
	return in
}

func (h *Handler) GetResult(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	viewerID, err := h.optionalUser(c)
	if err != nil {
		return err
	}
	v, isOwner, err := h.d.GetResult.Handle(c.Request().Context(), id, viewerID)
	if err != nil {
		return mapError(err)
	}
	private(c, isOwner)
	return c.JSON(http.StatusOK, resultResponse(v, isOwner))
}

func (h *Handler) ownResult(c echo.Context, status int, id, userID uuid.UUID) error {
	v, isOwner, err := h.d.GetResult.Handle(c.Request().Context(), id, userID)
	if err != nil {
		return mapError(err)
	}
	noStore(c)
	return c.JSON(status, resultResponse(v, isOwner))
}

// CreateResult starts a draft result on the snapshot of the template's current revision.
func (h *Handler) CreateResult(c echo.Context) error {
	userID, templateID, err := h.userAndID(c)
	if err != nil {
		return err
	}
	res, err := h.d.Results.Create(c.Request().Context(), templateID, userID)
	if err != nil {
		return mapError(err)
	}
	return h.ownResult(c, http.StatusCreated, res.ID, userID)
}

func (h *Handler) SaveResult(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	var body saveResultBody
	if err := bind(c, &body); err != nil {
		return err
	}
	if err := requireRevision(body.Revision); err != nil {
		return err
	}
	if _, err := h.d.Results.Save(c.Request().Context(), id, userID, body.input()); err != nil {
		return mapError(err)
	}
	return h.ownResult(c, http.StatusOK, id, userID)
}

func (h *Handler) PublishResult(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	var body publishBody
	if err := bind(c, &body); err != nil {
		return err
	}
	if err := requireRevision(body.Revision); err != nil {
		return err
	}
	if _, err := h.d.Results.Publish(c.Request().Context(), id, userID, body.Revision, body.Visibility); err != nil {
		return mapError(err)
	}
	return h.ownResult(c, http.StatusOK, id, userID)
}

func (h *Handler) UnpublishResult(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	if _, err := h.d.Results.Unpublish(c.Request().Context(), id, userID); err != nil {
		return mapError(err)
	}
	return h.ownResult(c, http.StatusOK, id, userID)
}

func (h *Handler) ResultVisibility(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	var body visibilityBody
	if err := bind(c, &body); err != nil {
		return err
	}
	if _, err := h.d.Results.SetVisibility(c.Request().Context(), id, userID, body.Visibility); err != nil {
		return mapError(err)
	}
	return h.ownResult(c, http.StatusOK, id, userID)
}

func (h *Handler) CopyResult(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	res, err := h.d.Results.Copy(c.Request().Context(), id, userID)
	if err != nil {
		return mapError(err)
	}
	return h.ownResult(c, http.StatusCreated, res.ID, userID)
}

func (h *Handler) DeleteResult(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	if err := h.d.Results.Delete(c.Request().Context(), id, userID); err != nil {
		return mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
