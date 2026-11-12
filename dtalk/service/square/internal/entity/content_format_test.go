package entity

import "testing"

func TestPostContentFormat(t *testing.T) {
	t.Parallel()
	if (*Post)(nil).ContentFormat() != "plain" {
		t.Fatal("nil post")
	}
	if (&Post{AuthorAddr: "1abc"}).ContentFormat() != "plain" {
		t.Fatal("personal")
	}
	if (&Post{AuthorAddr: PlatformOfficialAddr}).ContentFormat() != "markdown" {
		t.Fatal("platform official should be markdown")
	}
}
