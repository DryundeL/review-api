package listshttp

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/lists/internal/application/command"
)

type createTemplateBody struct {
	TypeCode string `json:"typeCode"`
	Format   string `json:"format"`
	Title    string `json:"title"`
}

type tierBody struct {
	ID    *uuid.UUID `json:"id"`
	Name  string     `json:"name"`
	Color string     `json:"color"`
}

type itemBody struct {
	ID            *uuid.UUID `json:"id"`
	MediaObjectID *uuid.UUID `json:"mediaObjectId"`
	ImageID       *uuid.UUID `json:"imageId"`
	Title         string     `json:"title"`
}

type saveTemplateBody struct {
	Revision    int        `json:"revision"`
	TypeCode    *string    `json:"typeCode"`
	Format      *string    `json:"format"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Tiers       []tierBody `json:"tiers"`
	Items       []itemBody `json:"items"`
}

type publishBody struct {
	Revision   int    `json:"revision"`
	Visibility string `json:"visibility"`
}

type visibilityBody struct {
	Visibility string `json:"visibility"`
}

type revisionBody struct {
	Revision int `json:"revision"`
}

func (b saveTemplateBody) input() command.SaveTemplateInput {
	in := command.SaveTemplateInput{
		Revision: b.Revision, TypeCode: b.TypeCode, Format: b.Format, Title: b.Title, Description: b.Description,
		Tiers: make([]command.TierInput, 0, len(b.Tiers)), Items: make([]command.ItemInput, 0, len(b.Items)),
	}
	for _, t := range b.Tiers {
		in.Tiers = append(in.Tiers, command.TierInput{ID: t.ID, Name: t.Name, Color: t.Color})
	}
	for _, it := range b.Items {
		in.Items = append(in.Items, command.ItemInput{ID: it.ID, MediaObjectID: it.MediaObjectID, ImageID: it.ImageID, Title: it.Title})
	}
	return in
}

func (h *Handler) GetTemplate(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	viewerID, err := h.optionalUser(c)
	if err != nil {
		return err
	}
	v, isOwner, err := h.d.GetTemplate.Handle(c.Request().Context(), id, viewerID)
	if err != nil {
		return mapError(err)
	}
	private(c, isOwner)
	return c.JSON(http.StatusOK, templateResponse(v, isOwner))
}

// ownTemplate answers a command with the author's view of the template.
func (h *Handler) ownTemplate(c echo.Context, status int, id, userID uuid.UUID) error {
	v, isOwner, err := h.d.GetTemplate.Handle(c.Request().Context(), id, userID)
	if err != nil {
		return mapError(err)
	}
	noStore(c)
	return c.JSON(status, templateResponse(v, isOwner))
}

func (h *Handler) CreateTemplate(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	var body createTemplateBody
	if err := bind(c, &body); err != nil {
		return err
	}
	tpl, err := h.d.CreateTemplate.Handle(c.Request().Context(), userID, command.CreateTemplateInput{
		TypeCode: body.TypeCode, Format: body.Format, Title: body.Title,
	})
	if err != nil {
		return mapError(err)
	}
	return h.ownTemplate(c, http.StatusCreated, tpl.ID, userID)
}

func (h *Handler) SaveTemplate(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	var body saveTemplateBody
	if err := bind(c, &body); err != nil {
		return err
	}
	if err := requireRevision(body.Revision); err != nil {
		return err
	}
	if _, err := h.d.SaveTemplate.Handle(c.Request().Context(), id, userID, body.input()); err != nil {
		return mapError(err)
	}
	return h.ownTemplate(c, http.StatusOK, id, userID)
}

func (h *Handler) PublishTemplate(c echo.Context) error {
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
	if _, err := h.d.TemplateState.Publish(c.Request().Context(), id, userID, body.Revision, body.Visibility); err != nil {
		return mapError(err)
	}
	return h.ownTemplate(c, http.StatusOK, id, userID)
}

func (h *Handler) UnpublishTemplate(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	if _, err := h.d.TemplateState.Unpublish(c.Request().Context(), id, userID); err != nil {
		return mapError(err)
	}
	return h.ownTemplate(c, http.StatusOK, id, userID)
}

func (h *Handler) TemplateVisibility(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	var body visibilityBody
	if err := bind(c, &body); err != nil {
		return err
	}
	if _, err := h.d.TemplateState.SetVisibility(c.Request().Context(), id, userID, body.Visibility); err != nil {
		return mapError(err)
	}
	return h.ownTemplate(c, http.StatusOK, id, userID)
}

func (h *Handler) RefreshItem(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	itemID, err := pathID(c, "itemId")
	if err != nil {
		return err
	}
	var body revisionBody
	if err := bind(c, &body); err != nil {
		return err
	}
	if err := requireRevision(body.Revision); err != nil {
		return err
	}
	if _, err := h.d.TemplateState.RefreshItem(c.Request().Context(), id, itemID, userID, body.Revision); err != nil {
		return mapError(err)
	}
	return h.ownTemplate(c, http.StatusOK, id, userID)
}

func (h *Handler) CopyTemplate(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	tpl, err := h.d.CopyTemplate.Handle(c.Request().Context(), id, userID)
	if err != nil {
		return mapError(err)
	}
	return h.ownTemplate(c, http.StatusCreated, tpl.ID, userID)
}

func (h *Handler) DeleteTemplate(c echo.Context) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	if err := h.d.TemplateState.Delete(c.Request().Context(), id, userID); err != nil {
		return mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// userAndID authenticates first, so guests get 401 before any lookup by id.
func (h *Handler) userAndID(c echo.Context) (uuid.UUID, uuid.UUID, error) {
	userID, err := h.requireUser(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	id, err := pathID(c, "id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return userID, id, nil
}
