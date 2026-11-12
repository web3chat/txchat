package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

func (s *Service) AdminOverview(ctx context.Context, days int, authorAddrs []string) (*entity.OverviewStats, error) {
	days = clampOverviewDays(days)
	authorAddrs = clampAuthorAddrs(authorAddrs)
	loc, err := timeLocation()
	if err != nil {
		s.log.Error("load timezone failed", "tz", "Asia/Shanghai", "err", err)
		loc = time.FixedZone("CST", 8*3600)
	}
	now := nowIn(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	until := today.AddDate(0, 0, 1)
	from := today.AddDate(0, 0, -(days - 1))
	stats, err := s.store.OverviewStats(ctx, from, until)
	if err != nil {
		s.log.Error("overview stats failed", "days", days, "from", from.Format(time.RFC3339), "until", until.Format(time.RFC3339), "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	stats.Days = days
	stats.Timezone = "Asia/Shanghai"
	if stats.Series == nil {
		stats.Series = []entity.StatDay{}
	}
	if len(authorAddrs) > 0 {
		series, total, err := s.store.AuthorPostSeries(ctx, from, until, authorAddrs)
		if err != nil {
			s.log.Error("author post series failed", "days", days, "addrs", authorAddrs, "from", from.Format(time.RFC3339), "until", until.Format(time.RFC3339), "err", err)
			return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
		}
		stats.AuthorAddrs = authorAddrs
		stats.AuthorTotal = total
		stats.AuthorSeries = series
	}
	if stats.AuthorSeries == nil {
		stats.AuthorSeries = []entity.StatDay{}
	}
	if stats.AuthorAddrs == nil {
		stats.AuthorAddrs = []string{}
	}
	return stats, nil
}

func clampAuthorAddrs(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	seen := map[string]struct{}{}
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" || len(addr) > 128 {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func (s *Service) AdminListAuthorRanks(ctx context.Context, f repository.AuthorRankFilter, page entity.PageQuery) ([]entity.AuthorRank, int64, error) {
	if !IsAuthorRankOrder(f.Order) {
		f.Order = "likes"
	}
	if f.From == nil {
		f.From = RankWindowStart("week", time.Now())
	}
	list, total, err := s.store.ListAuthorRanks(ctx, f, page)
	if err != nil {
		from := ""
		if f.From != nil {
			from = f.From.Format(time.RFC3339)
		}
		s.log.Error("admin list author ranks failed", "order", f.Order, "from", from, "addrs", f.AuthorAddrs, "names", f.AuthorNames, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.fillAuthorRankAvatars(ctx, list)
	return list, total, nil
}

func (s *Service) fillAuthorRankAvatars(ctx context.Context, list []entity.AuthorRank) {
	addrs := make([]string, 0)
	for i := range list {
		if list[i].AuthorKind == entity.KindOfficial || list[i].IsOfficial {
			addrs = append(addrs, list[i].Address)
		}
	}
	if len(addrs) == 0 {
		return
	}
	m, err := s.store.OfficialAvatarMap(ctx, addrs)
	if err != nil {
		s.log.Error("author rank official avatar map failed", "addrs", addrs, "err", err)
		return
	}
	for i := range list {
		if list[i].AuthorKind != entity.KindOfficial && !list[i].IsOfficial {
			continue
		}
		list[i].AuthorAvatar = m[list[i].Address]
	}
}

func clampOverviewDays(days int) int {
	if days < 7 {
		return 7
	}
	if days > 90 {
		return 90
	}
	return days
}
