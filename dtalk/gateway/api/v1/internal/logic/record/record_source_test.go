package record

import "testing"

func TestParseSourceJSON(t *testing.T) {
	t.Parallel()

	if got := parseSourceJSON(""); got != nil {
		t.Fatalf("empty should be nil, got %#v", got)
	}
	if got := parseSourceJSON("{"); got != nil {
		t.Fatalf("invalid json should be nil, got %#v", got)
	}
	got := parseSourceJSON(`{"burn":true}`)
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("want map, got %#v", got)
	}
	if m["burn"] != true {
		t.Fatalf("want burn=true, got %#v", m["burn"])
	}
}
