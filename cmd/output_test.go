package cmd

import "testing"

func TestApplyLimit(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	got, truncated := applyLimit(items, 0)
	if truncated || len(got) != 5 {
		t.Errorf("applyLimit(5 items, 0) = %v, %v; want all, false", got, truncated)
	}

	got, truncated = applyLimit(items, 10)
	if truncated || len(got) != 5 {
		t.Errorf("applyLimit(5 items, 10) = %v, %v; want all, false", got, truncated)
	}

	got, truncated = applyLimit(items, 3)
	if !truncated || len(got) != 3 {
		t.Errorf("applyLimit(5 items, 3) = %v, %v; want 3 items, true", got, truncated)
	}

	got, truncated = applyLimit(items, 5)
	if truncated || len(got) != 5 {
		t.Errorf("applyLimit(5 items, 5) = %v, %v; want all, false", got, truncated)
	}

	got, truncated = applyLimit(items, -1)
	if truncated || len(got) != 5 {
		t.Errorf("applyLimit(5 items, -1) = %v, %v; want all, false", got, truncated)
	}
}
