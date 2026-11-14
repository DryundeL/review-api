package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRenderBodyPreservesUnicodeWhenTruncated(t *testing.T) {
	got := RenderBody("Ответ", "@user", strings.Repeat("🙂", 256))
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) > 512 {
		t.Fatalf("invalid notification text: bytes=%d runes=%d", len(got), utf8.RuneCountInString(got))
	}
}

func TestRenderBody(t *testing.T) {
	t.Parallel()
	got := RenderBody("Новый лайк на ваш обзор", "@ann", "Neon")
	want := "Новый лайк на ваш обзор: @ann · «Neon»"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = RenderBody("Новый подписчик", "@bob", "")
	want = "Новый подписчик: @bob"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
