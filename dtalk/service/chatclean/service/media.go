package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"
	"github.com/web3chat/txchat/dtalk/service/chatclean/config"
)

const (
	prefixPicture = "chatList/picture/"
	prefixVideo   = "chatList/video/"
	prefixAudio   = "chatList/audio/"
	prefixFile    = "chatList/file/"
	squareMarker  = "_square_"
)

// mediaPrefixes 是聊天媒体。不包含 chatList/avatar/。
var mediaPrefixes = []string{prefixPicture, prefixVideo, prefixAudio, prefixFile}

type objectStore interface {
	List(ctx context.Context, prefix string, recursive bool) ([]string, error)
	Remove(ctx context.Context, key string) error
}

type ossResult struct {
	Removed       int
	SkippedSquare int
	More          bool
}

type minioStore struct {
	client *minio.Client
	bucket string
}

func newMinioStore(cfg config.MinIO) (objectStore, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("retention minio new endpoint=%s bucket=%s: %w", cfg.Endpoint, cfg.Bucket, err)
	}
	return &minioStore{client: client, bucket: cfg.Bucket}, nil
}

func (s *minioStore) List(ctx context.Context, prefix string, recursive bool) ([]string, error) {
	ch := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: recursive,
	})
	keys := make([]string, 0)
	for obj := range ch {
		if obj.Err != nil {
			return keys, fmt.Errorf("list prefix=%s: %w", prefix, obj.Err)
		}
		if obj.Key == "" {
			continue
		}
		keys = append(keys, obj.Key)
	}
	return keys, nil
}

func (s *minioStore) Remove(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("remove key=%s: %w", key, err)
	}
	return nil
}

// cleanExpiredObjects 删除目录名早于 cutoffDay 的对象。
// picture 键名包含 _square_ 的留下。单个对象失败继续。达到 maxObjects 就停，剩下的留给下一轮。
func cleanExpiredObjects(ctx context.Context, store objectStore, prefixes []string, cutoffDay string, maxObjects int) (ossResult, error) {
	var res ossResult
	if maxObjects <= 0 {
		maxObjects = defaultMaxObjects
	}
	for _, prefix := range prefixes {
		if err := ctx.Err(); err != nil {
			res.More = true
			return res, err
		}
		if res.Removed >= maxObjects {
			res.More = true
			return res, nil
		}
		listed, err := store.List(ctx, prefix, false)
		if err != nil {
			log.Error().Err(err).Str("prefix", prefix).Str("cutoff_day", cutoffDay).Msg("retention list date dirs")
			res.More = true
			continue
		}
		folders := dateFolders(prefix, listed)
		for _, folder := range folders {
			if !DateFolderExpired(folder, cutoffDay) {
				continue
			}
			if res.Removed >= maxObjects {
				res.More = true
				return res, nil
			}
			keys, err := store.List(ctx, prefix+folder+"/", true)
			if err != nil {
				log.Error().Err(err).Str("prefix", prefix).Str("dir", folder).Str("cutoff_day", cutoffDay).Msg("retention list objects")
				res.More = true
				continue
			}
			removed, skipped, failed := removeFolder(ctx, store, prefix, folder, keys, cutoffDay, maxObjects, &res)
			if removed > 0 || skipped > 0 || failed > 0 {
				log.Info().
					Str("prefix", prefix).
					Str("dir", folder).
					Str("cutoff_day", cutoffDay).
					Int("removed", removed).
					Int("skipped_square", skipped).
					Int("failed", failed).
					Bool("more", res.More).
					Msg("retention oss dir")
			}
			if res.More {
				if err = ctx.Err(); err != nil {
					return res, err
				}
				return res, nil
			}
		}
	}
	return res, nil
}

func removeFolder(ctx context.Context, store objectStore, prefix, folder string, keys []string, cutoffDay string, maxObjects int, res *ossResult) (removed, skipped, failed int) {
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			res.More = true
			return removed, skipped, failed
		}
		if key == "" || strings.HasSuffix(key, "/") {
			continue
		}
		if prefix == prefixPicture && strings.Contains(key, squareMarker) {
			skipped++
			res.SkippedSquare++
			continue
		}
		if res.Removed >= maxObjects {
			res.More = true
			return removed, skipped, failed
		}
		if err := store.Remove(ctx, key); err != nil {
			failed++
			log.Error().Err(err).
				Str("key", key).
				Str("prefix", prefix).
				Str("dir", folder).
				Str("cutoff_day", cutoffDay).
				Msg("retention remove object")
			continue
		}
		removed++
		res.Removed++
	}
	return removed, skipped, failed
}

func dateFolders(prefix string, keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		name, ok := dateFolderName(prefix, key)
		if !ok {
			continue
		}
		seen[name] = struct{}{}
	}
	folders := make([]string, 0, len(seen))
	for name := range seen {
		folders = append(folders, name)
	}
	sort.Strings(folders)
	return folders
}

func dateFolderName(prefix, key string) (string, bool) {
	if !strings.HasPrefix(key, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(key, prefix), "/")
	if rest == "" {
		return "", false
	}
	name := rest
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		name = rest[:i]
	}
	if !isDateFolder(name) {
		return "", false
	}
	return name, true
}
