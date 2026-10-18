package usershttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/ports"
	"review-api/internal/modules/users/internal/application/query"
	"review-api/internal/modules/users/internal/domain"
	"review-api/internal/platform/httpserver"
)

type stubProfiles struct {
	user        domain.User
	followers   int64
	following   int64
	isFollowing bool
}

func (s stubProfiles) FindByTelegramID(context.Context, int64) (domain.User, error) {
	return s.user, nil
}

func (s stubProfiles) FindCardByID(_ context.Context, userID, viewerID uuid.UUID) (ports.ProfileCard, error) {
	if userID != s.user.ID {
		return ports.ProfileCard{}, contract.ErrNotFound
	}
	following := s.isFollowing && viewerID != uuid.Nil && viewerID != s.user.ID
	return ports.ProfileCard{
		User: s.user, FollowersCount: s.followers, FollowingCount: s.following, IsFollowing: following,
	}, nil
}

func (s stubProfiles) FindCardByTelegramID(_ context.Context, telegramID int64, _ uuid.UUID) (ports.ProfileCard, error) {
	if telegramID != s.user.TelegramID {
		return ports.ProfileCard{}, contract.ErrNotFound
	}
	return ports.ProfileCard{User: s.user, FollowersCount: s.followers, FollowingCount: s.following}, nil
}

type stubUsers struct {
	id uuid.UUID
}

func (s stubUsers) UserID(context.Context, int64) (uuid.UUID, error) {
	return s.id, nil
}

func TestProfile(t *testing.T) {
	t.Parallel()
	uid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	h := NewHandler(query.NewGetProfileHandler(stubProfiles{
		user: domain.User{
			ID: uid, TelegramID: 42, Username: "ann", FirstName: "Ann",
			PhotoURL: "https://t.me/i/userpic/320/ann.jpg",
		},
		followers: 3, following: 1,
	}), nil, nil, stubUsers{id: uid})
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	e.GET("/v1/profile", h.Profile, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := authcontract.WithIdentity(c.Request().Context(), authcontract.TelegramIdentity{TelegramID: 42})
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body profileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.IsMe || body.FollowersCount != 3 || body.FollowingCount != 1 || body.TelegramID == nil || *body.TelegramID != 42 {
		t.Fatalf("body = %+v", body)
	}
	if body.PhotoURL != "https://t.me/i/userpic/320/ann.jpg" {
		t.Fatalf("photoUrl = %q", body.PhotoURL)
	}
}

func TestPublicProfile_isMeAndIsFollowing(t *testing.T) {
	t.Parallel()
	target := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	viewer := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	h := NewHandler(query.NewGetProfileHandler(stubProfiles{
		user:      domain.User{ID: target, TelegramID: 42, Username: "ann", FirstName: "Ann"},
		followers: 2, isFollowing: true,
	}), nil, nil, stubUsers{id: viewer})
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	e.GET("/v1/profiles/:id", h.PublicProfile, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := authcontract.WithIdentity(c.Request().Context(), authcontract.TelegramIdentity{TelegramID: 99})
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/profiles/"+target.String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body profileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.IsMe || !body.IsFollowing || body.FollowersCount != 2 || body.TelegramID != nil {
		t.Fatalf("other body = %+v", body)
	}

	// own id via /profiles/{id}
	h = NewHandler(query.NewGetProfileHandler(stubProfiles{
		user:      domain.User{ID: viewer, TelegramID: 99, Username: "bob", FirstName: "Bob"},
		followers: 0,
	}), nil, nil, stubUsers{id: viewer})
	e = echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	e.GET("/v1/profiles/:id", h.PublicProfile, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := authcontract.WithIdentity(c.Request().Context(), authcontract.TelegramIdentity{TelegramID: 99})
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
	req = httptest.NewRequest(http.MethodGet, "/v1/profiles/"+viewer.String(), nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.IsMe || body.IsFollowing || body.TelegramID == nil {
		t.Fatalf("self via id body = %+v", body)
	}
}
