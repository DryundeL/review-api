package cataloghttp

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/catalog/internal/application/command"
	"review-api/internal/modules/catalog/internal/application/ports"
	"review-api/internal/modules/catalog/internal/application/query"
	"review-api/internal/modules/catalog/internal/domain"
)

type Handler struct {
	create *command.CreateWorkHandler
	get    *query.GetWorkHandler
	search *query.SearchWorksHandler
}

func NewHandler(create *command.CreateWorkHandler, get *query.GetWorkHandler, search *query.SearchWorksHandler) *Handler {
	return &Handler{create: create, get: get, search: search}
}

func (h *Handler) Search(c echo.Context) error {
	limit := 20
	if raw := c.QueryParam("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return invalid("limit")
		}
		limit = parsed
	}
	items, err := h.search.Handle(c.Request().Context(), c.QueryParam("q"), c.QueryParam("type"), limit)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, listResponse{Items: worksResponse(items)})
}

func (h *Handler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "work not found")
	}
	work, err := h.get.Handle(c.Request().Context(), id)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, workResponse(work))
}

func (h *Handler) Create(c echo.Context) error {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil {
		return httperr.Unauthorized("unauthorized")
	}
	var body createBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	work, err := h.create.Handle(c.Request().Context(), id.TelegramID, domain.Draft{
		Title:         body.Title,
		PrimaryType:   body.PrimaryType,
		Types:         body.Types,
		ReleaseYear:   body.ReleaseYear,
		CoverURL:      body.CoverURL,
		ReleaseStatus: body.ReleaseStatus,
	})
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusCreated, workResponse(work))
}

type createBody struct {
	Title         string   `json:"title"`
	PrimaryType   string   `json:"primaryType"`
	Types         []string `json:"types"`
	ReleaseYear   *int     `json:"releaseYear"`
	CoverURL      string   `json:"coverUrl"`
	ReleaseStatus string   `json:"releaseStatus"`
}

type workJSON struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	PrimaryType   string   `json:"primaryType"`
	Types         []string `json:"types"`
	ReleaseYear   *int     `json:"releaseYear,omitempty"`
	CoverURL      string   `json:"coverUrl,omitempty"`
	ReleaseStatus string   `json:"releaseStatus"`
	Origin        string   `json:"origin"`
	CreatedBy     string   `json:"createdBy"`
}

type listResponse struct {
	Items []workJSON `json:"items"`
}

func worksResponse(items []ports.Work) []workJSON {
	out := make([]workJSON, 0, len(items))
	for _, item := range items {
		out = append(out, workResponse(item))
	}
	return out
}

func workResponse(item ports.Work) workJSON {
	types := item.Types
	if types == nil {
		types = []string{}
	}
	return workJSON{
		ID:            item.ID.String(),
		Title:         item.Title,
		PrimaryType:   item.PrimaryType,
		Types:         types,
		ReleaseYear:   item.ReleaseYear,
		CoverURL:      item.CoverURL,
		ReleaseStatus: item.ReleaseStatus,
		Origin:        item.Origin,
		CreatedBy:     item.CreatedBy.String(),
	}
}

func invalid(field string) error {
	return &httperr.Error{
		Status:  http.StatusUnprocessableEntity,
		Code:    "VALIDATION_ERROR",
		Message: "invalid work",
		Details: map[string]string{"field": field},
	}
}

func mapError(err error) error {
	var field contract.FieldError
	switch {
	case errors.As(err, &field):
		return invalid(field.Field)
	case errors.Is(err, contract.ErrNotFound):
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "work not found")
	case errors.Is(err, contract.ErrAuthorNotFound):
		return httperr.Unauthorized("unauthorized")
	case errors.Is(err, contract.ErrInvalid):
		return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid work")
	default:
		return err
	}
}
