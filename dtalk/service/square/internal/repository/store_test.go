package repository

import "testing"

func TestSplitSQLStatements(t *testing.T) {
	t.Parallel()
	raw := `-- comment
CREATE TABLE IF NOT EXISTS square_post (
    id BIGSERIAL PRIMARY KEY,
    content TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_square_post_kind ON square_post (author_kind);
ALTER TABLE square_post ADD COLUMN IF NOT EXISTS title VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE square_post ADD COLUMN IF NOT EXISTS intro TEXT NOT NULL DEFAULT '';
`
	got := splitSQLStatements(raw)
	if len(got) != 4 {
		t.Fatalf("len=%d got=%v", len(got), got)
	}
	if got[0][:12] != "CREATE TABLE" {
		t.Fatalf("stmt0=%s", got[0])
	}
	if got[2] != `ALTER TABLE square_post ADD COLUMN IF NOT EXISTS title VARCHAR(64) NOT NULL DEFAULT ''` {
		t.Fatalf("stmt2=%s", got[2])
	}
	if got[3] != `ALTER TABLE square_post ADD COLUMN IF NOT EXISTS intro TEXT NOT NULL DEFAULT ''` {
		t.Fatalf("stmt3=%s", got[3])
	}
}
