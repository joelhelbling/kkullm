package model

import "testing"

// blocked is now an orthogonal flag, not a status. It must be absent from the
// status enums entirely.

func TestBlockedRemovedFromValidStatuses(t *testing.T) {
	if ValidStatuses["blocked"] {
		t.Error("ValidStatuses still contains \"blocked\"; it must be removed (blocked is a flag now)")
	}
}

func TestBlockedRemovedFromAllStatuses(t *testing.T) {
	for _, s := range AllStatuses {
		if s == "blocked" {
			t.Error("AllStatuses still contains \"blocked\"; it must be removed")
		}
	}
}
