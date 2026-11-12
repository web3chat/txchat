package burn

import "testing"

func TestIsBurnFromSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want bool
	}{
		{name: "nil", in: nil, want: false},
		{name: "empty", in: []byte(""), want: false},
		{name: "invalid json", in: []byte("{"), want: false},
		{name: "no burn key", in: []byte(`{"channelType":1}`), want: false},
		{name: "burn false", in: []byte(`{"burn":false}`), want: false},
		{name: "burn true", in: []byte(`{"burn":true}`), want: true},
		{name: "proto json with burn", in: []byte(`{"channelType":0,"burn":true}`), want: true},
		{name: "numeric not bool", in: []byte(`{"burn":1}`), want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsBurnFromSource(tt.in); got != tt.want {
				t.Fatalf("IsBurnFromSource(%s) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
