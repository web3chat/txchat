package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

type ReportListFilter struct {
	Statuses      []int16
	Reasons       []string
	ReporterAddrs []string
	TargetIDs     []int64
}

func (s *Store) InsertReport(ctx context.Context, r *entity.Report) (int64, error) {
	const q = `
INSERT INTO square_report (reporter_addr, target_type, target_id, reason, detail, status, created_at)
VALUES ($1,$2,$3,$4,$5,$6,NOW())
RETURNING id, created_at`
	err := s.db.QueryRowContext(ctx, q,
		r.ReporterAddr, r.TargetType, r.TargetID, r.Reason, r.Detail, r.Status,
	).Scan(&r.ID, &r.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, errAlreadyReported
		}
		return 0, fmt.Errorf("insert report reporter=%s target=%s/%d: %w", r.ReporterAddr, r.TargetType, r.TargetID, err)
	}
	return r.ID, nil
}

func (s *Store) GetReport(ctx context.Context, id int64) (*entity.Report, error) {
	return s.scanReport(s.db.QueryRowContext(ctx, reportSelect+` WHERE r.id=$1`, id))
}

func (s *Store) ListReports(ctx context.Context, f ReportListFilter, page entity.PageQuery) ([]entity.Report, int64, error) {
	where, args := reportWhere(f)
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_report r `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}
	page = page.Normalize()
	args = append(args, page.PageSize, page.Offset())
	q := reportSelect + " " + where + fmt.Sprintf(" ORDER BY r.status ASC, r.created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()
	list := make([]entity.Report, 0)
	for rows.Next() {
		r, err := scanReportRows(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *r)
	}
	return list, total, rows.Err()
}

func (s *Store) HandleReport(ctx context.Context, id int64, status int16, handler string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_report SET status=$2, handled_by=$3, handled_at=NOW()
WHERE id=$1`, id, status, handler)
	if err != nil {
		return fmt.Errorf("handle report id=%d status=%d handler=%s: %w", id, status, handler, err)
	}
	return nil
}

const reportSelect = `
SELECT r.id, r.reporter_addr, r.target_type, r.target_id, r.reason, r.detail, r.status,
       r.handled_by, r.handled_at, r.created_at,
       COALESCE(p.title,''), COALESCE(p.intro,''), COALESCE(p.author_addr,''),
       COALESCE(p.author_name,''), COALESCE(p.author_kind,0), COALESCE(p.status,0)
FROM square_report r
LEFT JOIN square_post p ON p.id = r.target_id AND r.target_type='post'`

func reportWhere(f ReportListFilter) (string, []any) {
	w := []string{"1=1"}
	args := []any{}
	if len(f.Statuses) > 0 {
		ph := make([]string, 0, len(f.Statuses))
		for _, st := range f.Statuses {
			args = append(args, st)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		w = append(w, "r.status IN ("+strings.Join(ph, ",")+")")
	}
	if reasons := compactStrings(f.Reasons); len(reasons) > 0 {
		ph := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			args = append(args, reason)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		w = append(w, "r.reason IN ("+strings.Join(ph, ",")+")")
	}
	if addrs := compactStrings(f.ReporterAddrs); len(addrs) > 0 {
		ph := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			args = append(args, addr)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		w = append(w, "r.reporter_addr IN ("+strings.Join(ph, ",")+")")
	}
	if len(f.TargetIDs) > 0 {
		ph := make([]string, 0, len(f.TargetIDs))
		for _, id := range f.TargetIDs {
			if id <= 0 {
				continue
			}
			args = append(args, id)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		if len(ph) > 0 {
			w = append(w, "r.target_id IN ("+strings.Join(ph, ",")+")")
		}
	}
	return "WHERE " + strings.Join(w, " AND "), args
}

func (s *Store) scanReport(row *sql.Row) (*entity.Report, error) {
	r := &entity.Report{}
	var handled sql.NullTime
	err := row.Scan(
		&r.ID, &r.ReporterAddr, &r.TargetType, &r.TargetID, &r.Reason, &r.Detail, &r.Status,
		&r.HandledBy, &handled, &r.CreatedAt,
		&r.PostTitle, &r.PostIntro, &r.AuthorAddr, &r.AuthorName, &r.AuthorKind, &r.PostStatus,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan report: %w", err)
	}
	if handled.Valid {
		t := handled.Time
		r.HandledAt = &t
	}
	return r, nil
}

func scanReportRows(rows *sql.Rows) (*entity.Report, error) {
	r := &entity.Report{}
	var handled sql.NullTime
	if err := rows.Scan(
		&r.ID, &r.ReporterAddr, &r.TargetType, &r.TargetID, &r.Reason, &r.Detail, &r.Status,
		&r.HandledBy, &handled, &r.CreatedAt,
		&r.PostTitle, &r.PostIntro, &r.AuthorAddr, &r.AuthorName, &r.AuthorKind, &r.PostStatus,
	); err != nil {
		return nil, fmt.Errorf("scan report row: %w", err)
	}
	if handled.Valid {
		t := handled.Time
		r.HandledAt = &t
	}
	return r, nil
}

var errAlreadyReported = fmt.Errorf("already reported")

func IsAlreadyReported(err error) bool {
	return errors.Is(err, errAlreadyReported)
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique")
}
