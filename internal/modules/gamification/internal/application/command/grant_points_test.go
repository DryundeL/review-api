package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/modules/gamification/internal/domain"
)

type fakeTx struct{}

func (fakeTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type fakeWallet struct {
	balance int
	grants  map[string]int
	owned   map[string]string // code -> kind
	items   map[uuid.UUID]ports.ShopItem
}

func (f *fakeWallet) EnsureProfile(context.Context, uuid.UUID) error { return nil }

func (f *fakeWallet) GrantPoints(_ context.Context, _ uuid.UUID, amount int, _, sourceKey string) (bool, error) {
	if f.grants == nil {
		f.grants = map[string]int{}
	}
	if _, ok := f.grants[sourceKey]; ok {
		return false, nil
	}
	f.grants[sourceKey] = amount
	f.balance += amount
	return true, nil
}

func (f *fakeWallet) Purchase(_ context.Context, _ uuid.UUID, itemID uuid.UUID) error {
	item, ok := f.items[itemID]
	if !ok || !item.IsActive {
		return contract.ErrNotFound
	}
	if f.balance < item.Price {
		return contract.ErrInsufficient
	}
	if f.owned == nil {
		f.owned = map[string]string{}
	}
	if _, ok := f.owned[item.Code]; ok {
		return contract.ErrConflict
	}
	f.balance -= item.Price
	f.owned[item.Code] = item.Kind
	return nil
}

func (f *fakeWallet) Equip(_ context.Context, userID uuid.UUID, background, avatarFrame, backdrop *string) (ports.Profile, error) {
	p := ports.Profile{UserID: userID, PointsBalance: f.balance}
	apply := func(code *string, kind string) (string, error) {
		if code == nil {
			return "", nil
		}
		if *code == "" {
			return "", nil
		}
		if f.owned[*code] != kind {
			return "", contract.ErrNotOwned
		}
		return *code, nil
	}
	var err error
	if background != nil {
		p.EquippedBackground, err = apply(background, domain.KindBackground)
		if err != nil {
			return ports.Profile{}, err
		}
	}
	if avatarFrame != nil {
		p.EquippedAvatarFrame, err = apply(avatarFrame, domain.KindAvatarFrame)
		if err != nil {
			return ports.Profile{}, err
		}
	}
	if backdrop != nil {
		p.EquippedBackdrop, err = apply(backdrop, domain.KindBackdrop)
		if err != nil {
			return ports.Profile{}, err
		}
	}
	return p, nil
}

func TestGrantPointsIdempotent(t *testing.T) {
	t.Parallel()
	w := &fakeWallet{}
	h := NewGrantPointsHandler(w, fakeTx{})
	uid := uuid.New()
	if err := h.Handle(context.Background(), uid, 10, "review_publish", "review_publish:1"); err != nil {
		t.Fatal(err)
	}
	if err := h.Handle(context.Background(), uid, 10, "review_publish", "review_publish:1"); err != nil {
		t.Fatal(err)
	}
	if w.balance != 10 {
		t.Fatalf("balance=%d", w.balance)
	}
}

func TestPurchaseInsufficientAndConflict(t *testing.T) {
	t.Parallel()
	itemID := uuid.New()
	w := &fakeWallet{
		balance: 5,
		items: map[uuid.UUID]ports.ShopItem{
			itemID: {ID: itemID, Code: "frame_gold", Kind: domain.KindAvatarFrame, Price: 10, IsActive: true},
		},
	}
	h := NewPurchaseHandler(w, fakeTx{})
	uid := uuid.New()
	if err := h.Handle(context.Background(), uid, itemID); !errors.Is(err, contract.ErrInsufficient) {
		t.Fatalf("err=%v", err)
	}
	w.balance = 100
	if err := h.Handle(context.Background(), uid, itemID); err != nil {
		t.Fatal(err)
	}
	if err := h.Handle(context.Background(), uid, itemID); !errors.Is(err, contract.ErrConflict) {
		t.Fatalf("err=%v", err)
	}
}

func TestEquipNotOwned(t *testing.T) {
	t.Parallel()
	w := &fakeWallet{owned: map[string]string{}}
	h := NewEquipHandler(w, fakeTx{})
	code := "frame_gold"
	_, err := h.Handle(context.Background(), uuid.New(), EquipInput{AvatarFrame: &code})
	if !errors.Is(err, contract.ErrNotOwned) {
		t.Fatalf("err=%v", err)
	}
}
