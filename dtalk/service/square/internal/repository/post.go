package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

type PostListFilter struct {
	Statuses        []int16
	AuthorAddrs     []string
	AuthorNames     []string
	AuthorNameExact []string
	Kind            *int16
	From            *time.Time
	To              *time.Time
	OnlyVisible     bool
	ExcludeDraft    bool
	ExcludeDeleted  bool
	Verdicts        []string
	Order           string
	Feed            string
	FollowerAddr    string
	PinnedOnly      bool
	ExcludePinned   bool
	GoldOnly        bool
	Tags            []string
}

func (s *Store) InsertPost(ctx context.Context, p *entity.Post) (int64, error) {
	const q = `
INSERT INTO square_post (
    author_addr, author_name, author_kind, title, intro, content, tag, media_json,
    comments_enabled, status, comment_count, like_count, view_count, created_at, updated_at,
    publish_at, expire_at, expire_after_sec
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,0,0,0,NOW(),NOW(),$11,$12,$13)
RETURNING id, created_at, updated_at`
	err := s.db.QueryRowContext(ctx, q,
		p.AuthorAddr, p.AuthorName, p.AuthorKind, p.Title, p.Intro, p.Content, p.Tag, p.MediaJSON,
		p.CommentsEnabled, p.Status, p.PublishAt, p.ExpireAt, p.ExpireAfterSec,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert post: %w", err)
	}
	return p.ID, nil
}

// AuthorPostText 只取比较重复内容需要的字段，与查询列一一对应。
type AuthorPostText struct {
	Title   string
	Intro   string
	Content string
}

func (s *Store) ListAuthorPostTexts(ctx context.Context, addr string, since time.Time, limit int) ([]AuthorPostText, error) {
	if limit <= 0 {
		limit = 40
	}
	const q = `
SELECT COALESCE(title,''), COALESCE(intro,''), COALESCE(content,'')
FROM square_post
WHERE author_addr=$1 AND created_at>=$2
ORDER BY created_at DESC
LIMIT $3`
	rows, err := s.db.QueryContext(ctx, q, addr, since, limit)
	if err != nil {
		return nil, fmt.Errorf("list author post texts addr=%s since=%s limit=%d: %w", addr, since.Format(time.RFC3339), limit, err)
	}
	defer rows.Close()
	out := make([]AuthorPostText, 0)
	for rows.Next() {
		var row AuthorPostText
		if err := rows.Scan(&row.Title, &row.Intro, &row.Content); err != nil {
			return nil, fmt.Errorf("scan author post text addr=%s: %w", addr, err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate author post texts addr=%s: %w", addr, err)
	}
	return out, nil
}

func (s *Store) UpdateOfficialPost(ctx context.Context, p *entity.Post, fromStatus int16) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
UPDATE square_post
SET title=$2, intro=$3, content=$4, media_json=$5, comments_enabled=$6, status=$7, author_name=$8, tag=$9,
    publish_at=$10, expire_at=$11, expire_after_sec=$12, updated_at=NOW()
WHERE id=$1 AND author_addr=$13 AND status=$14`,
		p.ID, p.Title, p.Intro, p.Content, p.MediaJSON, p.CommentsEnabled, p.Status, p.AuthorName, p.Tag,
		p.PublishAt, p.ExpireAt, p.ExpireAfterSec,
		entity.PlatformOfficialAddr, fromStatus)
	if err != nil {
		return false, fmt.Errorf("update official post id=%d fromStatus=%d toStatus=%d: %w", p.ID, fromStatus, p.Status, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) UpdateUserPost(ctx context.Context, p *entity.Post) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
UPDATE square_post
SET title=$2, intro=$3, content=$4, media_json=$5, comments_enabled=$6, status=$7, author_name=$8, updated_at=NOW()
WHERE id=$1 AND author_addr=$9 AND status <> $10 AND status <> $11`,
		p.ID, p.Title, p.Intro, p.Content, p.MediaJSON, p.CommentsEnabled, p.Status, p.AuthorName,
		p.AuthorAddr, entity.StatusDeleted, entity.StatusDraft)
	if err != nil {
		return false, fmt.Errorf("update user post id=%d addr=%s status=%d: %w", p.ID, p.AuthorAddr, p.Status, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) GetPost(ctx context.Context, id int64) (*entity.Post, error) {
	return s.scanPost(s.db.QueryRowContext(ctx, postSelect+` FROM square_post p WHERE p.id=$1`, id))
}

func (s *Store) UpdatePostStatus(ctx context.Context, id int64, status int16) error {
	// $2 既写入 smallint 列，又参与比较。不强制类型时，PostgreSQL 会把同一参数推成 text 和 smallint。
	_, err := s.db.ExecContext(ctx, `
UPDATE square_post
SET status=$2::smallint, pinned_at=CASE WHEN $2::smallint=$3::smallint THEN pinned_at ELSE NULL::timestamptz END, updated_at=NOW()
WHERE id=$1`, id, status, entity.StatusVisible)
	if err != nil {
		return fmt.Errorf("update post status id=%d status=%d: %w", id, status, err)
	}
	return nil
}

func (s *Store) UpdatePostStatusMany(ctx context.Context, ids []int64, status int16) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := []any{status}
	ph := make([]string, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
		ph = append(ph, fmt.Sprintf("$%d", len(args)))
	}
	args = append(args, entity.StatusVisible)
	visiblePH := fmt.Sprintf("$%d", len(args))
	q := `UPDATE square_post SET status=$1, pinned_at=CASE WHEN $1=` + visiblePH + ` THEN pinned_at ELSE NULL::timestamptz END, updated_at=NOW()
WHERE id IN (` + strings.Join(ph, ",") + `) AND status <> $1
RETURNING id`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("update post status many n=%d status=%d: %w", len(ids), status, err)
	}
	defer rows.Close()
	updated := make([]int64, 0, len(ids))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		updated = append(updated, id)
	}
	return updated, rows.Err()
}

func (s *Store) UpdateCommentsEnabled(ctx context.Context, id int64, enabled int16) error {
	_, err := s.db.ExecContext(ctx, `UPDATE square_post SET comments_enabled=$2, updated_at=NOW() WHERE id=$1`, id, enabled)
	if err != nil {
		return fmt.Errorf("update comments_enabled id=%d: %w", id, err)
	}
	return nil
}

func (s *Store) HidePostsByAuthor(ctx context.Context, addr string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_post SET status=$2, pinned_at=NULL::timestamptz, updated_at=NOW()
WHERE author_addr=$1 AND status=$3`, addr, entity.StatusHidden, entity.StatusVisible)
	if err != nil {
		return fmt.Errorf("hide posts by author=%s: %w", addr, err)
	}
	return nil
}

func (s *Store) ListPosts(ctx context.Context, f PostListFilter, page entity.PageQuery) ([]entity.Post, int64, error) {
	from, where, args := postFromWhere(f)
	countQ := `SELECT COUNT(1) ` + from + " " + where
	var total int64
	if err := s.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}
	page = page.Normalize()
	args = append(args, page.PageSize, page.Offset())
	q := postSelect + " " + from + " " + where + sqlOrderBy(f.Order, "DESC") + fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()
	list := make([]entity.Post, 0)
	for rows.Next() {
		p, err := scanPostRows(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *p)
	}
	return list, total, rows.Err()
}

func (s *Store) ListPinned(ctx context.Context, f PostListFilter) ([]entity.Post, error) {
	f.PinnedOnly = true
	f.ExcludePinned = false
	f.From = nil
	f.To = nil
	f.OnlyVisible = true
	from, where, args := postFromWhere(f)
	args = append(args, entity.MaxPinnedPosts)
	q := postSelect + " " + from + " " + where + " ORDER BY p.pinned_at DESC, p.id DESC" + fmt.Sprintf(" LIMIT $%d", len(args))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list pinned posts: %w", err)
	}
	defer rows.Close()
	list := make([]entity.Post, 0)
	for rows.Next() {
		p, err := scanPostRows(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *p)
	}
	return list, rows.Err()
}

func (s *Store) CountPinnedVisible(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(1) FROM square_post WHERE pinned_at IS NOT NULL AND status=$1`, entity.StatusVisible).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count pinned visible: %w", err)
	}
	return n, nil
}

func (s *Store) SetPostPinned(ctx context.Context, id int64, pinned bool) (bool, error) {
	var (
		res sql.Result
		err error
	)
	if pinned {
		res, err = s.db.ExecContext(ctx, `
UPDATE square_post SET pinned_at=NOW(), updated_at=NOW() WHERE id=$1 AND status=$2`, id, entity.StatusVisible)
	} else {
		res, err = s.db.ExecContext(ctx, `
UPDATE square_post SET pinned_at=NULL, updated_at=NOW() WHERE id=$1 AND pinned_at IS NOT NULL`, id)
	}
	if err != nil {
		return false, fmt.Errorf("set post pinned id=%d pinned=%t: %w", id, pinned, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) LikedSet(ctx context.Context, addr string, postIDs []int64) (map[int64]struct{}, error) {
	out := make(map[int64]struct{})
	if addr == "" || len(postIDs) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(postIDs)+1)
	args = append(args, addr)
	ph := make([]string, 0, len(postIDs))
	for i, id := range postIDs {
		args = append(args, id)
		ph = append(ph, fmt.Sprintf("$%d", i+2))
	}
	q := `SELECT post_id FROM square_post_like WHERE addr=$1 AND post_id IN (` + strings.Join(ph, ",") + `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("liked set: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func (s *Store) AddLike(ctx context.Context, postID int64, addr string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `INSERT INTO square_post_like (post_id, addr) VALUES ($1,$2) ON CONFLICT DO NOTHING`, postID, addr)
	if err != nil {
		return false, fmt.Errorf("insert like post=%d addr=%s: %w", postID, addr, err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE square_post SET like_count=like_count+1, updated_at=NOW() WHERE id=$1`, postID); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) RemoveLike(ctx context.Context, postID int64, addr string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `DELETE FROM square_post_like WHERE post_id=$1 AND addr=$2`, postID, addr)
	if err != nil {
		return false, fmt.Errorf("delete like post=%d addr=%s: %w", postID, addr, err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE square_post SET like_count=GREATEST(like_count-1,0), updated_at=NOW() WHERE id=$1`, postID); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) AddView(ctx context.Context, postID int64, addr string, day time.Time) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `
INSERT INTO square_post_view (post_id, addr, viewed_on) VALUES ($1,$2,$3)
ON CONFLICT DO NOTHING`, postID, addr, day.Format("2006-01-02"))
	if err != nil {
		return false, fmt.Errorf("insert view post=%d addr=%s: %w", postID, addr, err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE square_post SET view_count=view_count+1, updated_at=NOW() WHERE id=$1`, postID); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) AuthorTraffic(ctx context.Context, addr string) (entity.AuthorTraffic, error) {
	var out entity.AuthorTraffic
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(1), COALESCE(SUM(view_count),0), COALESCE(SUM(like_count),0), COALESCE(SUM(comment_count),0)
FROM square_post
WHERE author_addr=$1 AND status <> $2 AND status <> $3`, addr, entity.StatusDeleted, entity.StatusDraft).
		Scan(&out.TotalPosts, &out.TotalViews, &out.TotalLikes, &out.TotalComments)
	if err != nil {
		return entity.AuthorTraffic{}, fmt.Errorf("author traffic addr=%s: %w", addr, err)
	}
	return out, nil
}

func (s *Store) ListExpiredPosts(ctx context.Context, before time.Time, limit int) ([]entity.Post, error) {
	if limit <= 0 {
		limit = 100
	}
	q := postSelect + ` FROM square_post p WHERE p.created_at < $1 AND p.status <> $2 ORDER BY p.id ASC LIMIT $3`
	rows, err := s.db.QueryContext(ctx, q, before, entity.StatusDeleted, limit)
	if err != nil {
		return nil, fmt.Errorf("list expired posts before=%s limit=%d: %w", before.Format(time.RFC3339), limit, err)
	}
	defer rows.Close()
	list := make([]entity.Post, 0)
	for rows.Next() {
		p, err := scanPostRows(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *p)
	}
	return list, rows.Err()
}

func (s *Store) CountLiveURLRefs(ctx context.Context, url string) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(1) FROM square_post
WHERE status <> $1 AND (
    media_json @> to_jsonb(ARRAY[$2::text])
    OR content LIKE '%' || $2 || '%'
)`, entity.StatusDeleted, url).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count live url refs url=%s: %w", url, err)
	}
	return n, nil
}

func (s *Store) OfficialAvatarExists(ctx context.Context, url string) (bool, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_official_account WHERE avatar=$1`, url).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("official avatar exists url=%s: %w", url, err)
	}
	return n > 0, nil
}

func (s *Store) IncrCommentCount(ctx context.Context, postID int64, delta int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE square_post SET comment_count=GREATEST(comment_count+$2,0), updated_at=NOW() WHERE id=$1`, postID, delta)
	if err != nil {
		return fmt.Errorf("incr comment_count post=%d delta=%d: %w", postID, delta, err)
	}
	return nil
}

const postSelect = `
SELECT p.id, p.author_addr, p.author_name, p.author_kind, p.title, p.intro, p.content, p.tag, p.media_json,
       p.comments_enabled, p.status, p.comment_count, p.like_count, p.view_count, p.created_at, p.updated_at, p.pinned_at,
       p.publish_at, p.expire_at, p.expire_after_sec`

func postFromWhere(f PostListFilter) (string, string, []any) {
	args := []any{}
	from := "FROM square_post p"
	if addr := strings.TrimSpace(f.FollowerAddr); addr != "" {
		args = append(args, addr)
		from += fmt.Sprintf(" INNER JOIN square_follow f ON f.target_addr = p.author_addr AND f.follower_addr=$%d", len(args))
	}
	if isAuthorStatOrder(f.Order) {
		from += authorStatJoins()
	}
	where, rest := postWhere(f, args)
	return from, where, rest
}

func postWhere(f PostListFilter, args []any) (string, []any) {
	w := []string{"1=1"}
	add := func(cond string, v any) {
		args = append(args, v)
		w = append(w, fmt.Sprintf(cond, len(args)))
	}
	if f.OnlyVisible {
		add("p.status=$%d", entity.StatusVisible)
	} else if len(f.Statuses) > 0 {
		ph := make([]string, 0, len(f.Statuses))
		for _, st := range f.Statuses {
			args = append(args, st)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		w = append(w, "p.status IN ("+strings.Join(ph, ",")+")")
	} else {
		if f.ExcludeDeleted {
			add("p.status<>$%d", entity.StatusDeleted)
		}
		if f.ExcludeDraft {
			add("p.status<>$%d", entity.StatusDraft)
		}
	}
	if addrs := compactStrings(f.AuthorAddrs); len(addrs) > 0 {
		ph := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			args = append(args, addr)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		w = append(w, "p.author_addr IN ("+strings.Join(ph, ",")+")")
	}
	if names := compactStrings(f.AuthorNames); len(names) > 0 {
		or := make([]string, 0, len(names))
		for _, name := range names {
			args = append(args, likeContains(name))
			or = append(or, fmt.Sprintf("p.author_name ILIKE $%d ESCAPE '\\'", len(args)))
		}
		w = append(w, "("+strings.Join(or, " OR ")+")")
	}
	if names := compactStrings(f.AuthorNameExact); len(names) > 0 {
		ph := make([]string, 0, len(names))
		for _, name := range names {
			args = append(args, name)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		w = append(w, "p.author_name IN ("+strings.Join(ph, ",")+")")
	}
	if f.Kind != nil {
		add("p.author_kind=$%d", *f.Kind)
	}
	if tags := compactStrings(f.Tags); len(tags) > 0 {
		ph := make([]string, 0, len(tags))
		for _, tag := range tags {
			if tag == "新闻" {
				tag = entity.OfficialPostTagMessage
			}
			args = append(args, tag)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		w = append(w, "p.tag IN ("+strings.Join(ph, ",")+")")
	}
	if f.From != nil {
		add("p.created_at>=$%d", *f.From)
	}
	if f.To != nil {
		add("p.created_at<=$%d", *f.To)
	}
	if f.PinnedOnly {
		w = append(w, "p.pinned_at IS NOT NULL")
	}
	if f.ExcludePinned {
		w = append(w, "p.pinned_at IS NULL")
	}
	if f.GoldOnly {
		w = append(w, `p.author_addr IN (SELECT address FROM square_official_account WHERE gold)`)
	}
	if len(f.Verdicts) > 0 {
		ph := make([]string, 0, len(f.Verdicts))
		for _, v := range f.Verdicts {
			args = append(args, v)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		w = append(w, `p.id IN (
			SELECT m.target_id FROM (
				SELECT DISTINCT ON (target_id) target_id, COALESCE(final_verdict, verdict) AS v
				FROM square_content_moderation
				WHERE target_type='post'
				ORDER BY target_id, id DESC
			) m WHERE m.v IN (`+strings.Join(ph, ",")+`)
		)`)
	}
	return "WHERE " + strings.Join(w, " AND "), args
}

// sqlOrderBy 只允许白名单排序，避免把用户输入拼进 SQL。
func sqlOrderBy(order, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "hot":
		return " ORDER BY (p.like_count * 2 + p.comment_count * 3) DESC, p.id DESC"
	case "likes":
		return " ORDER BY p.like_count DESC, p.id DESC"
	case "comments":
		return " ORDER BY p.comment_count DESC, p.id DESC"
	case "followers":
		return " ORDER BY COALESCE(author_followers.n, 0) DESC, p.id DESC"
	case "posts":
		return " ORDER BY COALESCE(author_posts.n, 0) DESC, p.id DESC"
	}
	dir := createdAtDir(order, fallback)
	return " ORDER BY p.created_at " + dir + ", p.id " + dir
}

func isAuthorStatOrder(order string) bool {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "followers", "posts":
		return true
	default:
		return false
	}
}

func authorStatJoins() string {
	return fmt.Sprintf(`
LEFT JOIN (
    SELECT target_addr, COUNT(1) AS n FROM square_follow GROUP BY target_addr
) author_followers ON author_followers.target_addr = p.author_addr
LEFT JOIN (
    SELECT author_addr, COUNT(1) AS n FROM square_post
    WHERE status <> %d AND status <> %d
    GROUP BY author_addr
) author_posts ON author_posts.author_addr = p.author_addr`, entity.StatusDeleted, entity.StatusDraft)
}

func createdAtDir(order, fallback string) string {
	dir := "DESC"
	if strings.EqualFold(strings.TrimSpace(fallback), "ASC") {
		dir = "ASC"
	}
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "asc":
		dir = "ASC"
	case "desc":
		dir = "DESC"
	}
	return dir
}

func sqlOrderByCreatedAt(order, fallback string) string {
	dir := createdAtDir(order, fallback)
	return " ORDER BY created_at " + dir + ", id " + dir
}

func compactStrings(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" && len(v) <= 128 {
			out = append(out, v)
		}
	}
	return out
}

func likeContains(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return "%" + s + "%"
}

func (s *Store) scanPost(row *sql.Row) (*entity.Post, error) {
	p, err := scanPostFields(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan post: %w", err)
	}
	return p, nil
}

func scanPostRows(rows *sql.Rows) (*entity.Post, error) {
	p, err := scanPostFields(rows.Scan)
	if err != nil {
		return nil, fmt.Errorf("scan post row: %w", err)
	}
	return p, nil
}

func scanPostFields(scan func(dest ...any) error) (*entity.Post, error) {
	p := &entity.Post{}
	var pinned, publishAt, expireAt sql.NullTime
	if err := scan(
		&p.ID, &p.AuthorAddr, &p.AuthorName, &p.AuthorKind, &p.Title, &p.Intro, &p.Content, &p.Tag, &p.MediaJSON,
		&p.CommentsEnabled, &p.Status, &p.CommentCount, &p.LikeCount, &p.ViewCount, &p.CreatedAt, &p.UpdatedAt, &pinned,
		&publishAt, &expireAt, &p.ExpireAfterSec,
	); err != nil {
		return nil, err
	}
	if pinned.Valid {
		t := pinned.Time
		p.PinnedAt = &t
	}
	if publishAt.Valid {
		t := publishAt.Time
		p.PublishAt = &t
	}
	if expireAt.Valid {
		t := expireAt.Time
		p.ExpireAt = &t
	}
	return p, nil
}
