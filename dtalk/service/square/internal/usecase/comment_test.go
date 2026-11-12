package usecase

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeCommentContent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		wantErr bool
		want    string
	}{
		{in: "", wantErr: true},
		{in: "   ", wantErr: true},
		{in: "  收到  ", want: "收到"},
		{in: strings.Repeat("字", maxCommentRunes), want: strings.Repeat("字", maxCommentRunes)},
		{in: strings.Repeat("字", maxCommentRunes+1), wantErr: true},
	}
	for _, tc := range cases {
		got, err := normalizeCommentContent(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("in=%q runes=%d want err", tc.in, utf8.RuneCountInString(strings.TrimSpace(tc.in)))
			}
			continue
		}
		if err != nil {
			t.Fatalf("in=%q err=%v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("got=%q want=%q", got, tc.want)
		}
	}
}
