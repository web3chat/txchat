package usecase

import (
	"testing"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

func TestCompactMediaURLs(t *testing.T) {
	t.Parallel()
	got, err := compactMediaURLs([]string{" a.jpg ", "", "  ", "b.png"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a.jpg" || got[1] != "b.png" {
		t.Fatalf("got %v", got)
	}
	tooMany := make([]string, maxImages+1)
	for i := range tooMany {
		tooMany[i] = "https://oss.example/" + string(rune('a'+i)) + ".jpg"
	}
	if _, err := compactMediaURLs(tooMany); err == nil {
		t.Fatal("too many images should fail")
	}
}

func TestNormalizePostText(t *testing.T) {
	t.Parallel()
	if _, _, _, err := normalizePostText("", "", "正文", true); err == nil {
		t.Fatal("official title required")
	}
	title, intro, content, err := normalizePostText(" 公告 ", " 说明 ", " 正文 ", true)
	if err != nil {
		t.Fatal(err)
	}
	if title != "公告" || intro != "说明" || content != "正文" {
		t.Fatalf("got %q %q %q", title, intro, content)
	}
	if _, _, _, err := normalizePostText("", "", "正文", false); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := normalizePostText("", "", "", false); err != nil {
		t.Fatal(err)
	}
	if err := draftNeedsText(entity.StatusDraft, false, "", ""); err == nil {
		t.Fatal("empty draft should fail")
	}
	if err := draftNeedsText(entity.StatusDraft, false, "标题", ""); err != nil {
		t.Fatal(err)
	}
	if err := draftNeedsText(entity.StatusDraft, false, "", "正文"); err != nil {
		t.Fatal(err)
	}
	if err := draftNeedsText(entity.StatusDraft, true, "", ""); err != nil {
		t.Fatal("scheduled drafts are checked by requireTitle")
	}
	if _, _, _, err := normalizePostText("", "", "", true); err == nil {
		t.Fatal("publish requires content")
	}
	longTitle := make([]rune, maxTitleRunes+1)
	for i := range longTitle {
		longTitle[i] = '标'
	}
	if _, _, _, err := normalizePostText(string(longTitle), "", "正文", false); err == nil {
		t.Fatal("long title should fail")
	}
	okBody := string(make([]rune, maxPostRunes))
	if _, _, _, err := normalizePostText("", "", okBody, false); err != nil {
		t.Fatal(err)
	}
	longBody := string(make([]rune, maxPostRunes+1))
	if _, _, _, err := normalizePostText("", "", longBody, false); err == nil {
		t.Fatal("long content should fail")
	}
}

func TestOfficialEditable(t *testing.T) {
	t.Parallel()
	if !officialEditable(entity.StatusDraft) || !officialEditable(entity.StatusVisible) || !officialEditable(entity.StatusHidden) {
		t.Fatal("draft/visible/hidden should be editable")
	}
	if officialEditable(entity.StatusDeleted) || officialEditable(entity.StatusPending) {
		t.Fatal("deleted/pending should not be official-editable")
	}
}

func TestMergePinnedPosts(t *testing.T) {
	t.Parallel()
	pins := []entity.Post{{ID: 9}, {ID: 8}}
	list := []entity.Post{{ID: 3}, {ID: 2}}
	got := mergePinnedPosts(pins, list, 1)
	if len(got) != 4 || got[0].ID != 9 || got[1].ID != 8 || got[2].ID != 3 {
		t.Fatalf("page1=%v", idsOf(got))
	}
	dup := mergePinnedPosts(pins, []entity.Post{{ID: 9}, {ID: 1}}, 1)
	if len(dup) != 3 || dup[0].ID != 9 || dup[2].ID != 1 {
		t.Fatalf("dedup=%v", idsOf(dup))
	}
	later := mergePinnedPosts(pins, list, 2)
	if len(later) != 2 || later[0].ID != 3 {
		t.Fatalf("page2 should keep unpinned only, got %v", idsOf(later))
	}
}

func idsOf(list []entity.Post) []int64 {
	out := make([]int64, len(list))
	for i := range list {
		out[i] = list[i].ID
	}
	return out
}

func TestIsRankOrder(t *testing.T) {
	t.Parallel()
	if !IsRankOrder("hot") || !IsRankOrder("likes") || !IsRankOrder("comments") {
		t.Fatal("rank orders")
	}
	if IsRankOrder("desc") || IsRankOrder("asc") || IsRankOrder("") || IsRankOrder("followers") || IsRankOrder("posts") {
		t.Fatal("time orders should not rank")
	}
}

func TestIsAuthorRankOrder(t *testing.T) {
	t.Parallel()
	if !IsAuthorRankOrder("likes") || !IsAuthorRankOrder("views") || !IsAuthorRankOrder("view") {
		t.Fatal("author rank orders")
	}
	if IsAuthorRankOrder("hot") || IsAuthorRankOrder("comments") || IsAuthorRankOrder("") {
		t.Fatal("post orders should not author-rank")
	}
}

func TestRankWindowStart(t *testing.T) {
	t.Parallel()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 21, 15, 30, 0, 0, loc)
	today := RankWindowStart("today", now)
	if today == nil || !today.Equal(time.Date(2026, 9, 21, 0, 0, 0, 0, loc)) {
		t.Fatalf("today=%v", today)
	}
	week := RankWindowStart("week", now)
	if week == nil || !week.Equal(time.Date(2026, 9, 15, 0, 0, 0, 0, loc)) {
		t.Fatalf("week=%v", week)
	}
	month := RankWindowStart("month", now)
	if month == nil || !month.Equal(time.Date(2026, 8, 23, 0, 0, 0, 0, loc)) {
		t.Fatalf("month=%v", month)
	}
}


func TestNormalizeOfficialTag(t *testing.T) {
	t.Parallel()
	if got, err := NormalizeOfficialTag(" 公告 "); err != nil || got != entity.OfficialPostTagAnnouncement {
		t.Fatalf("got %q %v", got, err)
	}
	if got, err := NormalizeOfficialTag(""); err != nil || got != "" {
		t.Fatalf("empty got %q %v", got, err)
	}
	if _, err := NormalizeOfficialTag("八卦"); err == nil {
		t.Fatal("invalid tag should fail")
	}
	if got, err := NormalizeOfficialTag("新闻"); err != nil || got != entity.OfficialPostTagMessage {
		t.Fatalf("legacy news got %q %v", got, err)
	}
	for _, tag := range entity.OfficialPostTags() {
		if got, err := NormalizeOfficialTag(tag); err != nil || got != tag {
			t.Fatalf("%q -> %q %v", tag, got, err)
		}
	}
}
