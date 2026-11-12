package dao

import "testing"

func TestContentFingerprintStable(t *testing.T) {
	a := ContentFingerprint("dtalk", 123, "seq-1", "from", "to", 1, 1000, "hello", "src", "ref")
	b := ContentFingerprint("dtalk", 123, "seq-1", "from", "to", 1, 1000, "hello", "src", "ref")
	if a == "" || a != b {
		t.Fatalf("fingerprint unstable: %s vs %s", a, b)
	}
	c := ContentFingerprint("dtalk", 123, "seq-1", "from", "to", 1, 1000, "hello2", "src", "ref")
	if a == c {
		t.Fatalf("different content should change fingerprint")
	}
	if len(a) != 64 {
		t.Fatalf("sha256 hex length want 64 got %d", len(a))
	}
}
