package domain

import "strings"

// ClientLabel turns a User-Agent into a browser and device a person can read.
// Unknown values are the word "Unknown"; the bot translates them.
func ClientLabel(userAgent string) (browser, device string) {
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}
	ua := strings.TrimSpace(userAgent)
	if ua == "" {
		return "Unknown", "Unknown"
	}
	return detectBrowser(ua), detectDevice(ua)
}

func detectBrowser(ua string) string {
	switch {
	case strings.Contains(ua, "Edg/") || strings.Contains(ua, "EdgA/") || strings.Contains(ua, "EdgiOS/"):
		return "Edge"
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera/"):
		return "Opera"
	case strings.Contains(ua, "YaBrowser"):
		return "Yandex"
	case strings.Contains(ua, "Firefox/") || strings.Contains(ua, "FxiOS/"):
		return "Firefox"
	case strings.Contains(ua, "CriOS/"):
		return "Chrome"
	case strings.Contains(ua, "Chrome/"):
		return "Chrome"
	case strings.Contains(ua, "Safari/"):
		return "Safari"
	default:
		return "Unknown"
	}
}

func detectDevice(ua string) string {
	switch {
	case strings.Contains(ua, "iPhone"):
		return "iPhone"
	case strings.Contains(ua, "iPad"):
		return "iPad"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "Windows"):
		return "Windows"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		return "macOS"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	default:
		return "Unknown"
	}
}
