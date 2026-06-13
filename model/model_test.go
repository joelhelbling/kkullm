package model

import "testing"

// blocked is now an orthogonal flag, not a status. It must be absent from the
// status enums and the transition map entirely.

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

func TestBlockedRemovedFromTransitions(t *testing.T) {
	if _, ok := ValidTransitions["blocked"]; ok {
		t.Error("ValidTransitions still has a \"blocked\" source key; it must be removed")
	}
	for from, targets := range ValidTransitions {
		if targets["blocked"] {
			t.Errorf("ValidTransitions[%q] still lists \"blocked\" as a target; it must be removed", from)
		}
	}
}

func TestCannotTransitionToBlocked(t *testing.T) {
	if CanTransition("todo", "blocked") {
		t.Error("CanTransition(\"todo\", \"blocked\") = true, want false")
	}
	if CanTransition("in_flight", "blocked") {
		t.Error("CanTransition(\"in_flight\", \"blocked\") = true, want false")
	}
}

func TestCoreTransitionsStillWork(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{"considering", "todo", true},
		{"todo", "in_flight", true},
		{"in_flight", "completed", true},
		{"todo", "tabled", true},
		{"considering", "in_flight", false},
	}
	for _, c := range cases {
		if got := CanTransition(c.from, c.to); got != c.want {
			t.Errorf("CanTransition(%q, %q) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}
