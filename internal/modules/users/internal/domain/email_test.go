package domain

import "testing"

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()

	got, err := NormalizeEmail("  User.Name+tag@Example.COM ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "user.name+tag@example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeEmailRejectsInvalid(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "display <user@example.com>", "a@@example.com", "user @example.com"} {
		if _, err := NormalizeEmail(value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
}
