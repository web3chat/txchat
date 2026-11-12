package usecase

import (
	"context"
	"log/slog"
	"testing"

	"github.com/web3chat/txchat/dtalk/service/square/internal/config"
	"github.com/web3chat/txchat/dtalk/service/square/internal/ossclient"
)

func TestExtractContentURLs(t *testing.T) {
	t.Parallel()
	media := []byte(`["https://cdn.example/a.png","https://cdn.example/a.png"]`)
	content := "正文 ![x](https://cdn.example/b.jpg) 还有 ![y](https://cdn.example/a.png)"
	got := ExtractContentURLs(media, content)
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if got[0] != "https://cdn.example/a.png" || got[1] != "https://cdn.example/b.jpg" {
		t.Fatalf("got %v", got)
	}
}

func TestSweepExpiredPostsDisabled(t *testing.T) {
	t.Parallel()
	s := &Service{
		cfg: &config.Config{Retention: config.RetentionConfig{TTLDays: 0}},
		oss: ossclient.New(config.OSSConfig{}),
		log: slog.Default(),
	}
	n, err := s.SweepExpiredPosts(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}
