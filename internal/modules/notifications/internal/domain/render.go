package domain

import (
	"strings"
	"unicode/utf8"
)

// RenderBody appends dynamic actor/review details to the type template text.
func RenderBody(baseText, actorLabel, reviewTitle string) string {
	baseText = strings.TrimSpace(baseText)
	actorLabel = strings.TrimSpace(actorLabel)
	reviewTitle = strings.TrimSpace(reviewTitle)

	var b strings.Builder
	b.WriteString(baseText)
	if actorLabel == "" && reviewTitle == "" {
		return b.String()
	}
	b.WriteString(": ")
	if actorLabel != "" {
		b.WriteString(actorLabel)
	}
	if reviewTitle != "" {
		if actorLabel != "" {
			b.WriteString(" · ")
		}
		b.WriteString("«")
		b.WriteString(reviewTitle)
		b.WriteString("»")
	}
	out := b.String()
	if len(out) > 512 {
		end := 512
		for !utf8.RuneStart(out[end]) {
			end--
		}
		return out[:end]
	}
	return out
}
