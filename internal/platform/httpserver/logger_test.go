package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
)

func TestLoggerRecordsErrorStatusAndCause(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	e := echo.New()
	e.HTTPErrorHandler = ErrorHandler
	e.Use(LoggerMiddleware(slog.New(slog.NewJSONHandler(&buf, nil))))
	e.GET("/boom", func(echo.Context) error { return errors.New("store image: permission denied") })
	e.GET("/gone", func(echo.Context) error { return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found") })

	for path, want := range map[string]int{"/boom": http.StatusInternalServerError, "/gone": http.StatusNotFound} {
		buf.Reset()
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Fatalf("%s: status %d, want %d", path, rec.Code, want)
		}
		if strings.Contains(rec.Body.String(), "permission denied") {
			t.Fatalf("%s: the cause leaked to the client: %s", path, rec.Body)
		}
		var access map[string]any
		var cause string
		for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				t.Fatalf("log line %q: %v", line, err)
			}
			switch entry["msg"] {
			case "http request":
				access = entry
			case "unhandled error":
				cause, _ = entry["error"].(string)
			}
		}
		if access == nil || access["status"] != float64(want) {
			t.Fatalf("%s: access log = %v, want status %d", path, access, want)
		}
		if wantCause := want >= 500; (cause != "") != wantCause || (wantCause && !strings.Contains(cause, "permission denied")) {
			t.Fatalf("%s: logged cause = %q", path, cause)
		}
	}
}
