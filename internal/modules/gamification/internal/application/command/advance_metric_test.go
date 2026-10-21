package command

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/modules/gamification/internal/domain"
)

type fakeAchievements struct {
	events   map[string]struct{}
	metrics  map[string]int
	unlocked map[uuid.UUID]struct{}
	catalog  []struct {
		ports.UnlockCandidate
		Metric string
		Target int
	}
}

func (f *fakeAchievements) CreateAchievement(context.Context, domain.Achievement) error {
	return nil
}
func (f *fakeAchievements) UpdateAchievement(context.Context, uuid.UUID, *string, *string, *int, *int, *int, *bool) (ports.AchievementItem, error) {
	return ports.AchievementItem{}, nil
}
func (f *fakeAchievements) NextAchievementSort(context.Context) (int, error) { return 10, nil }

func (f *fakeAchievements) RecordMetricEvent(_ context.Context, _ uuid.UUID, _ string, _ int, sourceKey string) (bool, error) {
	if f.events == nil {
		f.events = map[string]struct{}{}
	}
	if _, ok := f.events[sourceKey]; ok {
		return false, nil
	}
	f.events[sourceKey] = struct{}{}
	return true, nil
}

func (f *fakeAchievements) BumpMetric(_ context.Context, _ uuid.UUID, metric string, delta int) (int, error) {
	if f.metrics == nil {
		f.metrics = map[string]int{}
	}
	f.metrics[metric] += delta
	return f.metrics[metric], nil
}

func (f *fakeAchievements) ListUnlockable(_ context.Context, _ uuid.UUID, metric string, value int) ([]ports.UnlockCandidate, error) {
	out := make([]ports.UnlockCandidate, 0)
	for _, c := range f.catalog {
		if c.Metric != metric || value < c.Target {
			continue
		}
		if _, ok := f.unlocked[c.ID]; ok {
			continue
		}
		out = append(out, c.UnlockCandidate)
	}
	return out, nil
}

func (f *fakeAchievements) UnlockAchievement(_ context.Context, _ uuid.UUID, achievementID uuid.UUID) (bool, error) {
	if f.unlocked == nil {
		f.unlocked = map[uuid.UUID]struct{}{}
	}
	if _, ok := f.unlocked[achievementID]; ok {
		return false, nil
	}
	f.unlocked[achievementID] = struct{}{}
	return true, nil
}

type fakePublisher struct {
	events []string
}

func (p *fakePublisher) Publish(_ context.Context, eventName, _ string, _ any) error {
	p.events = append(p.events, eventName)
	return nil
}

func TestAdvanceMetricIdempotentAndUnlock(t *testing.T) {
	t.Parallel()
	achID := uuid.MustParse("a0000001-0001-4000-8000-000000000001")
	ach := &fakeAchievements{
		catalog: []struct {
			ports.UnlockCandidate
			Metric string
			Target int
		}{
			{UnlockCandidate: ports.UnlockCandidate{ID: achID, Code: "first_review", RewardPoints: 5}, Metric: "reviews_published", Target: 1},
		},
	}
	wallet := &fakeWallet{}
	pub := &fakePublisher{}
	h := NewAdvanceMetricHandler(ach, wallet, pub, fakeTx{})
	uid := uuid.New()

	applied, value, err := h.Handle(context.Background(), uid, "reviews_published", 1, "ach:pub:1")
	if err != nil {
		t.Fatal(err)
	}
	if !applied || value != 1 {
		t.Fatalf("applied=%v value=%d", applied, value)
	}
	if _, ok := ach.unlocked[achID]; !ok {
		t.Fatal("expected unlock")
	}
	if wallet.balance != 5 {
		t.Fatalf("balance=%d", wallet.balance)
	}
	if len(pub.events) != 1 || pub.events[0] != contract.EventUnlockGranted {
		t.Fatalf("events=%v", pub.events)
	}

	applied, value, err = h.Handle(context.Background(), uid, "reviews_published", 1, "ach:pub:1")
	if err != nil {
		t.Fatal(err)
	}
	if applied || value != 0 {
		t.Fatalf("replay applied=%v value=%d", applied, value)
	}
	if wallet.balance != 5 {
		t.Fatalf("balance after replay=%d", wallet.balance)
	}
}
