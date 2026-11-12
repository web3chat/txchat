package service

import "testing"

func TestNewUsersFromSnapshots(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		today        int64
		yesterday    int64
		hasToday     bool
		hasYesterday bool
		want         int64
	}{
		{name: "增长", today: 2300, yesterday: 2239, hasToday: true, hasYesterday: true, want: 61},
		{name: "持平", today: 2239, yesterday: 2239, hasToday: true, hasYesterday: true, want: 0},
		{name: "缺今天", today: 0, yesterday: 2239, hasToday: false, hasYesterday: true, want: 0},
		{name: "缺昨天", today: 2239, yesterday: 0, hasToday: true, hasYesterday: false, want: 0},
		{name: "回退", today: 2200, yesterday: 2239, hasToday: true, hasYesterday: true, want: 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newUsersFromSnapshots(tt.today, tt.yesterday, tt.hasToday, tt.hasYesterday)
			if got != tt.want {
				t.Fatalf("newUsersFromSnapshots() = %d, want %d", got, tt.want)
			}
		})
	}
}
