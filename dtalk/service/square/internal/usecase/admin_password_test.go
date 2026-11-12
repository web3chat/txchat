package usecase

import "testing"

func TestValidateAdminPassword(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pass    string
		wantErr bool
	}{
		{name: "empty", pass: "", wantErr: true},
		{name: "11", pass: "12345678901", wantErr: true},
		{name: "12", pass: "123456789012", wantErr: false},
		{name: "longer", pass: "change-me-please-12", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateAdminPassword(tt.pass)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestResetThisBootstrap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		reset bool
		root  string
		user  string
		want  bool
	}{
		{name: "off", reset: false, root: "admin", user: "admin", want: false},
		{name: "root", reset: true, root: "admin", user: "admin", want: true},
		{name: "other account", reset: true, root: "admin", user: "operator", want: false},
		{name: "blank root", reset: true, root: "  ", user: "admin", want: false},
		{name: "trimmed root", reset: true, root: " admin ", user: "admin", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resetThisBootstrap(tt.reset, tt.root, tt.user); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
