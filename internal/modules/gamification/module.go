package gamification

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/command"
	"review-api/internal/modules/gamification/internal/application/consumer"
	"review-api/internal/modules/gamification/internal/application/query"
	gamificationhttp "review-api/internal/modules/gamification/internal/delivery/http"
	outboxpub "review-api/internal/modules/gamification/internal/infrastructure/outbox"
	"review-api/internal/modules/gamification/internal/infrastructure/read"
	"review-api/internal/modules/gamification/internal/infrastructure/write"
	reviewscontract "review-api/internal/modules/reviews/contract"
	socialcontract "review-api/internal/modules/social/contract"
	"review-api/internal/platform/eventbus"
	"review-api/internal/platform/outbox"
	"review-api/internal/platform/transaction"
)

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type ReviewLookup interface {
	PublishedReviewRef(ctx context.Context, reviewID uuid.UUID) (reviewscontract.ReviewRef, error)
}

type PointsConfig struct {
	ReviewPerBlock int
	LikeGiven      int
	LikeReceived   int
	Follower       int
}

type Dependencies struct {
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	Admin   echo.MiddlewareFunc
	Auth    echo.MiddlewareFunc
	Users   UserResolver
	Reviews ReviewLookup
	Points  PointsConfig
}

type Module struct {
	list              *query.ListShopItemsHandler
	create            *command.CreateShopItemHandler
	update            *command.UpdateShopItemHandler
	me                *query.GetMeHandler
	purchase          *command.PurchaseHandler
	equip             *command.EquipHandler
	cosmetics         *query.CosmeticsHandler
	listAchievements  *query.ListAchievementsHandler
	listProgress      *query.ListProgressHandler
	createAchievement *command.CreateAchievementHandler
	updateAchievement *command.UpdateAchievementHandler
	published         *consumer.PublishedHandler
	voted             *consumer.VotedHandler
	followed          *consumer.FollowedHandler
	users             UserResolver
	admin             echo.MiddlewareFunc
	auth              echo.MiddlewareFunc
}

func New(deps Dependencies) *Module {
	writer := write.NewRepository(deps.Pool)
	reader := read.NewRepository(deps.Pool)
	catalogCache := read.NewShopCatalogCache(deps.Redis)
	achievementsCache := read.NewAchievementsCatalogCache(deps.Redis)
	listReader := read.NewCachedShopReader(reader, catalogCache)
	achievementsReader := read.NewCachedAchievementReader(reader, achievementsCache)
	tx := transaction.NewPgxManager(deps.Pool)
	pub := outboxpub.New(outbox.NewStore(deps.Pool))
	grant := command.NewGrantPointsHandler(writer, tx)
	purchase := command.NewPurchaseHandler(writer, tx)
	advance := command.NewAdvanceMetricHandler(writer, writer, pub, tx)
	grant.SetAdvance(advance)
	purchase.SetAdvance(advance)
	points := consumer.PointsConfig{
		ReviewPerBlock: deps.Points.ReviewPerBlock,
		LikeGiven:      deps.Points.LikeGiven,
		LikeReceived:   deps.Points.LikeReceived,
		Follower:       deps.Points.Follower,
	}
	return &Module{
		list:              query.NewListShopItemsHandler(listReader),
		create:            command.NewCreateShopItemHandler(writer, catalogCache),
		update:            command.NewUpdateShopItemHandler(writer, catalogCache),
		me:                query.NewGetMeHandler(reader),
		purchase:          purchase,
		equip:             command.NewEquipHandler(writer, tx),
		cosmetics:         query.NewCosmeticsHandler(reader),
		listAchievements:  query.NewListAchievementsHandler(achievementsReader),
		listProgress:      query.NewListProgressHandler(reader),
		createAchievement: command.NewCreateAchievementHandler(writer, achievementsCache),
		updateAchievement: command.NewUpdateAchievementHandler(writer, achievementsCache),
		published:         consumer.NewPublishedHandler(grant, advance, points),
		voted:             consumer.NewVotedHandler(grant, advance, deps.Reviews, points),
		followed:          consumer.NewFollowedHandler(grant, advance, points),
		users:             deps.Users,
		admin:             deps.Admin,
		auth:              deps.Auth,
	}
}

func (m *Module) Name() string { return "gamification" }

func (m *Module) Cosmetics(ctx context.Context, userID uuid.UUID) (contract.Cosmetics, error) {
	if m == nil || m.cosmetics == nil {
		return contract.Cosmetics{}, nil
	}
	return m.cosmetics.Handle(ctx, userID)
}

func (m *Module) RegisterHTTP(g *echo.Group, profileAccess echo.MiddlewareFunc) {
	gamificationhttp.Register(g, gamificationhttp.NewHandler(gamificationhttp.HandlerDeps{
		List: m.list, Create: m.create, Update: m.update,
		Me: m.me, Purchase: m.purchase, Equip: m.equip, Users: m.users,
		ListAchievements: m.listAchievements, ListProgress: m.listProgress,
		CreateAchievement: m.createAchievement, UpdateAchievement: m.updateAchievement,
	}), m.admin, m.auth, profileAccess)
}

func (m *Module) RegisterConsumers(d *eventbus.Dispatcher) {
	if m == nil || d == nil {
		return
	}
	if m.published != nil {
		d.Subscribe(reviewscontract.EventPublished, m.published.Handle)
	}
	if m.voted != nil {
		d.Subscribe(reviewscontract.EventVoted, m.voted.Handle)
	}
	if m.followed != nil {
		d.Subscribe(socialcontract.EventFollowed, m.followed.Handle)
	}
}
