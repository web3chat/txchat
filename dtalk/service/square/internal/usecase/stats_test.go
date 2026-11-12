package usecase

import "testing"

func TestClampOverviewDays(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want int
	}{
		{0, 7},
		{6, 7},
		{7, 7},
		{14, 14},
		{90, 90},
		{91, 90},
	}
	for _, tc := range cases {
		if got := clampOverviewDays(tc.in); got != tc.want {
			t.Fatalf("clampOverviewDays(%d)=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestClampAuthorAddrs(t *testing.T) {
	t.Parallel()
	got := clampAuthorAddrs([]string{" a ", "", "a", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got=%v", got)
	}
	many := make([]string, 30)
	for i := range many {
		many[i] = string(rune('A'+i%26)) + string(rune('0'+i/26))
	}
	if n := len(clampAuthorAddrs(many)); n != 20 {
		t.Fatalf("cap=%d want 20", n)
	}
}
