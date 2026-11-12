package usecase

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

const retentionBatch = 100

var mdImageURL = regexp.MustCompile(`!\[[^\]]*]\((https?://[^)\s]+)\)`)

func ExtractContentURLs(mediaJSON []byte, content string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	add := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	var media []string
	if len(mediaJSON) > 0 {
		_ = json.Unmarshal(mediaJSON, &media)
	}
	for _, u := range media {
		add(u)
	}
	for _, m := range mdImageURL.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	return out
}

func (s *Service) SweepExpiredPosts(ctx context.Context) (int, error) {
	days := s.cfg.Retention.TTLDays
	if days <= 0 {
		return 0, nil
	}
	before := time.Now().AddDate(0, 0, -days)
	list, err := s.store.ListExpiredPosts(ctx, before, retentionBatch)
	if err != nil {
		s.log.Error("list expired posts failed", "before", before.Format(time.RFC3339), "ttl_days", days, "err", err)
		return 0, err
	}
	if len(list) == 0 {
		return 0, nil
	}
	ids := make([]int64, 0, len(list))
	for i := range list {
		ids = append(ids, list[i].ID)
	}
	updated, err := s.store.UpdatePostStatusMany(ctx, ids, entity.StatusDeleted)
	if err != nil {
		s.log.Error("soft delete expired posts failed", "ids", ids, "before", before.Format(time.RFC3339), "err", err)
		return 0, err
	}
	s.log.Info("retention soft deleted", "n", len(updated), "ids", updated, "ttl_days", days, "before", before.Format(time.RFC3339))
	idSet := make(map[int64]struct{}, len(updated))
	for _, id := range updated {
		idSet[id] = struct{}{}
	}
	for i := range list {
		if _, ok := idSet[list[i].ID]; !ok {
			continue
		}
		s.deleteExpiredMedia(ctx, &list[i])
	}
	return len(updated), nil
}

func (s *Service) deleteExpiredMedia(ctx context.Context, p *entity.Post) {
	urls := ExtractContentURLs(p.MediaJSON, p.Content)
	if len(urls) == 0 {
		return
	}
	if !s.cfg.OSS.DeleteEnabled {
		s.log.Debug("oss delete skipped", "post", p.ID, "urls", len(urls), "oss_delete", false)
		return
	}
	for _, url := range urls {
		avatar, err := s.store.OfficialAvatarExists(ctx, url)
		if err != nil {
			s.log.Error("check official avatar failed", "post", p.ID, "url", url, "err", err)
			continue
		}
		if avatar {
			s.log.Info("skip official avatar url", "post", p.ID, "url", url)
			continue
		}
		n, err := s.store.CountLiveURLRefs(ctx, url)
		if err != nil {
			s.log.Error("count live url refs failed", "post", p.ID, "url", url, "err", err)
			continue
		}
		if n > 0 {
			s.log.Info("skip url still referenced", "post", p.ID, "url", url, "refs", n)
			continue
		}
		if err := s.oss.Delete(ctx, url); err != nil {
			s.log.Error("oss delete failed", "post", p.ID, "url", url, "err", err)
			continue
		}
		s.log.Info("oss delete ok", "post", p.ID, "url", url)
	}
}
