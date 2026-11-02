package cowbook

import "testing"

// TestZeroValueSpace checks that a Space declared without values starts
// with an unset ID. Step 1.1 replaces this with tests for NewSpace.
func TestZeroValueSpace(t *testing.T) {
	var s Space
	if s.ID != 0 {
		t.Errorf("zero value Space has ID = %d, want 0", s.ID)
	}
}
