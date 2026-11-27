package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
	"review-api/internal/platform/transaction"
)

// TierInput without ID is a new tier; an ID must belong to the template.
type TierInput struct {
	ID    *uuid.UUID
	Name  string
	Color string
}

// ItemInput without ID is a new item: a catalog work (MediaObjectID) or a manual one.
// An ID must belong to the template; its catalog link cannot change. ImageID is an upload
// of the author or the image the item already has; nil keeps the current image.
type ItemInput struct {
	ID            *uuid.UUID
	MediaObjectID *uuid.UUID
	ImageID       *uuid.UUID
	Title         string
}

// SaveTemplateInput is a full replacement. Nil TypeCode or Format keeps the current value.
type SaveTemplateInput struct {
	Revision    int
	TypeCode    *string
	Format      *string
	Title       string
	Description string
	Tiers       []TierInput
	Items       []ItemInput
}

type SaveTemplateHandler struct {
	repo    ports.TemplateWriter
	catalog ports.Catalog
	images  ports.Images
	types   ports.Types
	tx      transaction.Manager
}

func NewSaveTemplateHandler(repo ports.TemplateWriter, catalog ports.Catalog, images ports.Images, types ports.Types, tx transaction.Manager) *SaveTemplateHandler {
	return &SaveTemplateHandler{repo: repo, catalog: catalog, images: images, types: types, tx: tx}
}

// Handle replaces the working state of the author's template and bumps its revision.
// Images are attached in the same transaction, so a rejected save leaves them unattached.
func (h *SaveTemplateHandler) Handle(ctx context.Context, id, authorID uuid.UUID, in SaveTemplateInput) (domain.Template, error) {
	if h == nil || h.repo == nil || h.catalog == nil || h.images == nil || h.types == nil || h.tx == nil {
		return domain.Template{}, contract.ErrInvalid
	}
	// Bound the input before any lookup; the domain repeats these checks.
	if len(in.Items) > domain.MaxItems {
		return domain.Template{}, contract.Field("items", contract.ReasonTooMany)
	}
	if len(in.Tiers) > domain.MaxTiers {
		return domain.Template{}, contract.Field("tiers", contract.ReasonTooMany)
	}
	// Catalog data of new items is fetched before the row lock is taken.
	works, err := h.newWorks(ctx, in.Items)
	if err != nil {
		return domain.Template{}, err
	}
	var saved domain.Template
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		tpl, err := lockOwnTemplate(txCtx, h.repo, id, authorID)
		if err != nil {
			return err
		}
		if tpl.Revision != in.Revision {
			return contract.ErrRevisionConflict
		}
		content, err := h.content(txCtx, tpl, in, works)
		if err != nil {
			return err
		}
		if err := tpl.Replace(content); err != nil {
			return err
		}
		tpl.Revision++
		if err := h.repo.UpdateTemplate(txCtx, tpl, true); err != nil {
			return err
		}
		saved = tpl
		return nil
	})
	if err != nil {
		return domain.Template{}, fmt.Errorf("save list template: %w", err)
	}
	return saved, nil
}

func (h *SaveTemplateHandler) newWorks(ctx context.Context, items []ItemInput) (map[uuid.UUID]ports.CatalogWork, error) {
	works := make(map[uuid.UUID]ports.CatalogWork)
	for i, item := range items {
		if item.ID != nil || item.MediaObjectID == nil {
			continue
		}
		workID := *item.MediaObjectID
		if _, ok := works[workID]; ok {
			continue // the domain reports the duplicate
		}
		work, err := h.catalog.Work(ctx, workID)
		if errors.Is(err, contract.ErrNotFound) {
			return nil, contract.FieldAt("items.mediaObjectId", i, contract.ReasonInvalid)
		}
		if err != nil {
			return nil, err
		}
		works[workID] = work
	}
	return works, nil
}

// content resolves the request against the stored template: new ids, catalog data and image URLs.
func (h *SaveTemplateHandler) content(ctx context.Context, tpl domain.Template, in SaveTemplateInput, works map[uuid.UUID]ports.CatalogWork) (domain.TemplateContent, error) {
	c := domain.TemplateContent{TypeCode: tpl.TypeCode, Format: tpl.Format, Title: in.Title, Description: in.Description}
	if in.TypeCode != nil {
		c.TypeCode = strings.TrimSpace(*in.TypeCode)
	}
	if in.Format != nil {
		f, err := domain.ParseFormat(*in.Format)
		if err != nil {
			return c, err
		}
		c.Format = f
	}
	typeChanged := c.TypeCode != tpl.TypeCode
	// After the first publication the domain rejects the change as immutable.
	if typeChanged && tpl.FirstPublishedAt == nil && c.TypeCode != "" && c.TypeCode != domain.TypeMixed {
		if err := h.types.RequireActive(ctx, c.TypeCode); err != nil {
			return c, err
		}
	}
	tiers, err := resolveTiers(tpl.Tiers, in.Tiers)
	if err != nil {
		return c, err
	}
	items, uploads, err := resolveItems(tpl.Items, in.Items, works)
	if err != nil {
		return c, err
	}
	if err := h.checkWorkTypes(ctx, c.TypeCode, items, works, typeChanged); err != nil {
		return c, err
	}
	if err := h.attach(ctx, tpl.AuthorID, items, uploads); err != nil {
		return c, err
	}
	c.Tiers, c.Items = tiers, items
	return c, nil
}

func resolveTiers(current []domain.Tier, in []TierInput) ([]domain.Tier, error) {
	known := make(map[uuid.UUID]struct{}, len(current))
	for _, t := range current {
		known[t.ID] = struct{}{}
	}
	out := make([]domain.Tier, 0, len(in))
	for i, t := range in {
		id := uuid.New()
		if t.ID != nil {
			if _, ok := known[*t.ID]; !ok {
				return nil, contract.FieldAt("tiers.id", i, contract.ReasonInvalid)
			}
			id = *t.ID
		}
		out = append(out, domain.Tier{ID: id, Name: t.Name, Color: t.Color})
	}
	return out, nil
}

// resolveItems returns the items and, for each item index, the upload that still has to be attached.
func resolveItems(current []domain.Item, in []ItemInput, works map[uuid.UUID]ports.CatalogWork) ([]domain.Item, map[int]uuid.UUID, error) {
	known := make(map[uuid.UUID]domain.Item, len(current))
	for _, it := range current {
		known[it.ID] = it
	}
	out := make([]domain.Item, 0, len(in))
	uploads := make(map[int]uuid.UUID)
	for i, req := range in {
		var item domain.Item
		if req.ID != nil {
			prev, ok := known[*req.ID]
			if !ok {
				return nil, nil, contract.FieldAt("items.id", i, contract.ReasonInvalid)
			}
			if req.MediaObjectID != nil && (prev.MediaObjectID == nil || *prev.MediaObjectID != *req.MediaObjectID) {
				return nil, nil, contract.FieldAt("items.mediaObjectId", i, contract.ReasonImmutable)
			}
			item = prev
			item.Title = req.Title
		} else {
			item = domain.Item{ID: uuid.New(), Title: req.Title}
			if req.MediaObjectID != nil {
				work := works[*req.MediaObjectID]
				id := work.ID
				item.MediaObjectID = &id
				item.ImageURL = work.PosterURL
				if strings.TrimSpace(req.Title) == "" {
					item.Title = work.Title
				}
			}
		}
		// The image the item already has stays without a new attach (including foreign uploads in copies).
		if req.ImageID != nil && (item.ImageID == nil || *item.ImageID != *req.ImageID) {
			if *req.ImageID == uuid.Nil {
				return nil, nil, contract.FieldAt("items.imageId", i, contract.ReasonInvalid)
			}
			uploads[i] = *req.ImageID
		}
		out = append(out, item)
	}
	return out, uploads, nil
}

// checkWorkTypes requires every catalog item to match the template type; a mixed template takes
// any. Existing items are re-checked only when the type changes.
func (h *SaveTemplateHandler) checkWorkTypes(ctx context.Context, typeCode string, items []domain.Item, works map[uuid.UUID]ports.CatalogWork, typeChanged bool) error {
	if typeCode == domain.TypeMixed {
		return nil
	}
	for i, item := range items {
		if item.MediaObjectID == nil {
			continue
		}
		work, isNew := works[*item.MediaObjectID]
		if !isNew {
			if !typeChanged {
				continue
			}
			var err error
			work, err = h.catalog.Work(ctx, *item.MediaObjectID)
			if errors.Is(err, contract.ErrNotFound) {
				continue // removed from the catalog: the item is manual from now on
			}
			if err != nil {
				return err
			}
		}
		if work.Type != typeCode {
			return contract.FieldAt("items.mediaObjectId", i, contract.ReasonMismatch)
		}
	}
	return nil
}

func (h *SaveTemplateHandler) attach(ctx context.Context, authorID uuid.UUID, items []domain.Item, uploads map[int]uuid.UUID) error {
	if len(uploads) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(uploads))
	for _, id := range uploads {
		ids = append(ids, id)
	}
	urls, err := h.images.Attach(ctx, authorID, ids)
	if errors.Is(err, contract.ErrNotFound) {
		return contract.Field("items.imageId", contract.ReasonInvalid)
	}
	if err != nil {
		return err
	}
	for i, id := range uploads {
		url, ok := urls[id]
		if !ok {
			return contract.FieldAt("items.imageId", i, contract.ReasonInvalid)
		}
		imageID := id
		items[i].ImageID = &imageID
		items[i].ImageURL = url
	}
	return nil
}
