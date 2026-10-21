package gamificationhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/command"
	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/modules/gamification/internal/application/query"
)

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Handler struct {
	list              *query.ListShopItemsHandler
	create            *command.CreateShopItemHandler
	update            *command.UpdateShopItemHandler
	me                *query.GetMeHandler
	purchase          *command.PurchaseHandler
	equip             *command.EquipHandler
	listAchievements  *query.ListAchievementsHandler
	listProgress      *query.ListProgressHandler
	createAchievement *command.CreateAchievementHandler
	updateAchievement *command.UpdateAchievementHandler
	users             UserResolver
}

type HandlerDeps struct {
	List              *query.ListShopItemsHandler
	Create            *command.CreateShopItemHandler
	Update            *command.UpdateShopItemHandler
	Me                *query.GetMeHandler
	Purchase          *command.PurchaseHandler
	Equip             *command.EquipHandler
	ListAchievements  *query.ListAchievementsHandler
	ListProgress      *query.ListProgressHandler
	CreateAchievement *command.CreateAchievementHandler
	UpdateAchievement *command.UpdateAchievementHandler
	Users             UserResolver
}

func NewHandler(deps HandlerDeps) *Handler {
	return &Handler{
		list: deps.List, create: deps.Create, update: deps.Update,
		me: deps.Me, purchase: deps.Purchase, equip: deps.Equip, users: deps.Users,
		listAchievements: deps.ListAchievements, listProgress: deps.ListProgress,
		createAchievement: deps.CreateAchievement, updateAchievement: deps.UpdateAchievement,
	}
}

func (h *Handler) ListShop(c echo.Context) error {
	items, err := h.list.Handle(c.Request().Context(), true)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=60")
	return c.JSON(http.StatusOK, listResponse{Items: publicItems(items)})
}

func (h *Handler) AdminList(c echo.Context) error {
	items, err := h.list.Handle(c.Request().Context(), false)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, listResponse{Items: adminItems(items)})
}

func (h *Handler) CreateItem(c echo.Context) error {
	var body createItemBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	item, err := h.create.Handle(
		c.Request().Context(),
		body.Code,
		command.NormalizeKind(body.Kind),
		body.Title,
		body.Price,
		body.SortOrder,
		body.IsActive,
	)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusCreated, adminItem(item))
}

func (h *Handler) UpdateItem(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	}
	var body updateItemBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	item, err := h.update.Handle(c.Request().Context(), id, body.Title, body.Price, body.IsActive, body.SortOrder)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, adminItem(item))
}

func (h *Handler) ListProfileAchievements(c echo.Context) error {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil || userID == uuid.Nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	}
	if h.listProgress == nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	}
	items, err := h.listProgress.Handle(c.Request().Context(), userID)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, achievementsListResponse{Achievements: progressItems(items)})
}

func (h *Handler) AdminListAchievements(c echo.Context) error {
	if h.listAchievements == nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	}
	items, err := h.listAchievements.Handle(c.Request().Context(), false)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, adminAchievementsResponse{Achievements: adminAchievementItems(items)})
}

func (h *Handler) CreateAchievement(c echo.Context) error {
	if h.createAchievement == nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	}
	var body createAchievementBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	item, err := h.createAchievement.Handle(
		c.Request().Context(),
		body.Code, body.Title, body.Description, body.Metric,
		body.Target, body.RewardPoints, body.SortOrder, body.IsActive,
	)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusCreated, adminAchievementItem(item))
}

func (h *Handler) UpdateAchievement(c echo.Context) error {
	if h.updateAchievement == nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	}
	var body updateAchievementBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	item, err := h.updateAchievement.Handle(
		c.Request().Context(), id,
		body.Title, body.Description, body.Target, body.RewardPoints, body.SortOrder, body.IsActive,
	)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, adminAchievementItem(item))
}

func (h *Handler) Me(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	me, err := h.me.Handle(c.Request().Context(), userID)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, meJSON(me))
}

func (h *Handler) Purchase(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	var body purchaseBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	itemID, err := uuid.Parse(body.ItemID)
	if err != nil || itemID == uuid.Nil {
		return &httperr.Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "VALIDATION_ERROR",
			Message: "invalid purchase",
			Details: map[string]string{"field": "itemId"},
		}
	}
	if err := h.purchase.Handle(c.Request().Context(), userID, itemID); err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) Equip(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	in := command.EquipInput{}
	if v, ok := rawMap["background"]; ok {
		s, err := decodeOptionalString(v)
		if err != nil {
			return &httperr.Error{
				Status: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR",
				Message: "invalid equip", Details: map[string]string{"field": "background"},
			}
		}
		in.Background = s
	}
	if v, ok := rawMap["avatarFrame"]; ok {
		s, err := decodeOptionalString(v)
		if err != nil {
			return &httperr.Error{
				Status: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR",
				Message: "invalid equip", Details: map[string]string{"field": "avatarFrame"},
			}
		}
		in.AvatarFrame = s
	}
	if v, ok := rawMap["backdrop"]; ok {
		s, err := decodeOptionalString(v)
		if err != nil {
			return &httperr.Error{
				Status: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR",
				Message: "invalid equip", Details: map[string]string{"field": "backdrop"},
			}
		}
		in.Backdrop = s
	}
	profile, err := h.equip.Handle(c.Request().Context(), userID, in)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, equippedJSON(profile))
}

func decodeOptionalString(raw json.RawMessage) (*string, error) {
	if string(raw) == "null" {
		empty := ""
		return &empty, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (h *Handler) requireUser(c echo.Context) (uuid.UUID, error) {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	if h.users == nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	userID, err := h.users.UserID(c.Request().Context(), id.TelegramID)
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	return userID, nil
}

type createItemBody struct {
	Code      string `json:"code"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Price     int    `json:"price"`
	SortOrder *int   `json:"sortOrder"`
	IsActive  *bool  `json:"isActive"`
}

type updateItemBody struct {
	Title     *string `json:"title"`
	Price     *int    `json:"price"`
	SortOrder *int    `json:"sortOrder"`
	IsActive  *bool   `json:"isActive"`
}

type purchaseBody struct {
	ItemID string `json:"itemId"`
}

type createAchievementBody struct {
	Code         string `json:"code"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Metric       string `json:"metric"`
	Target       int    `json:"target"`
	RewardPoints int    `json:"rewardPoints"`
	SortOrder    *int   `json:"sortOrder"`
	IsActive     *bool  `json:"isActive"`
}

type updateAchievementBody struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	Target       *int    `json:"target"`
	RewardPoints *int    `json:"rewardPoints"`
	SortOrder    *int    `json:"sortOrder"`
	IsActive     *bool   `json:"isActive"`
}

type achievementResponse struct {
	Code        string  `json:"code"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Progress    int     `json:"progress"`
	Current     int     `json:"current"`
	Target      int     `json:"target"`
	UnlockedAt  *string `json:"unlockedAt,omitempty"`
}

type achievementsListResponse struct {
	Achievements []achievementResponse `json:"achievements"`
}

type adminAchievementResponse struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Metric       string `json:"metric"`
	Target       int    `json:"target"`
	RewardPoints int    `json:"rewardPoints"`
	SortOrder    int    `json:"sortOrder"`
	IsActive     bool   `json:"isActive"`
}

type adminAchievementsResponse struct {
	Achievements []adminAchievementResponse `json:"achievements"`
}

type itemResponse struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Price     int    `json:"price"`
	SortOrder int    `json:"sortOrder"`
	IsActive  *bool  `json:"isActive,omitempty"`
}

type listResponse struct {
	Items []itemResponse `json:"items"`
}

type meResponse struct {
	PointsBalance int              `json:"pointsBalance"`
	OwnedCodes    []string         `json:"ownedCodes"`
	Equipped      equippedResponse `json:"equipped"`
}

type equippedResponse struct {
	Background  string `json:"background,omitempty"`
	AvatarFrame string `json:"avatarFrame,omitempty"`
	Backdrop    string `json:"backdrop,omitempty"`
}

func publicItems(items []ports.ShopItem) []itemResponse {
	out := make([]itemResponse, 0, len(items))
	for _, item := range items {
		out = append(out, itemResponse{
			ID: item.ID.String(), Code: item.Code, Kind: item.Kind,
			Title: item.Title, Price: item.Price, SortOrder: item.SortOrder,
		})
	}
	return out
}

func adminItems(items []ports.ShopItem) []itemResponse {
	out := make([]itemResponse, 0, len(items))
	for _, item := range items {
		out = append(out, adminItem(item))
	}
	return out
}

func adminItem(item ports.ShopItem) itemResponse {
	active := item.IsActive
	return itemResponse{
		ID: item.ID.String(), Code: item.Code, Kind: item.Kind,
		Title: item.Title, Price: item.Price, SortOrder: item.SortOrder, IsActive: &active,
	}
}

func meJSON(me ports.Me) meResponse {
	codes := me.OwnedCodes
	if codes == nil {
		codes = []string{}
	}
	return meResponse{
		PointsBalance: me.PointsBalance,
		OwnedCodes:    codes,
		Equipped:      equippedJSON(me.Profile),
	}
}

func equippedJSON(p ports.Profile) equippedResponse {
	return equippedResponse{
		Background:  p.EquippedBackground,
		AvatarFrame: p.EquippedAvatarFrame,
		Backdrop:    p.EquippedBackdrop,
	}
}

func progressItems(items []ports.AchievementProgress) []achievementResponse {
	out := make([]achievementResponse, 0, len(items))
	for _, item := range items {
		out = append(out, achievementResponse{
			Code: item.Code, Title: item.Title, Description: item.Description,
			Progress: query.ProgressPercent(item.Current, item.Target),
			Current:  item.Current, Target: item.Target, UnlockedAt: item.UnlockedAt,
		})
	}
	return out
}

func adminAchievementItems(items []ports.AchievementItem) []adminAchievementResponse {
	out := make([]adminAchievementResponse, 0, len(items))
	for _, item := range items {
		out = append(out, adminAchievementItem(item))
	}
	return out
}

func adminAchievementItem(item ports.AchievementItem) adminAchievementResponse {
	return adminAchievementResponse{
		ID: item.ID.String(), Code: item.Code, Title: item.Title, Description: item.Description,
		Metric: item.Metric, Target: item.Target, RewardPoints: item.RewardPoints,
		SortOrder: item.SortOrder, IsActive: item.IsActive,
	}
}

func mapError(err error) error {
	var field contract.FieldError
	switch {
	case errors.As(err, &field):
		return &httperr.Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "VALIDATION_ERROR",
			Message: "invalid gamification input",
			Details: map[string]string{"field": field.Field},
		}
	case errors.Is(err, contract.ErrNotFound):
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	case errors.Is(err, contract.ErrConflict):
		return httperr.New(http.StatusConflict, "CONFLICT", "already exists")
	case errors.Is(err, contract.ErrInsufficient):
		return httperr.New(http.StatusConflict, "INSUFFICIENT_POINTS", "not enough points")
	case errors.Is(err, contract.ErrNotOwned):
		return httperr.New(http.StatusUnprocessableEntity, "NOT_OWNED", "item not owned")
	case errors.Is(err, contract.ErrInvalid):
		return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid gamification input")
	default:
		return err
	}
}
