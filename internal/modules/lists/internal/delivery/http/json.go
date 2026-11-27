package listshttp

import (
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/application/query"
	"review-api/internal/modules/lists/internal/domain"
)

// moderationHidden is the only moderation state shown, and only to the author.
const moderationHidden = "hidden"

type authorJSON struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	PhotoURL string `json:"photoUrl,omitempty"`
}

type tierJSON struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type itemJSON struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	MediaObjectID *string `json:"mediaObjectId,omitempty"`
	ImageURL      string  `json:"imageUrl,omitempty"`
	ImageID       *string `json:"imageId,omitempty"` // author only
}

// ownerJSON holds the fields only the author receives: guests never see drafts or revisions.
type ownerJSON struct {
	Revision   int     `json:"revision,omitempty"`
	Status     string  `json:"status,omitempty"`
	Visibility string  `json:"visibility,omitempty"`
	Moderation string  `json:"moderation,omitempty"`
	CreatedAt  *string `json:"createdAt,omitempty"`
	UpdatedAt  *string `json:"updatedAt,omitempty"`
}

type templateJSON struct {
	ID               string     `json:"id"`
	TypeCode         string     `json:"typeCode"`
	Format           string     `json:"format"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	CoverURL         string     `json:"coverUrl,omitempty"`
	Author           authorJSON `json:"author"`
	FirstPublishedAt *string    `json:"firstPublishedAt,omitempty"`
	Tiers            []tierJSON `json:"tiers"`
	Items            []itemJSON `json:"items"`
	// CanCreateResult: the viewer may start a result on this template (guests after login).
	CanCreateResult bool `json:"canCreateResult"`
	ownerJSON
}

type resultTemplateJSON struct {
	// ID is set only when the template is published, public and not hidden: the link and
	// "make your own" are offered only then.
	ID     string      `json:"id,omitempty"`
	Title  string      `json:"title"`
	Author *authorJSON `json:"author,omitempty"`
}

type entryJSON struct {
	ItemID   string  `json:"itemId"`
	TierID   *string `json:"tierId,omitempty"`
	Position int     `json:"position"`
}

type resultJSON struct {
	ID               string             `json:"id"`
	TypeCode         string             `json:"typeCode"`
	Format           string             `json:"format"`
	Title            string             `json:"title"`
	Description      string             `json:"description"`
	CoverURL         string             `json:"coverUrl,omitempty"`
	Author           authorJSON         `json:"author"`
	FirstPublishedAt *string            `json:"firstPublishedAt,omitempty"`
	Template         resultTemplateJSON `json:"template"`
	CanCreateOwn     bool               `json:"canCreateOwn"`
	Tiers            []tierJSON         `json:"tiers"`
	Items            []itemJSON         `json:"items"`
	Entries          []entryJSON        `json:"entries"`
	ownerJSON
}

type cardJSON struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	TypeCode         string     `json:"typeCode"`
	Format           string     `json:"format"`
	CoverURL         string     `json:"coverUrl,omitempty"`
	Author           authorJSON `json:"author"`
	FirstPublishedAt *string    `json:"firstPublishedAt,omitempty"`
	TemplateTitle    string     `json:"templateTitle,omitempty"` // results only
	ownerJSON
}

type pageJSON struct {
	Items      []cardJSON `json:"items"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

type reportJSON struct {
	ID         string  `json:"id"`
	Target     string  `json:"target"`
	TargetID   string  `json:"targetId"`
	ReasonCode string  `json:"reasonCode"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"createdAt"`
	ResolvedAt *string `json:"resolvedAt,omitempty"`
}

type adminReportJSON struct {
	reportJSON
	ReporterID     string `json:"reporterId"`
	TargetTitle    string `json:"targetTitle"`
	TargetAuthorID string `json:"targetAuthorId"`
	TargetHidden   bool   `json:"targetHidden"`
	TargetDeleted  bool   `json:"targetDeleted"`
}

type reportPageJSON struct {
	Items      []adminReportJSON `json:"items"`
	NextCursor string            `json:"nextCursor,omitempty"`
}

func timeJSON(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func timePtrJSON(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := timeJSON(*t)
	return &s
}

func idPtrJSON(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func authorOf(a ports.AuthorRef) authorJSON {
	return authorJSON{ID: a.ID.String(), Username: a.Username, PhotoURL: a.PhotoURL}
}

func tiersJSON(in []domain.Tier) []tierJSON {
	out := make([]tierJSON, 0, len(in))
	for _, t := range in {
		out = append(out, tierJSON{ID: t.ID.String(), Name: t.Name, Color: t.Color})
	}
	return out
}

func itemsJSON(in []domain.Item, isOwner bool) []itemJSON {
	out := make([]itemJSON, 0, len(in))
	for _, it := range in {
		j := itemJSON{ID: it.ID.String(), Title: it.Title, MediaObjectID: idPtrJSON(it.MediaObjectID), ImageURL: it.ImageURL}
		if isOwner {
			j.ImageID = idPtrJSON(it.ImageID)
		}
		out = append(out, j)
	}
	return out
}

func owner(status domain.Status, visibility domain.Visibility, revision int, moderated bool, createdAt, updatedAt time.Time) ownerJSON {
	o := ownerJSON{Revision: revision, Status: string(status), Visibility: string(visibility)}
	if moderated {
		o.Moderation = moderationHidden
	}
	if !createdAt.IsZero() {
		o.CreatedAt = new(timeJSON(createdAt))
	}
	if !updatedAt.IsZero() {
		o.UpdatedAt = new(timeJSON(updatedAt))
	}
	return o
}

func templateResponse(v ports.TemplateView, isOwner bool) templateJSON {
	t := v.Template
	j := templateJSON{
		ID: t.ID.String(), TypeCode: t.TypeCode, Format: string(t.Format), Title: t.Title, Description: t.Description,
		CoverURL: t.CoverURL(), Author: authorOf(v.Author), FirstPublishedAt: timePtrJSON(t.FirstPublishedAt),
		Tiers: tiersJSON(t.Tiers), Items: itemsJSON(t.Items, isOwner),
		CanCreateResult: t.AcceptsResults(),
	}
	if isOwner {
		j.ownerJSON = owner(t.Status, t.Visibility, t.Revision, t.ModerationHiddenAt != nil, t.CreatedAt, v.UpdatedAt)
	}
	return j
}

func resultResponse(v ports.ResultView, isOwner bool) resultJSON {
	r := v.Result
	j := resultJSON{
		ID: r.ID.String(), TypeCode: v.TypeCode, Format: string(r.Format), Title: r.Title, Description: r.Description,
		CoverURL: r.CoverURL(v.Snapshot), Author: authorOf(v.Author), FirstPublishedAt: timePtrJSON(r.FirstPublishedAt),
		Template:     resultTemplateJSON{Title: v.Snapshot.Title},
		CanCreateOwn: v.TemplatePublic,
		Tiers:        tiersJSON(v.Snapshot.Tiers), Items: itemsJSON(v.Snapshot.Items, false),
		Entries: entriesJSON(r.Entries),
	}
	if v.TemplateAuthor.ID != uuid.Nil {
		j.Template.Author = new(authorOf(v.TemplateAuthor))
	}
	if v.TemplatePublic {
		j.Template.ID = r.TemplateID.String()
	}
	if isOwner {
		j.ownerJSON = owner(r.Status, r.Visibility, r.Revision, r.ModerationHiddenAt != nil || v.TemplateModerated, r.CreatedAt, v.UpdatedAt)
	}
	return j
}

func entriesJSON(in []domain.Entry) []entryJSON {
	out := make([]entryJSON, 0, len(in))
	for _, e := range in {
		out = append(out, entryJSON{ItemID: e.ItemID.String(), TierID: idPtrJSON(e.TierID), Position: e.Position})
	}
	return out
}

func pageResponse(p query.ListPage, mine bool) pageJSON {
	out := pageJSON{Items: make([]cardJSON, 0, len(p.Items)), NextCursor: p.NextCursor}
	for _, c := range p.Items {
		j := cardJSON{
			ID: c.ID.String(), Title: c.Title, TypeCode: c.TypeCode, Format: string(c.Format), CoverURL: c.CoverURL,
			Author: authorOf(c.Author), FirstPublishedAt: timePtrJSON(c.FirstPublishedAt), TemplateTitle: c.TemplateTitle,
		}
		if mine {
			j.ownerJSON = owner(c.Status, c.Visibility, 0, c.Moderated, time.Time{}, c.UpdatedAt)
		}
		out.Items = append(out.Items, j)
	}
	return out
}

func reportResponse(r ports.Report) reportJSON {
	return reportJSON{
		ID: r.ID.String(), Target: string(r.Target), TargetID: r.TargetID.String(), ReasonCode: r.ReasonCode,
		Status: string(r.Status), CreatedAt: timeJSON(r.CreatedAt), ResolvedAt: timePtrJSON(r.ResolvedAt),
	}
}

func reportPageResponse(p query.ReportPage) reportPageJSON {
	out := reportPageJSON{Items: make([]adminReportJSON, 0, len(p.Items)), NextCursor: p.NextCursor}
	for _, c := range p.Items {
		out.Items = append(out.Items, adminReportJSON{
			reportJSON: reportResponse(c.Report), ReporterID: c.ReporterID.String(), TargetTitle: c.TargetTitle,
			TargetAuthorID: c.TargetAuthorID.String(), TargetHidden: c.TargetHidden, TargetDeleted: c.TargetDeleted,
		})
	}
	return out
}
