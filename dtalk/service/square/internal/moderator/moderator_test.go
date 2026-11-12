package moderator

import (
	"strings"
	"testing"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

func TestStatusOf(t *testing.T) {
	if StatusOf(entity.VerdictPass) != entity.StatusVisible {
		t.Fatal("pass")
	}
	if StatusOf(entity.VerdictReject) != entity.StatusAIReject {
		t.Fatal("reject")
	}
	if StatusOf(entity.VerdictReview) != entity.StatusPending {
		t.Fatal("review")
	}
}

func TestLoosenResult(t *testing.T) {
	pass := loosenResult(Result{Verdict: "review", Reason: "看不懂"})
	if pass.Verdict != entity.VerdictPass {
		t.Fatalf("unclear text should pass, got %s", pass.Verdict)
	}
	news := loosenResult(Result{Verdict: "review", Categories: []string{"politics"}, Reason: "时政"})
	if news.Verdict != entity.VerdictPass {
		t.Fatalf("politics review should pass, got %s", news.Verdict)
	}
	porn := loosenResult(Result{Verdict: "review", Categories: []string{"porn"}})
	if porn.Verdict != entity.VerdictReview {
		t.Fatalf("porn suspicion should stay review, got %s", porn.Verdict)
	}
	reject := loosenResult(Result{Verdict: "reject", Categories: []string{"illegal"}})
	if reject.Verdict != entity.VerdictReject {
		t.Fatalf("explicit illegal should stay reject, got %s", reject.Verdict)
	}
	unknown := loosenResult(Result{Verdict: "maybe"})
	if unknown.Verdict != entity.VerdictPass {
		t.Fatalf("unknown verdict should pass, got %s", unknown.Verdict)
	}
}

func TestHoldImageFailure(t *testing.T) {
	t.Parallel()
	held := HoldImageFailure(Result{Verdict: entity.VerdictPass, Unavailable: true, Reason: "moderator timeout"}, true)
	if held.Verdict != entity.VerdictReview {
		t.Fatalf("unreviewed image should stay pending, got %s", held.Verdict)
	}
	text := HoldImageFailure(Result{Verdict: entity.VerdictPass, Unavailable: true}, false)
	if text.Verdict != entity.VerdictPass {
		t.Fatalf("text failure should stay passed by policy, got %s", text.Verdict)
	}
	reject := HoldImageFailure(Result{Verdict: entity.VerdictReject, Unavailable: true}, true)
	if reject.Verdict != entity.VerdictReject {
		t.Fatalf("explicit reject should stay, got %s", reject.Verdict)
	}
}

func TestUserContentImages(t *testing.T) {
	t.Parallel()
	textOnly, ok := userContent("你好", nil).(string)
	if !ok || !strings.Contains(textOnly, "你好") {
		t.Fatalf("text content=%v", userContent("你好", nil))
	}
	parts, ok := userContent("你好", []string{"https://cdn.example/a.jpg"}).([]map[string]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("parts=%v", userContent("你好", []string{"https://cdn.example/a.jpg"}))
	}
	if parts[1]["type"] != "image_url" {
		t.Fatalf("image part=%v", parts[1])
	}
	kept, dropped := httpImageURLs([]string{" https://cdn.example/a.jpg ", "ftp://x", "", "data:image/png;base64,aa"})
	if dropped != 2 || len(kept) != 1 || kept[0] != "https://cdn.example/a.jpg" {
		t.Fatalf("kept=%v dropped=%d", kept, dropped)
	}
}
func TestResolveTextModel(t *testing.T) {
	t.Parallel()
	if got := resolveTextModel(" deepseek-chat "); got != "deepseek-flash" {
		t.Fatalf("retired model=%s", got)
	}
	if got := resolveTextModel(""); got != "deepseek-flash" {
		t.Fatalf("empty model=%s", got)
	}
	if got := resolveTextModel("deepseek-v4-pro"); got != "deepseek-v4-pro" {
		t.Fatalf("custom model=%s", got)
	}
}

func TestHTTPErrorSnippet(t *testing.T) {
	t.Parallel()
	got := httpErrorSnippet([]byte("  model  not  exist  key sk-secret\n"), "sk-secret")
	if got != "model not exist key [redacted]" {
		t.Fatalf("snippet=%s", got)
	}
	long := strings.Repeat("错", 500)
	if n := len([]rune(httpErrorSnippet([]byte(long), ""))); n != 400 {
		t.Fatalf("truncated len=%d", n)
	}
}

func TestTightenResult(t *testing.T) {
	pass := tightenResult(Result{Verdict: "pass"})
	if pass.Verdict != entity.VerdictPass {
		t.Fatalf("pass should stay, got %s", pass.Verdict)
	}
	review := tightenResult(Result{Verdict: "review", Reason: "拿不准"})
	if review.Verdict != entity.VerdictReview {
		t.Fatalf("review should stay in strict, got %s", review.Verdict)
	}
	unknown := tightenResult(Result{Verdict: "maybe"})
	if unknown.Verdict != entity.VerdictReview {
		t.Fatalf("unknown should review in strict, got %s", unknown.Verdict)
	}
}
