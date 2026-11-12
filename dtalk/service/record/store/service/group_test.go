package service

import "testing"

func TestGroupContentNewlyInserted(t *testing.T) {
	t.Parallel()
	if !groupContentNewlyInserted(1) {
		t.Fatal("rowsAffected=1 should be new insert")
	}
	if groupContentNewlyInserted(0) {
		t.Fatal("rowsAffected=0 is duplicate, skip relations")
	}
	if groupContentNewlyInserted(2) {
		t.Fatal("rowsAffected=2 is duplicate update, skip relations")
	}
}
