package gamificationhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/gamification/internal/application/command"
	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/modules/gamification/internal/application/query"
	"review-api/internal/modules/gamification/internal/domain"
	"review-api/internal/platform/httpserver"
)

type memShop struct {
	items []ports.ShopItem
}

func (m *memShop) CreateItem(_ context.Context, item domain.ShopItem) error {
	m.items = append(m.items, ports.ShopItem{
		ID: item.ID, Code: item.Code, Kind: item.Kind, Title: item.Title,
		Price: item.Price, IsActive: item.IsActive, SortOrder: item.SortOrder,
	})
	return nil
}

func (m *memShop) UpdateItem(context.Context, uuid.UUID, *string, *int, *bool, *int) (ports.ShopItem, error) {
	return ports.ShopItem{}, nil
}

func (m *memShop) GetItem(context.Context, uuid.UUID) (ports.ShopItem, error) {
	return ports.ShopItem{}, nil
}

func (m *memShop) NextSort(context.Context) (int, error) { return 10, nil }

func (m *memShop) ListItems(_ context.Context, activeOnly bool) ([]ports.ShopItem, error) {
	out := make([]ports.ShopItem, 0, len(m.items))
	for _, item := range m.items {
		if activeOnly && !item.IsActive {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func TestAdminCreateRequiresToken(t *testing.T) {
	t.Parallel()
	shop := &memShop{}
	h := NewHandler(HandlerDeps{
		List:   query.NewListShopItemsHandler(shop),
		Create: command.NewCreateShopItemHandler(shop, nil),
		Update: command.NewUpdateShopItemHandler(shop, nil),
	})
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	token := strings.Repeat("a", 32)
	Register(e.Group("/v1"), h, httpserver.AdminToken(token), nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/shop-items", strings.NewReader(
		`{"code":"frame_gold","kind":"avatar_frame","title":"Gold","price":50}`,
	))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/admin/shop-items", strings.NewReader(
		`{"code":"frame_gold","kind":"avatar_frame","title":"Gold","price":50}`,
	))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(httpserver.AdminTokenHeader, token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
