package usecase

import "testing"

func TestNormalizeBatchIDs(t *testing.T) {
	t.Parallel()
	got, err := normalizeBatchIDs([]int64{0, -1, 3, 3, 2, 1}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != 3 || got[1] != 2 || got[2] != 1 {
		t.Fatalf("got %v", got)
	}
	if _, err := normalizeBatchIDs(nil, 100); err == nil {
		t.Fatal("empty should fail")
	}
	tooMany := make([]int64, 101)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}
	if _, err := normalizeBatchIDs(tooMany, 100); err == nil {
		t.Fatal("over max should fail")
	}
}
