package usecase

import "testing"

func TestNearDuplicate(t *testing.T) {
	t.Parallel()
	base := stringsRepeat("广场公告正文", 20)
	changed10 := []rune(base)
	for i := 0; i < len(changed10)/10; i++ {
		changed10[i] = '改'
	}
	changed11 := []rune(base)
	n := len(changed11)/10 + 1
	for i := 0; i < n && i < len(changed11); i++ {
		changed11[i] = '改'
	}
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "完全相同", a: base, b: base, want: true},
		{name: "空白差异仍相同", a: "你好 世界", b: "你好世界", want: true},
		{name: "改动不超过一成", a: base, b: string(changed10), want: true},
		{name: "改动超过一成", a: base, b: string(changed11), want: false},
		{name: "空正文不比较", a: "   ", b: base, want: false},
		{name: "长度差超过一成", a: base, b: base + "完全不同的补充说明内容用来拉开长度差距", want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := nearDuplicate(tc.a, tc.b); got != tc.want {
				t.Fatalf("nearDuplicate()=%v, want %v", got, tc.want)
			}
		})
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
