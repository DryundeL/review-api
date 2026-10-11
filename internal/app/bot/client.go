package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	telegramAPIBase = "https://api.telegram.org"
	maxResponseSize = 1 << 20
	pollSeconds     = 25
)

// APIError is a Telegram Bot API rejection. Temporary is true for rate limits and 5xx.
type APIError struct {
	Method      string
	Code        int
	Description string
	RetryAfter  time.Duration
}

func (e *APIError) Error() string {
	if e.Description == "" {
		return fmt.Sprintf("telegram %s: code %d", e.Method, e.Code)
	}
	return fmt.Sprintf("telegram %s: code %d: %s", e.Method, e.Code, e.Description)
}

func (e *APIError) Temporary() bool {
	return e.Code == 429 || e.Code >= 500
}

type Client struct {
	token string
	base  string
	http  *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token: token,
		base:  telegramAPIBase,
		http: &http.Client{
			Timeout: 40 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	From *User  `json:"from"`
	Chat Chat   `json:"chat"`
	Text string `json:"text"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type User struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
	IsPremium    bool   `json:"is_premium"`
}

func (c *Client) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	payload := map[string]any{
		"timeout":         pollSeconds,
		"allowed_updates": []string{"message"},
	}
	if offset > 0 {
		payload["offset"] = offset
	}
	var updates []Update
	if err := c.post(ctx, "getUpdates", payload, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	if chatID <= 0 || text == "" {
		return fmt.Errorf("telegram sendMessage: invalid request")
	}
	return c.post(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	}, nil)
}

func (c *Client) post(ctx context.Context, method string, payload any, result any) error {
	if !validToken(c.token) {
		return fmt.Errorf("telegram %s: invalid bot token", method)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram %s: encode request", method)
	}
	endpoint := c.base + "/bot" + c.token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, redact(err, c.token))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, redact(err, c.token))
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return fmt.Errorf("telegram %s: read response", method)
	}
	if len(raw) > maxResponseSize {
		return fmt.Errorf("telegram %s: response too large", method)
	}

	var envelope struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		if resp.StatusCode >= 400 {
			return &APIError{Method: method, Code: resp.StatusCode}
		}
		return fmt.Errorf("telegram %s: decode response", method)
	}
	if !envelope.OK {
		code := envelope.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		return &APIError{
			Method:      method,
			Code:        code,
			Description: clip(redactString(envelope.Description, c.token), 180),
			RetryAfter:  time.Duration(envelope.Parameters.RetryAfter) * time.Second,
		}
	}
	if result == nil || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("telegram %s: decode result", method)
	}
	return nil
}

func validToken(token string) bool {
	parts := strings.Split(token, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	for _, r := range parts[0] {
		if r < '0' || r > '9' {
			return false
		}
	}
	for _, r := range parts[1] {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

func redact(err error, token string) error {
	if err == nil {
		return nil
	}
	return errors.New(redactString(err.Error(), token))
}

func redactString(value, token string) string {
	if token == "" {
		return value
	}
	return strings.ReplaceAll(value, token, "[redacted]")
}

func clip(value string, maxRunes int) string {
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	return string([]rune(value)[:maxRunes])
}
