package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
)

var now = time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

func manual(title string) Item {
	img := uuid.New()
	return Item{ID: uuid.New(), Title: title, ImageID: &img, ImageURL: "/storage/images/" + img.String() + ".png"}
}

func newDraft(t *testing.T, format Format) Template {
	t.Helper()
	tpl, err := NewTemplate(uuid.New(), uuid.New(), "game", format, "Игры 2026", now)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func wantField(t *testing.T, err error, field, reason string) {
	t.Helper()
	var fe contract.FieldError
	if !errors.As(err, &fe) || fe.Field != field || fe.Reason != reason {
		t.Fatalf("err = %#v, want %s/%s", err, field, reason)
	}
}

func TestNewTemplateDefaults(t *testing.T) {
	t.Parallel()
	f, err := ParseFormat("")
	if err != nil || f != FormatTiers {
		t.Fatalf("default format = %q, %v", f, err)
	}
	tpl := newDraft(t, f)
	if tpl.Status != StatusDraft || tpl.Visibility != VisibilityPublic || tpl.Revision != 1 {
		t.Fatalf("template = %+v", tpl)
	}
	if len(tpl.Tiers) != 5 || tpl.Tiers[0].Name != "S" || tpl.Tiers[4].Name != "D" {
		t.Fatalf("tiers = %+v", tpl.Tiers)
	}
	if ranked := newDraft(t, FormatRanked); len(ranked.Tiers) != 0 {
		t.Fatal("ranked template must not have tiers")
	}
	if _, err := ParseFormat("pyramid"); err == nil {
		t.Fatal("unknown format accepted")
	}
	if _, err := NewTemplate(uuid.New(), uuid.New(), " ", FormatTiers, "", now); err == nil {
		t.Fatal("empty type accepted")
	}
}

func TestTemplateTextLimits(t *testing.T) {
	t.Parallel()
	tpl := newDraft(t, FormatCollection)
	emoji := strings.Repeat("🎮", MaxTitleLen) // 150 code points, 600 bytes
	if err := tpl.Replace(TemplateContent{TypeCode: "game", Format: FormatCollection, Title: "  " + emoji + "  "}); err != nil {
		t.Fatalf("150 emoji: %v", err)
	}
	if tpl.Title != emoji {
		t.Fatal("title must be trimmed")
	}
	cases := []struct {
		name    string
		content TemplateContent
		field   string
		reason  string
	}{
		{"title 151", TemplateContent{Title: strings.Repeat("я", 151)}, "title", contract.ReasonTooLong},
		{"title control", TemplateContent{Title: "a\x00b"}, "title", contract.ReasonInvalid},
		{"title newline", TemplateContent{Title: "a\nb"}, "title", contract.ReasonInvalid},
		{"description 2001", TemplateContent{Description: strings.Repeat("d", 2001)}, "description", contract.ReasonTooLong},
		{"item title 151", TemplateContent{Items: []Item{manual(strings.Repeat("x", 151))}}, "items.title", contract.ReasonTooLong},
		{"51 items", TemplateContent{Items: make51()}, "items", contract.ReasonTooMany},
	}
	for _, tc := range cases {
		tc.content.TypeCode, tc.content.Format = "game", FormatCollection
		wantField(t, tpl.Replace(tc.content), tc.field, tc.reason)
	}
	if err := tpl.Replace(TemplateContent{TypeCode: "game", Format: FormatCollection, Description: "line1\nline2\ttab"}); err != nil {
		t.Fatalf("multiline description: %v", err)
	}
	if err := tpl.Replace(TemplateContent{TypeCode: "game", Format: FormatCollection, Items: make51()[:50]}); err != nil {
		t.Fatalf("50 items: %v", err)
	}
}

func make51() []Item {
	out := make([]Item, 51)
	for i := range out {
		out[i] = manual("item")
	}
	return out
}

func TestTemplateTiers(t *testing.T) {
	t.Parallel()
	tpl := newDraft(t, FormatTiers)
	base := TemplateContent{TypeCode: "game", Format: FormatTiers}
	tiers := func(n int) []Tier {
		out := make([]Tier, n)
		for i := range out {
			out[i] = Tier{ID: uuid.New(), Name: "T", Color: "#AABBCC"}
		}
		return out
	}
	for _, n := range []int{1, 10} {
		c := base
		c.Tiers = tiers(n)
		if err := tpl.Replace(c); err != nil {
			t.Fatalf("%d tiers: %v", n, err)
		}
	}
	if tpl.Tiers[0].Color != "#aabbcc" {
		t.Fatal("color must be normalized to lower case")
	}
	c := base
	wantField(t, tpl.Replace(c), "tiers", contract.ReasonRequired)
	c.Tiers = tiers(11)
	wantField(t, tpl.Replace(c), "tiers", contract.ReasonTooMany)
	c.Tiers = tiers(2)
	c.Tiers[1].ID = c.Tiers[0].ID
	wantField(t, tpl.Replace(c), "tiers.id", contract.ReasonDuplicate)
	c.Tiers = tiers(1)
	c.Tiers[0].Name = strings.Repeat("т", 31)
	wantField(t, tpl.Replace(c), "tiers.name", contract.ReasonTooLong)
	for _, color := range []string{"red", "#abc", "#gggggg", "url(x)"} {
		c.Tiers = tiers(1)
		c.Tiers[0].Color = color
		wantField(t, tpl.Replace(c), "tiers.color", contract.ReasonInvalid)
	}
	ranked := TemplateContent{TypeCode: "game", Format: FormatRanked, Tiers: tiers(1)}
	wantField(t, tpl.Replace(ranked), "tiers", contract.ReasonInvalid)
}

func TestTemplateItems(t *testing.T) {
	t.Parallel()
	tpl := newDraft(t, FormatCollection)
	work := uuid.New()
	a, b := manual("Same"), manual("Same")
	catalogA := Item{ID: uuid.New(), Title: "Halo", MediaObjectID: &work, ImageURL: "https://cdn.example/halo.jpg"}
	catalogB := Item{ID: uuid.New(), Title: "Halo", MediaObjectID: &work, ImageURL: "https://cdn.example/halo.jpg"}
	content := func(items ...Item) TemplateContent {
		return TemplateContent{TypeCode: "game", Format: FormatCollection, Items: items}
	}
	if err := tpl.Replace(content(a, b, catalogA)); err != nil {
		t.Fatalf("manual items with equal titles must be allowed: %v", err)
	}
	wantField(t, tpl.Replace(content(catalogA, catalogB)), "items.mediaObjectId", contract.ReasonDuplicate)
	dupID := b
	dupID.ID = a.ID
	wantField(t, tpl.Replace(content(a, dupID)), "items.id", contract.ReasonDuplicate)
	noURL := a
	noURL.ImageURL = ""
	wantField(t, tpl.Replace(content(noURL)), "items.imageId", contract.ReasonInvalid)
}

func TestTemplatePublishRules(t *testing.T) {
	t.Parallel()
	tpl := newDraft(t, FormatTiers)
	wantField(t, tpl.Publish(VisibilityPublic, now), "items", contract.ReasonRequired)

	noImage := Item{ID: uuid.New(), Title: "No poster", MediaObjectID: ptr(uuid.New())}
	c := TemplateContent{TypeCode: "game", Format: FormatTiers, Title: "Игры", Tiers: tpl.Tiers, Items: []Item{noImage}}
	if err := tpl.Replace(c); err != nil {
		t.Fatalf("draft may contain incomplete items: %v", err)
	}
	wantField(t, tpl.Publish(VisibilityPublic, now), "items.image", contract.ReasonRequired)

	c.Items = []Item{manual("Halo")}
	c.Tiers = append([]Tier(nil), tpl.Tiers...)
	c.Tiers[0].Name = " "
	if err := tpl.Replace(c); err != nil {
		t.Fatal(err)
	}
	wantField(t, tpl.Publish(VisibilityPublic, now), "tiers.name", contract.ReasonRequired)

	c.Tiers[0].Name = "S"
	c.Title = ""
	_ = tpl.Replace(c)
	wantField(t, tpl.Publish(VisibilityPublic, now), "title", contract.ReasonRequired)

	c.Title = "Игры"
	if err := tpl.Replace(c); err != nil {
		t.Fatal(err)
	}
	if err := tpl.Publish(VisibilityUnlisted, now); err != nil {
		t.Fatal(err)
	}
	if tpl.Status != StatusPublished || tpl.Visibility != VisibilityUnlisted || !tpl.FirstPublishedAt.Equal(now) {
		t.Fatalf("template = %+v", tpl)
	}
	if tpl.CoverURL() != c.Items[0].ImageURL {
		t.Fatal("cover must be the first item image")
	}

	// A published template cannot be saved into an unpublishable state.
	broken := c
	broken.Items = nil
	wantField(t, tpl.Replace(broken), "items", contract.ReasonRequired)

	// Type and format are locked after the first publication, even back in draft.
	tpl.Unpublish()
	moved := c
	moved.TypeCode = "movie"
	wantField(t, tpl.Replace(moved), "typeCode", contract.ReasonImmutable)
	moved = c
	moved.Format, moved.Tiers = FormatRanked, nil
	wantField(t, tpl.Replace(moved), "format", contract.ReasonImmutable)

	// Republishing keeps the first publication date.
	if err := tpl.Publish(VisibilityPublic, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if !tpl.FirstPublishedAt.Equal(now) {
		t.Fatal("first publication date changed")
	}
}

func TestTemplateVisibilityModerationCopy(t *testing.T) {
	t.Parallel()
	tpl := newDraft(t, FormatCollection)
	if err := tpl.SetVisibility(VisibilityUnlisted); !errors.Is(err, contract.ErrConflict) {
		t.Fatalf("draft visibility: %v", err)
	}
	_ = tpl.Replace(TemplateContent{TypeCode: "game", Format: FormatCollection, Title: "T", Items: []Item{manual("a")}})
	if err := tpl.Publish(VisibilityPublic, now); err != nil {
		t.Fatal(err)
	}
	if err := tpl.SetVisibility(VisibilityUnlisted); err != nil || tpl.Visibility != VisibilityUnlisted {
		t.Fatalf("visibility: %v", err)
	}
	if !tpl.AcceptsResults() {
		t.Fatal("unlisted template accepts results from its own page")
	}

	cp, err := tpl.Copy(uuid.New(), now)
	if err != nil {
		t.Fatal(err)
	}
	if cp.Status != StatusDraft || cp.FirstPublishedAt != nil || cp.Visibility != VisibilityPublic || len(cp.Items) != 1 || cp.ID == tpl.ID {
		t.Fatalf("copy = %+v", cp)
	}
	cp.Items[0].Title = "changed"
	if tpl.Items[0].Title == "changed" {
		t.Fatal("copy shares item storage with the original")
	}

	hidden := now
	tpl.ModerationHiddenAt = &hidden
	if tpl.AcceptsResults() {
		t.Fatal("moderated template must not accept results")
	}
	tpl.Unpublish()
	if err := tpl.Publish(VisibilityPublic, now); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("republish of moderated: %v", err)
	}
	if _, err := tpl.Copy(uuid.New(), now); !errors.Is(err, contract.ErrModerated) {
		t.Fatalf("copy of moderated: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }
