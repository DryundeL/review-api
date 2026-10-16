package reviews

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/command"
	"review-api/internal/modules/reviews/internal/application/query"
	reviewshttp "review-api/internal/modules/reviews/internal/delivery/http"
	outboxpub "review-api/internal/modules/reviews/internal/infrastructure/outbox"
	"review-api/internal/modules/reviews/internal/infrastructure/read"
	"review-api/internal/modules/reviews/internal/infrastructure/write"
	"review-api/internal/platform/outbox"
	"review-api/internal/platform/transaction"
)

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Dependencies struct {
	Pool  *pgxpool.Pool
	Admin echo.MiddlewareFunc
	Auth  echo.MiddlewareFunc
	Users UserResolver
}

type Module struct {
	list         *query.ListTypesHandler
	create       *command.CreateTypeHandler
	update       *command.UpdateTypeHandler
	createReview *command.CreateReviewHandler
	updateReview *command.UpdateReviewHandler
	getReview    *query.GetReviewHandler
	listReviews  *query.ListReviewsHandler
	voteReview   *command.VoteReviewHandler
	viewReview   *command.ViewReviewHandler
	types        *read.Repository
	users        UserResolver
	admin        echo.MiddlewareFunc
	auth         echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	typeReader := read.NewRepository(deps.Pool)
	typeWriter := write.NewRepository(deps.Pool)
	reviewWriter := write.NewReviewRepository(deps.Pool)
	reviewReader := read.NewReviewRepository(deps.Pool)
	tx := transaction.NewPgxManager(deps.Pool)
	pub := outboxpub.New(outbox.NewStore(deps.Pool))
	return &Module{
		list:         query.NewListTypesHandler(typeReader),
		create:       command.NewCreateTypeHandler(typeWriter),
		update:       command.NewUpdateTypeHandler(typeWriter),
		createReview: command.NewCreateReviewHandler(reviewWriter, tx, pub),
		updateReview: command.NewUpdateReviewHandler(reviewWriter, tx, pub),
		getReview:    query.NewGetReviewHandler(reviewReader),
		listReviews:  query.NewListReviewsHandler(reviewReader),
		voteReview:   command.NewVoteReviewHandler(reviewWriter, tx, pub),
		viewReview:   command.NewViewReviewHandler(reviewWriter, tx, pub),
		types:        typeReader,
		users:        deps.Users,
		admin:        deps.Admin,
		auth:         deps.Auth,
	}
}

func (m *Module) RequireActiveTypes(ctx context.Context, codes []string) error {
	if m == nil || m.types == nil {
		return contract.FieldError{Field: "type"}
	}
	return m.types.RequireActive(ctx, codes)
}

func (m *Module) statusCatalog() reviewshttp.StatusCatalog {
	return reviewshttp.StatusCatalog{
		Publication: statusItems(PublicationStatuses()),
		Release:     statusItems(ReleaseStatuses()),
	}
}

func statusItems(items []Status) []reviewshttp.StatusItem {
	out := make([]reviewshttp.StatusItem, 0, len(items))
	for _, item := range items {
		out = append(out, reviewshttp.StatusItem{Code: item.Code, Label: item.Label})
	}
	return out
}

func (m *Module) Name() string { return "reviews" }

func (m *Module) RegisterHTTP(g *echo.Group) {
	reviewshttp.Register(g, reviewshttp.NewHandler(reviewshttp.HandlerDeps{
		ListTypes:    m.list,
		CreateType:   m.create,
		UpdateType:   m.update,
		Statuses:     m.statusCatalog(),
		CreateReview: m.createReview,
		UpdateReview: m.updateReview,
		GetReview:    m.getReview,
		ListReviews:  m.listReviews,
		VoteReview:   m.voteReview,
		ViewReview:   m.viewReview,
		Users:        m.users,
	}), m.admin, m.auth)
}
