package cataloghttp

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	"review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/catalog/internal/application/command"
	"review-api/internal/modules/catalog/internal/application/ports"
	"review-api/internal/modules/catalog/internal/application/query"
	"review-api/internal/modules/catalog/internal/domain"
)

type Handler struct {
	importBatch *command.ImportMediaObjectsHandler
	get         *query.GetMediaObjectHandler
	search      *query.SearchMediaObjectsHandler
}

func NewHandler(importBatch *command.ImportMediaObjectsHandler, get *query.GetMediaObjectHandler, search *query.SearchMediaObjectsHandler) *Handler {
	return &Handler{importBatch: importBatch, get: get, search: search}
}

func (h *Handler) Search(c echo.Context) error {
	limit := 20
	if raw := c.QueryParam("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return invalid("limit", -1)
		}
		limit = parsed
	}
	items, err := h.search.Handle(c.Request().Context(), c.QueryParam("q"), c.QueryParam("type"), limit)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, listResponse{Items: itemsResponse(items)})
}

func (h *Handler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "media object not found")
	}
	item, err := h.get.Handle(c.Request().Context(), id)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, itemResponse(item))
}

func (h *Handler) Import(c echo.Context) error {
	drafts, err := parseImportBody(c)
	if err != nil {
		return err
	}
	items, err := h.importBatch.Handle(c.Request().Context(), drafts)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusCreated, importResponse{Items: itemsResponse(items), Count: len(items)})
}

func parseImportBody(c echo.Context) ([]domain.Draft, error) {
	ct := c.Request().Header.Get(echo.HeaderContentType)
	switch {
	case strings.HasPrefix(ct, echo.MIMEApplicationJSON):
		var body importJSONBody
		if err := c.Bind(&body); err != nil {
			return nil, httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		}
		return body.Items, nil
	case strings.HasPrefix(ct, "text/csv"):
		return parseCSV(c.Request().Body)
	default:
		return nil, httperr.New(http.StatusUnsupportedMediaType, "VALIDATION_ERROR", "unsupported content type")
	}
}

func parseCSV(r io.Reader) ([]domain.Draft, error) {
	reader := csv.NewReader(r)
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return nil, invalid("items", -1)
	}
	col, err := csvColumns(header)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Draft, 0)
	for {
		rec, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, invalid("items", -1)
		}
		draft, err := draftFromCSV(rec, col)
		if err != nil {
			return nil, err
		}
		out = append(out, draft)
	}
	return out, nil
}

type csvCol struct {
	title, original, typ, genre, year, poster, status, source, externalID, info int
}

func csvColumns(header []string) (csvCol, error) {
	idx := map[string]int{}
	for i, name := range header {
		idx[strings.TrimSpace(strings.ToLower(name))] = i
	}
	required := []string{"title", "type"}
	if _, ok := idx["info"]; !ok {
		required = append(required, "release_status")
	}
	for _, name := range required {
		if _, ok := idx[name]; !ok {
			return csvCol{}, invalid("items", -1)
		}
	}
	col := csvCol{
		title: idx["title"], typ: idx["type"], status: -1, info: -1,
		original: -1, genre: -1, year: -1, poster: -1, source: -1, externalID: -1,
	}
	if i, ok := idx["release_status"]; ok {
		col.status = i
	}
	if i, ok := idx["info"]; ok {
		col.info = i
	}
	if i, ok := idx["original_title"]; ok {
		col.original = i
	}
	if i, ok := idx["genre"]; ok {
		col.genre = i
	}
	if i, ok := idx["release_year"]; ok {
		col.year = i
	}
	if i, ok := idx["poster_url"]; ok {
		col.poster = i
	}
	if i, ok := idx["external_source"]; ok {
		col.source = i
	}
	if i, ok := idx["external_id"]; ok {
		col.externalID = i
	}
	return col, nil
}

func draftFromCSV(rec []string, col csvCol) (domain.Draft, error) {
	get := func(i int) string {
		if i < 0 || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	draft := domain.Draft{
		Title:          get(col.title),
		OriginalTitle:  get(col.original),
		Type:           get(col.typ),
		Genre:          get(col.genre),
		PosterURL:      get(col.poster),
		ReleaseStatus:  get(col.status),
		ExternalSource: get(col.source),
		ExternalID:     get(col.externalID),
	}
	if col.info >= 0 {
		if err := json.Unmarshal([]byte(get(col.info)), &draft.Info); err != nil || draft.Info == nil {
			return domain.Draft{}, invalid("info", -1)
		}
	}
	if raw := get(col.year); raw != "" {
		year, err := strconv.Atoi(raw)
		if err != nil {
			return domain.Draft{}, invalid("releaseYear", -1)
		}
		draft.ReleaseYear = &year
	}
	return draft, nil
}

type importJSONBody struct {
	Items []domain.Draft `json:"items"`
}

type itemJSON struct {
	ID             string      `json:"id"`
	Title          string      `json:"title"`
	OriginalTitle  string      `json:"originalTitle,omitempty"`
	Type           string      `json:"type"`
	Info           domain.Info `json:"info"`
	PosterURL      string      `json:"posterUrl,omitempty"`
	ExternalSource string      `json:"externalSource,omitempty"`
	ExternalID     string      `json:"externalId,omitempty"`
	CreatedBy      *string     `json:"createdBy,omitempty"`
}

type listResponse struct {
	Items []itemJSON `json:"items"`
}

type importResponse struct {
	Items []itemJSON `json:"items"`
	Count int        `json:"count"`
}

func itemsResponse(items []ports.MediaObject) []itemJSON {
	out := make([]itemJSON, 0, len(items))
	for _, item := range items {
		out = append(out, itemResponse(item))
	}
	return out
}

func itemResponse(item ports.MediaObject) itemJSON {
	out := itemJSON{
		ID:             item.ID.String(),
		Title:          item.Title,
		OriginalTitle:  item.OriginalTitle,
		Type:           item.Type,
		Info:           item.Info,
		PosterURL:      item.PosterURL,
		ExternalSource: item.ExternalSource,
		ExternalID:     item.ExternalID,
	}
	if item.CreatedBy != nil {
		s := item.CreatedBy.String()
		out.CreatedBy = &s
	}
	return out
}

func invalid(field string, index int) error {
	details := map[string]any{"field": field}
	if index >= 0 {
		details["index"] = index
	}
	return &httperr.Error{
		Status:  http.StatusUnprocessableEntity,
		Code:    "VALIDATION_ERROR",
		Message: "invalid media object",
		Details: details,
	}
}

func mapError(err error) error {
	var field contract.FieldError
	switch {
	case errors.As(err, &field):
		return invalid(field.Field, field.Index)
	case errors.Is(err, contract.ErrNotFound):
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "media object not found")
	case errors.Is(err, contract.ErrInvalid):
		return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid media object")
	default:
		return err
	}
}
