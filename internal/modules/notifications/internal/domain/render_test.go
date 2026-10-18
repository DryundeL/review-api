package domain

import "testing"

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
