package lists

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	catalogcontract "review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/command"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/application/query"
	"review-api/internal/modules/lists/internal/infrastructure/read"
	"review-api/internal/modules/lists/internal/infrastructure/write"
	mediacontract "review-api/internal/modules/media/contract"
	reviewscontract "review-api/internal/modules/reviews/contract"
	"review-api/internal/platform/transaction"
)

// CatalogLookup is the catalog module.
type CatalogLookup interface {
	MediaObjectRef(ctx context.Context, id uuid.UUID) (catalogcontract.MediaObjectRef, error)
}

// ImageAttacher is the media module.
type ImageAttacher interface {
	AttachImages(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) ([]mediacontract.ImageRef, error)
}

// ReviewDictionaries is the reviews module: review types and report reasons.
type ReviewDictionaries interface {
	RequireActiveTypes(ctx context.Context, codes []string) error
	RequireActiveReportReason(ctx context.Context, code string) error
}

type Dependencies struct {
	Pool          *pgxpool.Pool
	Catalog       CatalogLookup
	Images        ImageAttacher
	Reviews       ReviewDictionaries
	TemplateQuota int
	ResultQuota   int
}

type Module struct {
	createTemplate *command.CreateTemplateHandler
	saveTemplate   *command.SaveTemplateHandler
	templateState  *command.TemplateStateHandler
	copyTemplate   *command.CopyTemplateHandler
	getTemplate    *query.GetTemplateHandler
	results        *command.ResultHandler
	getResult      *query.GetResultHandler
	lists          *query.ListHandler
}

func New(deps Dependencies) *Module {
	writer := write.NewRepository(deps.Pool)
	reader := read.NewRepository(deps.Pool)
	tx := transaction.NewPgxManager(deps.Pool)
	catalog := catalogAdapter{deps.Catalog}
	types := typesAdapter{deps.Reviews}
	return &Module{
		createTemplate: command.NewCreateTemplateHandler(writer, types, tx, deps.TemplateQuota),
		saveTemplate:   command.NewSaveTemplateHandler(writer, catalog, imagesAdapter{deps.Images}, types, tx),
		templateState:  command.NewTemplateStateHandler(writer, catalog, tx),
		copyTemplate:   command.NewCopyTemplateHandler(writer, tx, deps.TemplateQuota),
		getTemplate:    query.NewGetTemplateHandler(reader),
		results:        command.NewResultHandler(writer, tx, deps.ResultQuota),
		getResult:      query.NewGetResultHandler(reader),
		lists:          query.NewListHandler(reader),
	}
}

func (m *Module) Name() string { return "lists" }

type catalogAdapter struct{ catalog CatalogLookup }

func (a catalogAdapter) Work(ctx context.Context, id uuid.UUID) (ports.CatalogWork, error) {
	if a.catalog == nil {
		return ports.CatalogWork{}, contract.ErrInvalid
	}
	ref, err := a.catalog.MediaObjectRef(ctx, id)
	if errors.Is(err, catalogcontract.ErrNotFound) {
		return ports.CatalogWork{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.CatalogWork{}, err
	}
	return ports.CatalogWork{ID: ref.ID, Type: ref.Type, Title: ref.Title, PosterURL: ref.PosterURL}, nil
}

type imagesAdapter struct{ images ImageAttacher }

func (a imagesAdapter) Attach(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if a.images == nil {
		return nil, contract.ErrInvalid
	}
	refs, err := a.images.AttachImages(ctx, ownerID, ids)
	if errors.Is(err, mediacontract.ErrNotFound) || errors.Is(err, mediacontract.ErrInvalid) {
		return nil, contract.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]string, len(refs))
	for _, ref := range refs {
		out[ref.ID] = ref.URL
	}
	return out, nil
}

type typesAdapter struct{ reviews ReviewDictionaries }

func (a typesAdapter) RequireActive(ctx context.Context, code string) error {
	if a.reviews == nil {
		return contract.ErrInvalid
	}
	err := a.reviews.RequireActiveTypes(ctx, []string{code})
	if _, ok := errors.AsType[reviewscontract.FieldError](err); ok {
		return contract.Field("typeCode", contract.ReasonInvalid)
	}
	return err
}
