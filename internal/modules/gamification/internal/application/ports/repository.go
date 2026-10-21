package ports

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/internal/domain"
)

type ShopItem struct {
	ID        uuid.UUID
	Code      string
	Kind      string
	Title     string
	Price     int
	IsActive  bool
	SortOrder int
}

type Profile struct {
	UserID              uuid.UUID
	PointsBalance       int
	EquippedBackground  string
	EquippedAvatarFrame string
	EquippedBackdrop    string
}

type Me struct {
	Profile
	OwnedCodes []string
}

type ShopWriter interface {
	CreateItem(ctx context.Context, item domain.ShopItem) error
	UpdateItem(ctx context.Context, id uuid.UUID, title *string, price *int, active *bool, sortOrder *int) (ShopItem, error)
	GetItem(ctx context.Context, id uuid.UUID) (ShopItem, error)
	NextSort(ctx context.Context) (int, error)
}

type ShopReader interface {
	ListItems(ctx context.Context, activeOnly bool) ([]ShopItem, error)
}

// CatalogInvalidator drops the cached public shop list after admin writes.
type CatalogInvalidator interface {
	Invalidate(ctx context.Context) error
}

type WalletWriter interface {
	EnsureProfile(ctx context.Context, userID uuid.UUID) error
	GrantPoints(ctx context.Context, userID uuid.UUID, amount int, reason, sourceKey string) (bool, error)
	Purchase(ctx context.Context, userID, itemID uuid.UUID) error
	Equip(ctx context.Context, userID uuid.UUID, background, avatarFrame, backdrop *string) (Profile, error)
}

type WalletReader interface {
	GetMe(ctx context.Context, userID uuid.UUID) (Me, error)
	GetCosmetics(ctx context.Context, userID uuid.UUID) (Profile, error)
}

type AchievementItem struct {
	ID           uuid.UUID
	Code         string
	Title        string
	Description  string
	Metric       string
	Target       int
	RewardPoints int
	IsActive     bool
	SortOrder    int
}

type AchievementProgress struct {
	AchievementItem
	Current    int
	UnlockedAt *string // RFC3339 UTC, nil if locked
}

type UnlockCandidate struct {
	ID           uuid.UUID
	Code         string
	RewardPoints int
}

type AchievementWriter interface {
	CreateAchievement(ctx context.Context, item domain.Achievement) error
	UpdateAchievement(ctx context.Context, id uuid.UUID, title, description *string, target, reward, sortOrder *int, active *bool) (AchievementItem, error)
	NextAchievementSort(ctx context.Context) (int, error)
	// RecordMetricEvent inserts the idempotency row. applied=false on duplicate source_key.
	RecordMetricEvent(ctx context.Context, userID uuid.UUID, metric string, delta int, sourceKey string) (applied bool, err error)
	BumpMetric(ctx context.Context, userID uuid.UUID, metric string, delta int) (newValue int, err error)
	ListUnlockable(ctx context.Context, userID uuid.UUID, metric string, value int) ([]UnlockCandidate, error)
	UnlockAchievement(ctx context.Context, userID, achievementID uuid.UUID) (inserted bool, err error)
}

type AchievementReader interface {
	ListAchievements(ctx context.Context, activeOnly bool) ([]AchievementItem, error)
	ListProgress(ctx context.Context, userID uuid.UUID) ([]AchievementProgress, error)
}

type Publisher interface {
	Publish(ctx context.Context, eventName, aggregateID string, payload any) error
}
