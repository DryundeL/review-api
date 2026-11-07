package contract

import "regexp"

// Cosmetics is the public equipped cosmetic image URLs for a user profile.
// Empty strings mean frontend defaults.
type Cosmetics struct {
	Background  string
	AvatarFrame string
	Backdrop    string
}

var cosmeticCodeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// CosmeticURL returns the relative image URL, or empty for an unset or invalid code.
func CosmeticURL(code string) string {
	if !cosmeticCodeRe.MatchString(code) {
		return ""
	}
	return "/storage/gemification/" + code + ".webp"
}
