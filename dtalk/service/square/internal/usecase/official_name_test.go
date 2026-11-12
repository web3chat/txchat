package usecase

import (
	"strings"
	"testing"
)

func TestNormalizePersonaName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "trim", in: "  谈信活动  ", want: "谈信活动"},
		{name: "empty", in: "   ", wantErr: true},
		{name: "too long", in: "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizePersonaName(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q", got)
			}
		})
	}
}

func TestNormalizePersonaIntro(t *testing.T) {
	t.Parallel()
	got, err := normalizePersonaIntro("  广场公告  ")
	if err != nil || got != "广场公告" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := normalizePersonaIntro(""); err != nil {
		t.Fatal(err)
	}
	if _, err := normalizePersonaIntro(strings.Repeat("字", 200)); err != nil {
		t.Fatal(err)
	}
	if _, err := normalizePersonaIntro(strings.Repeat("字", 201)); err == nil {
		t.Fatal("expected error")
	}
}

func TestOfficialNameDeleteBlock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		names   int
		posts   int64
		wantErr bool
	}{
		{name: "last name", names: 1, posts: 0, wantErr: true},
		{name: "has posts", names: 2, posts: 1, wantErr: true},
		{name: "empty extra", names: 2, posts: 0, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := officialNameDeleteBlock(tt.names, tt.posts)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}
