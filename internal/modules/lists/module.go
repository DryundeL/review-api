package lists

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	catalogcontract "review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/command"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/application/query"
	listshttp "review-api/internal/modules/lists/internal/delivery/http"
	"review-api/internal/modules/lists/internal/infrastructure/read"
	"review-api/internal/modules/lists/internal/infrastructure/write"
	mediacontract "review-api/internal/modules/media/contract"
	reviewscontract "review-api/internal/modules/reviews/contract"
	"review-api/internal/platform/ratelimit"
	"review-api/internal/platform/transaction"
)

// DefaultReportsPerHour is used when the config leaves the report limit unset.
const DefaultReportsPerHour = 20

// UserResolver is the users module.
type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

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
	Pool           *pgxpool.Pool
	Redis          *redis.Client
	Auth           echo.MiddlewareFunc
	Admin          echo.MiddlewareFunc
	Users          UserResolver
	Catalog        CatalogLookup
	Images         ImageAttacher
	Reviews        ReviewDictionaries
	TemplateQuota  int
	ResultQuota    int
	ReportsPerHour int
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
	reports        *command.ReportHandler
	moderation     *command.ModerationHandler
	reportQueue    *query.ReportQueueHandler
	reactions      *command.ReactionHandler
	comments       *query.CommentsHandler
	users          UserResolver
	auth           echo.MiddlewareFunc
	admin          echo.MiddlewareFunc
	reportLimiter  reportLimiter
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
		reports:        command.NewReportHandler(reader, reader, writer, reasonsAdapter{deps.Reviews}),
		moderation:     command.NewModerationHandler(writer),
		reportQueue:    query.NewReportQueueHandler(reader),
		reactions:      command.NewReactionHandler(writer, reader, tx),
		comments:       query.NewCommentsHandler(reader),
		users:          deps.Users,
		auth:           deps.Auth,
		admin:          deps.Admin,
		reportLimiter:  reportLimiter{rdb: deps.Redis, limit: reportsPerHour(deps.ReportsPerHour)},
	}
}

func (m *Module) Name() string { return "lists" }

func (m *Module) RegisterHTTP(g *echo.Group, profileAccess echo.MiddlewareFunc) {
	h := listshttp.NewHandler(listshttp.Deps{
		CreateTemplate: m.createTemplate, SaveTemplate: m.saveTemplate, TemplateState: m.templateState,
		CopyTemplate: m.copyTemplate, GetTemplate: m.getTemplate, Results: m.results, GetResult: m.getResult,
		Lists: m.lists, Reports: m.reports, Moderation: m.moderation, ReportQueue: m.reportQueue,
		Reactions: m.reactions, Comments: m.comments,
		Users: m.users, ReportLimiter: m.reportLimiter,
	})
	listshttp.Register(g, h, m.admin, m.auth, profileAccess)
}

func reportsPerHour(n int) int {
	if n > 0 {
		return n
	}
	return DefaultReportsPerHour
}

// reportLimiter caps reports per user and hour, on top of the per-IP limit of the API.
type reportLimiter struct {
	rdb   *redis.Client
	limit int
}

func (l reportLimiter) Allow(ctx context.Context, userID uuid.UUID) (bool, error) {
	return ratelimit.Allow(ctx, l.rdb, "rl:lists:reports:"+userID.String(), l.limit, time.Hour)
}

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

type reasonsAdapter struct{ reviews ReviewDictionaries }

func (a reasonsAdapter) RequireActive(ctx context.Context, code string) error {
	if a.reviews == nil {
		return contract.ErrInvalid
	}
	err := a.reviews.RequireActiveReportReason(ctx, code)
	if _, ok := errors.AsType[reviewscontract.FieldError](err); ok {
		return contract.Field("reasonCode", contract.ReasonInvalid)
	}
	return err
}
