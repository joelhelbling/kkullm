package store

import (
	"testing"

	"github.com/joelhelbling/kkullm/model"
)

func TestAppendAndListCardEvents(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	card, err := s.CreateCard(CardCreateParams{Title: "audited", ProjectID: proj.ID})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	statusEv, err := s.AppendCardEvent(model.CardEvent{
		CardID:    card.ID,
		Actor:     "worker",
		EventType: "status_changed",
		FromValue: "considering",
		ToValue:   "todo",
	})
	if err != nil {
		t.Fatalf("AppendCardEvent(status): %v", err)
	}
	if statusEv.ID == 0 {
		t.Error("expected non-zero event ID")
	}
	if statusEv.CreatedAt.IsZero() {
		t.Error("expected populated created_at")
	}

	if _, err := s.AppendCardEvent(model.CardEvent{
		CardID:    card.ID,
		Actor:     "", // empty actor must be preserved
		EventType: "assignee_added",
		ToValue:   "alice",
	}); err != nil {
		t.Fatalf("AppendCardEvent(assignee): %v", err)
	}

	events, err := s.ListCardEvents(card.ID)
	if err != nil {
		t.Fatalf("ListCardEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	// Chronological order: status first, assignee second.
	if events[0].EventType != "status_changed" {
		t.Errorf("events[0].EventType = %q, want status_changed", events[0].EventType)
	}
	if events[0].Actor != "worker" {
		t.Errorf("events[0].Actor = %q, want worker", events[0].Actor)
	}
	if events[0].FromValue != "considering" || events[0].ToValue != "todo" {
		t.Errorf("events[0] from/to = %q/%q, want considering/todo", events[0].FromValue, events[0].ToValue)
	}
	if events[1].EventType != "assignee_added" {
		t.Errorf("events[1].EventType = %q, want assignee_added", events[1].EventType)
	}
	if events[1].Actor != "" {
		t.Errorf("events[1].Actor = %q, want empty", events[1].Actor)
	}
	if events[1].ToValue != "alice" {
		t.Errorf("events[1].ToValue = %q, want alice", events[1].ToValue)
	}
}

func TestUpdateCardEmitsStatusEvent(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	card, err := s.CreateCard(CardCreateParams{Title: "c", ProjectID: proj.ID, Status: "considering"})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	if _, err := s.UpdateCard(card.ID, CardUpdateParams{Status: strPtr("todo"), Actor: "mover"}); err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}

	events, err := s.ListCardEvents(card.ID)
	if err != nil {
		t.Fatalf("ListCardEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	e := events[0]
	if e.EventType != "status_changed" || e.FromValue != "considering" || e.ToValue != "todo" {
		t.Errorf("event = %+v, want status_changed considering->todo", e)
	}
	if e.Actor != "mover" {
		t.Errorf("actor = %q, want mover", e.Actor)
	}
}

func TestUpdateCardNoOpStatusEmitsNoEvent(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	card, err := s.CreateCard(CardCreateParams{Title: "c", ProjectID: proj.ID, Status: "considering"})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	// Same status -> no event.
	if _, err := s.UpdateCard(card.ID, CardUpdateParams{Status: strPtr("considering")}); err != nil {
		t.Fatalf("UpdateCard same status: %v", err)
	}
	// Title/body change -> no event.
	if _, err := s.UpdateCard(card.ID, CardUpdateParams{Title: strPtr("renamed"), Body: strPtr("text")}); err != nil {
		t.Fatalf("UpdateCard title/body: %v", err)
	}

	events, err := s.ListCardEvents(card.ID)
	if err != nil {
		t.Fatalf("ListCardEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("got %d events, want 0", len(events))
	}
}

func TestUpdateCardEmitsAssigneeEvents(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	createTestAgent(t, s, "a", proj.ID)
	createTestAgent(t, s, "b", proj.ID)

	card, err := s.CreateCard(CardCreateParams{Title: "c", ProjectID: proj.ID, Assignees: []string{"a"}})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	// [a] -> [a, b] : one assignee_added for b.
	if _, err := s.UpdateCard(card.ID, CardUpdateParams{Assignees: []string{"a", "b"}, Actor: "x"}); err != nil {
		t.Fatalf("UpdateCard add: %v", err)
	}
	events, _ := s.ListCardEvents(card.ID)
	if len(events) != 1 {
		t.Fatalf("after add: got %d events, want 1", len(events))
	}
	if events[0].EventType != "assignee_added" || events[0].ToValue != "b" {
		t.Errorf("add event = %+v, want assignee_added to=b", events[0])
	}

	// [a, b] -> [a] : one assignee_removed for b.
	if _, err := s.UpdateCard(card.ID, CardUpdateParams{Assignees: []string{"a"}, Actor: "x"}); err != nil {
		t.Fatalf("UpdateCard remove: %v", err)
	}
	events, _ = s.ListCardEvents(card.ID)
	if len(events) != 2 {
		t.Fatalf("after remove: got %d events, want 2", len(events))
	}
	rem := events[1]
	if rem.EventType != "assignee_removed" || rem.FromValue != "b" {
		t.Errorf("remove event = %+v, want assignee_removed from=b", rem)
	}
}
