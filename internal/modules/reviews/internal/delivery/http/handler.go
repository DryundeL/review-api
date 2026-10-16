package reviewshttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/command"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/application/query"
)

// StatusItem is one fixed status from the module config.
type StatusItem struct {
	Code  string
	Label string
}

// StatusCatalog is the read-only status config served to the client.
type StatusCatalog struct {
	Publication []StatusItem
	Release     []StatusItem
}

// UserResolver maps Telegram identity to internal user id.
type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Handler struct {
	list         *query.ListTypesHandler
	create       *command.CreateTypeHandler
	update       *command.UpdateTypeHandler
	statuses     StatusCatalog
	createReview *command.CreateReviewHandler
	updateReview *command.UpdateReviewHandler
	getReview    *query.GetReviewHandler
	listReviews  *query.ListReviewsHandler
	voteReview   *command.VoteReviewHandler
	viewReview   *command.ViewReviewHandler
	users        UserResolver
}

type HandlerDeps struct {
	ListTypes    *query.ListTypesHandler
	CreateType   *command.CreateTypeHandler
	UpdateType   *command.UpdateTypeHandler
	Statuses     StatusCatalog
	CreateReview *command.CreateReviewHandler
	UpdateReview *command.UpdateReviewHandler
	GetReview    *query.GetReviewHandler
	ListReviews  *query.ListReviewsHandler
	VoteReview   *command.VoteReviewHandler
	ViewReview   *command.ViewReviewHandler
	Users        UserResolver
}

func NewHandler(deps HandlerDeps) *Handler {
	return &Handler{
		list:         deps.ListTypes,
		create:       deps.CreateType,
		update:       deps.UpdateType,
		statuses:     deps.Statuses,
		createReview: deps.CreateReview,
		updateReview: deps.UpdateReview,
		getReview:    deps.GetReview,
		listReviews:  deps.ListReviews,
		voteReview:   deps.VoteReview,
		viewReview:   deps.ViewReview,
		users:        deps.Users,
	}
}

func (h *Handler) List(c echo.Context) error {
	items, err := h.list.Handle(c.Request().Context(), true)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=60")
	return c.JSON(http.StatusOK, listResponse{Items: publicTypes(items)})
}

func (h *Handler) Statuses(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	return c.JSON(http.StatusOK, statusesResponse{
		Publication: statusItems(h.statuses.Publication),
		Release:     statusItems(h.statuses.Release),
	})
}

func (h *Handler) AdminList(c echo.Context) error {
	items, err := h.list.Handle(c.Request().Context(), false)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, listResponse{Items: adminTypes(items)})
}

func (h *Handler) Create(c echo.Context) error {
	var body typeBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	item, err := h.create.Handle(c.Request().Context(), body.Code, body.Label, body.SortOrder, body.IsActive)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusCreated, adminType(item))
}

func (h *Handler) Update(c echo.Context) error {
	var body typeBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	item, err := h.update.Handle(c.Request().Context(), c.Param("code"), body.LabelPtr(), body.SortOrder, body.IsActive)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, adminType(item))
}

type typeBody struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	SortOrder *int   `json:"sortOrder"`
	IsActive  *bool  `json:"isActive"`
}

func (b typeBody) LabelPtr() *string {
	if b.Label == "" && b.SortOrder == nil && b.IsActive == nil {
		return nil
	}
	if b.Label == "" {
		return nil
	}
	return &b.Label
}

type typeResponse struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	SortOrder int    `json:"sortOrder"`
	IsActive  *bool  `json:"isActive,omitempty"`
}

type listResponse struct {
	Items []typeResponse `json:"items"`
}

type statusItem struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type statusesResponse struct {
	Publication []statusItem `json:"publication"`
	Release     []statusItem `json:"release"`
}

func publicTypes(items []ports.Type) []typeResponse {
	out := make([]typeResponse, 0, len(items))
	for _, item := range items {
		out = append(out, typeResponse{Code: item.Code, Label: item.Label, SortOrder: item.SortOrder})
	}
	return out
}

func adminTypes(items []ports.Type) []typeResponse {
	out := make([]typeResponse, 0, len(items))
	for _, item := range items {
		out = append(out, adminType(item))
	}
	return out
}

func adminType(item ports.Type) typeResponse {
	active := item.IsActive
	return typeResponse{Code: item.Code, Label: item.Label, SortOrder: item.SortOrder, IsActive: &active}
}

func statusItems(items []StatusItem) []statusItem {
	out := make([]statusItem, 0, len(items))
	for _, item := range items {
		out = append(out, statusItem{Code: item.Code, Label: item.Label})
	}
	return out
}

func mapError(err error) error {
	var field contract.FieldError
	switch {
	case errors.As(err, &field):
		return &httperr.Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "VALIDATION_ERROR",
			Message: "invalid review type",
			Details: map[string]string{"field": field.Field},
		}
	case errors.Is(err, contract.ErrNotFound):
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	case errors.Is(err, contract.ErrConflict):
		return httperr.New(http.StatusConflict, "CONFLICT", "review type already exists")
	case errors.Is(err, contract.ErrInvalid):
		return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid review type")
	default:
		return err
	}
}
