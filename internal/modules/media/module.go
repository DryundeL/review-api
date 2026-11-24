package media

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/media/internal/application/command"
	mediahttp "review-api/internal/modules/media/internal/delivery/http"
	"review-api/internal/modules/media/internal/infrastructure/quota"
	"review-api/internal/modules/media/internal/infrastructure/storage"
	"review-api/internal/modules/media/internal/infrastructure/write"
	"review-api/internal/platform/transaction"
)

// UnattachedTTL is how long an upload may stay unused before the worker removes it.
const UnattachedTTL = 24 * time.Hour

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Dependencies struct {
	Pool           *pgxpool.Pool
	Redis          *redis.Client
	Auth           echo.MiddlewareFunc
	Users          UserResolver
	StorageDir     string
	PublicPrefix   string
	UploadsPerHour int
}

type Module struct {
	upload       *command.UploadImageHandler
	collect      *command.CollectUnattachedHandler
	users        UserResolver
	auth         echo.MiddlewareFunc
	publicPrefix string
}

func New(deps Dependencies) *Module {
	repo := write.NewRepository(deps.Pool)
	files := storage.NewLocalStore(deps.StorageDir)
	return &Module{
		upload:       command.NewUploadImageHandler(repo, files, quota.NewRedis(deps.Redis, deps.UploadsPerHour, time.Hour)),
		collect:      command.NewCollectUnattachedHandler(repo, files, transaction.NewPgxManager(deps.Pool), UnattachedTTL),
		users:        deps.Users,
		auth:         deps.Auth,
		publicPrefix: deps.PublicPrefix,
	}
}

func (m *Module) Name() string { return "media" }

// CollectUnattached removes one batch of expired unattached uploads (worker job).
func (m *Module) CollectUnattached(ctx context.Context, batch int) (int, error) {
	return m.collect.Handle(ctx, batch)
}

func (m *Module) RegisterHTTP(g *echo.Group) {
	mediahttp.Register(g, mediahttp.NewHandler(m.upload, m.users, m.publicPrefix), m.auth)
}
