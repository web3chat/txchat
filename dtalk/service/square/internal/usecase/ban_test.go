package usecase

import "testing"

func TestNormalizeBanAddr(t *testing.T) {
	t.Parallel()
	got, err := normalizeBanAddr(" 1FKxgaEh5fuSm7a35BfUnKYAmradowpiTR ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "1FKxgaEh5fuSm7a35BfUnKYAmradowpiTR" {
		t.Fatalf("got %q", got)
	}
	if _, err := normalizeBanAddr(""); err == nil {
		t.Fatal("empty addr should fail")
	}
	if _, err := normalizeBanAddr(repeatRune('a', 65)); err == nil {
		t.Fatal("long addr should fail")
	}
}

func TestNormalizeBanReason(t *testing.T) {
	t.Parallel()
	got, err := normalizeBanReason(" 违规 ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "违规" {
		t.Fatalf("got %q", got)
	}
	if _, err := normalizeBanReason(repeatRune('x', maxBanReasonRunes+1)); err == nil {
		t.Fatal("want too long")
	}
}

func TestBanAuditExtra(t *testing.T) {
	t.Parallel()
	if got := BanAuditExtra("", true, false); got != "hidePosts=true hideComments=false" {
		t.Fatalf("got %q", got)
	}
	if got := BanAuditExtra("辱骂", true, true); got != "辱骂 | hidePosts=true hideComments=true" {
		t.Fatalf("got %q", got)
	}
}

func repeatRune(r rune, n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = r
	}
	return string(b)
}
