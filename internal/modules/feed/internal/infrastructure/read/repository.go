package read

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/feed/contract"
	"review-api/internal/modules/feed/internal/application/ports"
)

// spoilersExistSQL reads only the published body, never the working version.
const spoilersExistSQL = `(r.body ~ '\|\|.*\|\|')`

// Repository reads the published feed via keyset pagination: reviews and public list results,
// merged by one sort key (counter, sort time, id).
// ponytail: direct read of reviews/lists/follows; materialize when write volume or following fan-out hurts
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// branch is the fixed SQL of one item kind. Only these fragments and numbered placeholders are
// concatenated; user input always travels as arguments. Columns of both selects are in scanItem order.
type branch struct {
	selectSQL string // SELECT list and FROM, the viewer placeholder written as {viewer}
	where     string // visibility filter of the kind
	author    string
	typeCol   string
	mixed     string // filter for type=mixed
	sortT     string
	views     string
	likes     string
	id        string
}

var (
	reviewBranch = branch{
		selectSQL: `
		SELECT 'review'::text AS kind, r.id AS item_id, r.author_id, COALESCE(u.username, ''), COALESCE(u.photo_url, ''),
			` + isFollowingSQL("r.author_id") + `,
			COALESCE(gp.equipped_avatar_frame, ''), COALESCE(gp.equipped_backdrop, ''),
			mo.id, mo.title, mo.type::text, mo.poster_url,
			r.title, r.rating, ` + spoilersExistSQL + `,
			r.likes_count::bigint,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND NOT v.is_like),
			r.views_count::bigint,
			(SELECT v.is_like FROM review_votes v WHERE v.review_id = r.id AND v.user_id = {viewer}),
			r.created_at AS sort_t,
			NULL::text, NULL::text, NULL::text, NULL::text,
			{sortv} AS sort_v
		FROM reviews r
		INNER JOIN media_objects mo ON mo.id = r.media_object_id
		INNER JOIN users u ON u.id = r.author_id
		LEFT JOIN gamification_profiles gp ON gp.user_id = r.author_id`,
		where:   `r.status = 'published'`,
		author:  "r.author_id",
		typeCol: "mo.type",
		mixed:   "FALSE",
		sortT:   "r.created_at",
		views:   "r.views_count",
		likes:   "r.likes_count",
		id:      "r.id",
	}
	// Cover: the first entry in canonical order (tier order, then position), as in the lists module.
	listBranch = branch{
		selectSQL: `
		SELECT 'list'::text AS kind, lr.id AS item_id, lr.author_id, COALESCE(u.username, ''), COALESCE(u.photo_url, ''),
			` + isFollowingSQL("lr.author_id") + `,
			COALESCE(gp.equipped_avatar_frame, ''), COALESCE(gp.equipped_backdrop, ''),
			NULL::uuid, NULL::text, NULL::text, NULL::text,
			lr.title, NULL::smallint, false,
			lr.likes_count::bigint, lr.dislikes_count::bigint, lr.views_count::bigint,
			(SELECT v.is_like FROM list_result_votes v WHERE v.result_id = lr.id AND v.user_id = {viewer}),
			lr.first_published_at AS sort_t,
			COALESCE(lv.type_code, 'mixed'), lv.format,
			COALESCE((SELECT vi.image_url
				FROM list_result_entries e
				JOIN list_template_version_items vi ON vi.version_id = e.version_id AND vi.item_id = e.item_id
				LEFT JOIN list_template_version_tiers vt ON vt.version_id = e.version_id AND vt.tier_id = e.tier_id
				WHERE e.result_id = lr.id
				ORDER BY vt.position NULLS FIRST, e.position LIMIT 1), ''),
			lv.title,
			{sortv} AS sort_v
		FROM list_results lr
		INNER JOIN list_template_versions lv ON lv.id = lr.version_id
		INNER JOIN list_templates lt ON lt.id = lr.template_id
		INNER JOIN users u ON u.id = lr.author_id
		LEFT JOIN gamification_profiles gp ON gp.user_id = lr.author_id`,
		// Same rule as the "Топы пользователей" tab; matches the list_results_*_idx predicates.
		where: `lr.status = 'published' AND lr.visibility = 'public' AND lr.deleted_at IS NULL
			AND lr.moderation_hidden_at IS NULL AND lt.moderation_hidden_at IS NULL`,
		author:  "lr.author_id",
		typeCol: "lv.type_code",
		mixed:   "lv.type_code IS NULL",
		sortT:   "lr.first_published_at",
		views:   "lr.views_count",
		likes:   "lr.likes_count",
		id:      "lr.id",
	}
)

func isFollowingSQL(authorCol string) string {
	return `({viewer} <> '00000000-0000-0000-0000-000000000000'::uuid
				AND EXISTS (SELECT 1 FROM follows f WHERE f.follower_id = {viewer} AND f.followee_id = ` + authorCol + `))`
}

func (r *Repository) List(ctx context.Context, p ports.ListParams) ([]ports.Item, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	sql, args, err := buildList(p)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list feed: %w", err)
	}
	defer rows.Close()

	out := make([]ports.Item, 0, p.Limit)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan feed item: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list feed: %w", err)
	}
	return out, nil
}

// buildList selects up to Limit rows from each included kind with the same keyset, then merges
// them. Each branch keeps its own index-backed ORDER BY ... LIMIT, so a page reads at most
// 2*Limit rows whatever the mix.
func buildList(p ports.ListParams) (string, []any, error) {
	if p.Limit <= 0 {
		return "", nil, contract.ErrInvalid
	}
	if p.Mode == ports.ModeFollowing && p.ViewerID == uuid.Nil {
		return "", nil, contract.ErrUnauthorized
	}
	var branches []branch
	switch p.Kind {
	case "", ports.KindAll:
		branches = []branch{reviewBranch, listBranch}
	case ports.KindReviews:
		branches = []branch{reviewBranch}
	case ports.KindLists:
		branches = []branch{listBranch}
	default:
		return "", nil, contract.FieldError{Field: "kind"}
	}

	args := make([]any, 0, 8)
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	viewer := arg(p.ViewerID)
	limit := arg(p.Limit)
	var mediaType, curV, curT, curID string
	if p.Type != "" && p.Type != ports.TypeMixed {
		mediaType = arg(p.Type)
	}
	if p.Cursor != nil {
		curV, curT, curID = arg(p.Cursor.V), arg(p.Cursor.T), arg(p.Cursor.ID)
	}

	parts := make([]string, 0, len(branches))
	for _, br := range branches {
		var sortV string
		switch p.Sort {
		case ports.SortViews:
			sortV = br.views
		case ports.SortLikes:
			sortV = br.likes
		}
		var b strings.Builder
		b.WriteString("(")
		sel := strings.ReplaceAll(br.selectSQL, "{viewer}", viewer)
		if sortV == "" {
			sel = strings.ReplaceAll(sel, "{sortv}", "0::bigint")
		} else {
			sel = strings.ReplaceAll(sel, "{sortv}", sortV+"::bigint")
		}
		b.WriteString(sel)
		b.WriteString("\n\t\tWHERE ")
		b.WriteString(br.where)
		if p.Mode == ports.ModeFollowing {
			b.WriteString(` AND EXISTS (SELECT 1 FROM follows f WHERE f.follower_id = ` + viewer +
				` AND f.followee_id = ` + br.author + `)`)
		}
		if p.Type == ports.TypeMixed {
			b.WriteString(" AND " + br.mixed)
		} else if mediaType != "" {
			b.WriteString(" AND " + br.typeCol + " = " + mediaType)
		}
		if p.Cursor != nil {
			if sortV == "" {
				b.WriteString(" AND (" + br.sortT + ", " + br.id + ") < (" + curT + ", " + curID + ")")
			} else {
				b.WriteString(" AND (" + sortV + ", " + br.sortT + ", " + br.id + ") < (" + curV + ", " + curT + ", " + curID + ")")
			}
		}
		b.WriteString("\n\t\tORDER BY ")
		if sortV != "" {
			b.WriteString(sortV + " DESC, ")
		}
		b.WriteString(br.sortT + " DESC, " + br.id + " DESC LIMIT " + limit + ")")
		parts = append(parts, b.String())
	}

	// sort_v is the counter of the active sort, 0 for "new".
	sql := "SELECT * FROM (" + strings.Join(parts, "\nUNION ALL\n") +
		") feed\nORDER BY sort_v DESC, sort_t DESC, item_id DESC LIMIT " + limit
	return sql, args, nil
}

func scanItem(row interface {
	Scan(dest ...any) error
}) (ports.Item, error) {
	var item ports.Item
	var (
		moID                         *uuid.UUID
		moTitle, moType, poster      *string
		rating                       *int16
		typeCode, format, cover, tpl *string
		sortV                        int64
	)
	err := row.Scan(
		&item.Kind, &item.ID, &item.AuthorID, &item.Author.Username, &item.Author.PhotoURL, &item.Author.IsFollowing,
		&item.Author.AvatarFrame, &item.Author.Backdrop,
		&moID, &moTitle, &moType, &poster,
		&item.Title, &rating,
		&item.HasSpoilers,
		&item.LikesCount, &item.DislikesCount, &item.ViewsCount,
		&item.ViewerIsLike,
		&item.CreatedAt,
		&typeCode, &format, &cover, &tpl,
		&sortV,
	)
	if err != nil {
		return ports.Item{}, err
	}
	item.Author.ID = item.AuthorID
	if moID != nil {
		item.MediaObject = ports.MediaObjectRef{ID: *moID, Title: deref(moTitle), Type: deref(moType), PosterURL: deref(poster)}
	}
	if rating != nil {
		item.Rating = *rating
	}
	item.List = ports.ListRef{TypeCode: deref(typeCode), Format: deref(format), CoverURL: deref(cover), TemplateTitle: deref(tpl)}
	return item, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
