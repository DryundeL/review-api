package mediahttp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/media/contract"
	"review-api/internal/modules/media/internal/application/command"
	"review-api/internal/modules/media/internal/application/ports"
)

const (
	// maxRequestBytes leaves room for multipart headers around one MaxImageBytes file.
	maxRequestBytes = contract.MaxImageBytes + 16*1024
	maxParts        = 4
)

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Handler struct {
	upload       *command.UploadImageHandler
	users        UserResolver
	publicPrefix string
}

func NewHandler(upload *command.UploadImageHandler, users UserResolver, publicPrefix string) *Handler {
	return &Handler{upload: upload, users: users, publicPrefix: strings.TrimRight(publicPrefix, "/")}
}

type imageResponse struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

// UploadImage accepts multipart/form-data with a single "file" part.
// File name and part Content-Type are ignored: the format comes from the bytes.
func (h *Handler) UploadImage(c echo.Context) error {
	ownerID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	data, err := readFilePart(c)
	if err != nil {
		return err
	}
	rec, err := h.upload.Handle(c.Request().Context(), ownerID, data)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusCreated, h.response(rec))
}

func (h *Handler) response(rec ports.ImageRecord) imageResponse {
	return imageResponse{
		ID:          rec.ID.String(),
		URL:         h.publicPrefix + "/" + rec.StorageKey,
		ContentType: rec.ContentType,
		Width:       rec.Width,
		Height:      rec.Height,
	}
}

func readFilePart(c echo.Context) ([]byte, error) {
	req := c.Request()
	req.Body = http.MaxBytesReader(c.Response(), req.Body, maxRequestBytes)
	mr, err := req.MultipartReader()
	if err != nil {
		return nil, badRequest()
	}
	for range maxParts {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, readError(err)
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, contract.MaxImageBytes+1))
		_ = part.Close()
		if err != nil {
			return nil, readError(err)
		}
		if len(data) > contract.MaxImageBytes {
			return nil, mapError(contract.ErrTooLarge)
		}
		return data, nil
	}
	return nil, validation("file")
}

func readError(err error) error {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return mapError(contract.ErrTooLarge)
	}
	return badRequest()
}

func (h *Handler) requireUser(c echo.Context) (uuid.UUID, error) {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil || h.users == nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	userID, err := h.users.UserID(c.Request().Context(), id.TelegramID)
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	return userID, nil
}

func badRequest() error {
	return httperr.New(http.StatusBadRequest, "BAD_REQUEST", "expected multipart/form-data with a file part")
}

func validation(field string) error {
	return &httperr.Error{
		Status:  http.StatusUnprocessableEntity,
		Code:    "VALIDATION_ERROR",
		Message: "invalid image",
		Details: map[string]string{"field": field},
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, contract.ErrTooLarge):
		return httperr.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "image must be at most 300 KB")
	case errors.Is(err, contract.ErrUnsupportedFormat):
		return httperr.New(http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "only JPEG, PNG and WebP are allowed")
	case errors.Is(err, contract.ErrDimensions):
		return &httperr.Error{
			Status: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR", Message: "invalid image",
			Details: map[string]string{"field": "file", "reason": "dimensions"},
		}
	case errors.Is(err, contract.ErrInvalid):
		return validation("file")
	case errors.Is(err, contract.ErrQuotaExceeded):
		return httperr.New(http.StatusTooManyRequests, "RATE_LIMITED", "too many uploads")
	default:
		return err
	}
}
