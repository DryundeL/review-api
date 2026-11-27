package read

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
)

// listSpec holds the fixed SQL of one entity. Only these fragments and numbered
// placeholders are ever concatenated; user input always travels as arguments.
type listSpec struct {
	from      string // SELECT list and FROM clause, columns in scanCard order
	alias     string // alias of the listed table
	moderated string // true when the object (or its template) is hidden by a moderator
	typeCol   string
	formatCol string
}

// Template cover: the first item. Result cover: the first entry in canonical order
// (tier order, then position), the same rule as domain.Result.CoverURL.
var (
	templateList = listSpec{
		from: `
			SELECT t.id, t.title, COALESCE(t.type_code, 'mixed'), t.format, t.status, t.visibility,
				t.moderation_hidden_at IS NOT NULL, t.first_published_at, t.updated_at,
				t.author_id, COALESCE(u.username, ''), COALESCE(u.photo_url, ''),
				COALESCE((SELECT i.image_url FROM list_template_items i
					WHERE i.template_id = t.id ORDER BY i.position LIMIT 1), ''),
				'00000000-0000-0000-0000-000000000000'::uuid, ''
			FROM list_templates t
			JOIN users u ON u.id = t.author_id`,
		alias:     "t",
		moderated: "t.moderation_hidden_at IS NOT NULL",
		typeCol:   "t.type_code",
		formatCol: "t.format",
	}
	resultList = listSpec{
		from: `
			SELECT r.id, r.title, COALESCE(v.type_code, 'mixed'), v.format, r.status, r.visibility,
				(r.moderation_hidden_at IS NOT NULL OR t.moderation_hidden_at IS NOT NULL),
				r.first_published_at, r.updated_at,
				r.author_id, COALESCE(u.username, ''), COALESCE(u.photo_url, ''),
				COALESCE((SELECT vi.image_url
					FROM list_result_entries e
					JOIN list_template_version_items vi ON vi.version_id = e.version_id AND vi.item_id = e.item_id
					LEFT JOIN list_template_version_tiers vt ON vt.version_id = e.version_id AND vt.tier_id = e.tier_id
					WHERE e.result_id = r.id
					ORDER BY vt.position NULLS FIRST, e.position LIMIT 1), ''),
				r.template_id, v.title
			FROM list_results r
			JOIN list_template_versions v ON v.id = r.version_id
			JOIN list_templates t ON t.id = r.template_id
			JOIN users u ON u.id = r.author_id`,
		alias:     "r",
		moderated: "(r.moderation_hidden_at IS NOT NULL OR t.moderation_hidden_at IS NOT NULL)",
		typeCol:   "v.type_code",
		formatCol: "v.format",
	}
)

func (r *Repository) ListTemplates(ctx context.Context, q ports.ListQuery) ([]ports.ListCard, error) {
	return r.list(ctx, templateList, q)
}

func (r *Repository) ListResults(ctx context.Context, q ports.ListQuery) ([]ports.ListCard, error) {
	return r.list(ctx, resultList, q)
}

func (r *Repository) list(ctx context.Context, spec listSpec, q ports.ListQuery) ([]ports.ListCard, error) {
	if r == nil || r.pool == nil || q.Limit < 1 {
		return nil, contract.ErrInvalid
	}
	sql, args, err := buildList(spec, q)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", spec.alias, err)
	}
	cards, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ports.ListCard, error) {
		var c ports.ListCard
		err := row.Scan(&c.ID, &c.Title, &c.TypeCode, &c.Format, &c.Status, &c.Visibility, &c.Moderated,
			&c.FirstPublishedAt, &c.UpdatedAt, &c.Author.ID, &c.Author.Username, &c.Author.PhotoURL,
			&c.CoverURL, &c.TemplateID, &c.TemplateTitle)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", spec.alias, err)
	}
	return cards, nil
}

func buildList(spec listSpec, q ports.ListQuery) (string, []any, error) {
	a := spec.alias
	var (
		where []string
		args  []any
	)
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	where = append(where, a+".deleted_at IS NULL")
	sortCol := a + ".first_published_at"
	if q.Mine {
		if q.AuthorID == uuid.Nil {
			return "", nil, contract.ErrInvalid
		}
		sortCol = a + ".updated_at"
		where = append(where, a+".author_id = "+arg(q.AuthorID))
		switch q.Status {
		case ports.MineAll:
		case ports.MineDraft:
			where = append(where, a+".status = 'draft'", "NOT "+spec.moderated)
		case ports.MinePublished:
			where = append(where, a+".status = 'published'", a+".visibility = 'public'", "NOT "+spec.moderated)
		case ports.MineUnlisted:
			where = append(where, a+".status = 'published'", a+".visibility = 'unlisted'", "NOT "+spec.moderated)
		case ports.MineHidden:
			where = append(where, spec.moderated)
		default:
			return "", nil, contract.Field("status", contract.ReasonInvalid)
		}
	} else {
		// Matches the partial indexes *_public_idx and *_profile_idx.
		where = append(where, a+".status = 'published'", a+".visibility = 'public'", "NOT "+spec.moderated)
		if q.AuthorID != uuid.Nil {
			where = append(where, a+".author_id = "+arg(q.AuthorID))
		}
	}
	switch q.TypeCode {
	case "":
	case domain.TypeMixed:
		where = append(where, spec.typeCol+" IS NULL")
	default:
		where = append(where, spec.typeCol+" = "+arg(q.TypeCode))
	}
	if q.Format != "" {
		where = append(where, spec.formatCol+" = "+arg(string(q.Format)))
	}
	if q.Search != "" {
		where = append(where, a+".title ILIKE '%' || "+arg(escapeLike(q.Search))+" || '%' ESCAPE '\\'")
	}
	if q.After != nil {
		where = append(where, "("+sortCol+", "+a+".id) < ("+arg(q.After.T)+", "+arg(q.After.ID)+")")
	}
	sql := spec.from + "\nWHERE " + strings.Join(where, "\n\tAND ") +
		"\nORDER BY " + sortCol + " DESC, " + a + ".id DESC\nLIMIT " + arg(q.Limit)
	return sql, args, nil
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}
