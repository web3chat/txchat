package usecase

import "testing"

func TestNormalizeGateMode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "", want: "open"},
		{in: "open", want: "open"},
		{in: "CLOSED", want: "closed"},
		{in: "limited", want: "limited"},
		{in: "semi", want: "limited"},
		{in: "weird", wantErr: true},
	}
	for _, tc := range cases {
		got, err := normalizeGateMode(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("in=%q want err", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("in=%q err=%v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("in=%q got=%s want=%s", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeModerationLevel(t *testing.T) {
	t.Parallel()
	got, err := normalizeModerationLevel("")
	if err != nil || got != "default" {
		t.Fatalf("empty default got=%s err=%v", got, err)
	}
	got, err = normalizeModerationLevel("strict")
	if err != nil || got != "strict" {
		t.Fatalf("strict got=%s err=%v", got, err)
	}
	got, err = normalizeModerationLevel("all")
	if err != nil || got != "strict" {
		t.Fatalf("all got=%s err=%v", got, err)
	}
	if _, err := normalizeModerationLevel("soft"); err == nil {
		t.Fatal("soft should err")
	}
}
