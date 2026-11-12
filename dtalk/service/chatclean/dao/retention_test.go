package dao

import "testing"

func TestValidIdent(t *testing.T) {
	got, err := validIdent("", "dtalk_record")
	if err != nil || got != "dtalk_record" {
		t.Fatalf("fallback = %q err=%v", got, err)
	}
	if _, err = validIdent("bad-name", "dtalk_record"); err == nil {
		t.Fatal("hyphen should be rejected")
	}
	if _, err = validIdent("dtalk_record", "x"); err != nil {
		t.Fatal(err)
	}
}

func TestCheckTable(t *testing.T) {
	if err := checkTable(PrivateContentTable); err != nil {
		t.Fatal(err)
	}
	if err := checkTable("dtalk_signal_content"); err == nil {
		t.Fatal("signal table must not be deleted")
	}
}

func TestCacheMemberExpired(t *testing.T) {
	raw := []byte(`{"CreateTime":100,"Content":"secret"}`)
	expired, ok := cacheMemberExpired(raw, 101)
	if !ok || !expired {
		t.Fatalf("expired=%v ok=%v", expired, ok)
	}
	expired, ok = cacheMemberExpired(raw, 100)
	if !ok || expired {
		t.Fatalf("equal time expired=%v ok=%v", expired, ok)
	}
	expired, ok = cacheMemberExpired([]byte(`{"CreateTime":0}`), 100)
	if !ok || expired {
		t.Fatalf("zero time expired=%v ok=%v", expired, ok)
	}
	if _, ok = cacheMemberExpired([]byte(`not-json`), 100); ok {
		t.Fatal("bad json should be skipped")
	}
}
