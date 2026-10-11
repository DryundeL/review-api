package bot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientGetUpdates(t *testing.T) {
	t.Parallel()
	const token = "123456:abcDEF_token-1"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot"+token+"/getUpdates" {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"offset"`) {
			t.Errorf("offset sent on first poll: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"ok": true,
			"result": [{
				"update_id": 7,
				"message": {
					"from": {"id": 42, "is_bot": false, "first_name": "Ann", "username": "ann", "language_code": "en", "is_premium": true},
					"chat": {"id": 42, "type": "private"},
					"text": "/start"
				}
			}]
		}`)
	}))
	t.Cleanup(srv.Close)

	client := NewClient(token)
	client.base = srv.URL
	updates, err := client.GetUpdates(context.Background(), 0)
	if err != nil {
		t.Fatalf("getUpdates: %v", err)
	}
	if len(updates) != 1 || updates[0].UpdateID != 7 || updates[0].Message.From.ID != 42 || !updates[0].Message.From.IsPremium {
		t.Fatalf("updates = %+v", updates)
	}
}

func TestClientRedactsToken(t *testing.T) {
	t.Parallel()
	const token = "123456:abcDEF_token-1"
	client := NewClient(token)
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial %s", r.URL.String())
	})
	_, err := client.GetUpdates(context.Background(), 0)
	if err == nil || strings.Contains(err.Error(), token) {
		t.Fatalf("err = %v", err)
	}
}

func TestClientAPIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`)
	}))
	t.Cleanup(srv.Close)

	client := NewClient("123456:abcDEF_token-1")
	client.base = srv.URL
	err := client.SendMessage(context.Background(), 42, "Профиль сохранён.", nil)
	var api *APIError
	if !errorAs(err, &api) || api.Code != 403 || api.Temporary() {
		t.Fatalf("err = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func errorAs(err error, target **APIError) bool {
	api, ok := err.(*APIError)
	if !ok {
		return false
	}
	*target = api
	return true
}
