package domain

import (
	"regexp"
	"strings"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
)

const (
	KindBackground  = "background"
	KindAvatarFrame = "avatar_frame"
	KindBackdrop    = "backdrop"
)

var codeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type ShopItem struct {
	ID        uuid.UUID
	Code      string
	Kind      string
	Title     string
	Price     int
	IsActive  bool
	SortOrder int
}

func NewShopItem(code, kind, title string, price, sortOrder int, active bool) (ShopItem, error) {
	code = strings.TrimSpace(strings.ToLower(code))
	kind = strings.TrimSpace(kind)
	title = strings.TrimSpace(title)
	if !ValidCode(code) {
		return ShopItem{}, contract.FieldError{Field: "code"}
	}
	if !ValidKind(kind) {
		return ShopItem{}, contract.FieldError{Field: "kind"}
	}
	if len(title) < 1 || len(title) > 64 {
		return ShopItem{}, contract.FieldError{Field: "title"}
	}
	if price < 1 {
		return ShopItem{}, contract.FieldError{Field: "price"}
	}
	if sortOrder < 0 || sortOrder > 10000 {
		return ShopItem{}, contract.FieldError{Field: "sortOrder"}
	}
	return ShopItem{
		ID:        uuid.New(),
		Code:      code,
		Kind:      kind,
		Title:     title,
		Price:     price,
		IsActive:  active,
		SortOrder: sortOrder,
	}, nil
}

func ValidCode(code string) bool {
	return codeRe.MatchString(code)
}

func ValidKind(kind string) bool {
	switch kind {
	case KindBackground, KindAvatarFrame, KindBackdrop:
		return true
	default:
		return false
	}
}

func KindForEquipField(field string) string {
	switch field {
	case "background":
		return KindBackground
	case "avatarFrame", "avatar_frame":
		return KindAvatarFrame
	case "backdrop":
		return KindBackdrop
	default:
		return ""
	}
}
