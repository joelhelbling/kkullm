package cmd

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/db"
	"github.com/joelhelbling/kkullm/model"
	"github.com/joelhelbling/kkullm/store"
)

// TestResolveBlockedFlags exercises the pure flag-combination validation that
// `card update` uses for --blocked / --unblocked / --reason.
func TestResolveBlockedFlags(t *testing.T) {
	cases := []struct {
		name             string
		blocked          bool
		unblocked        bool
		reason           string
		wantErr          bool
		wantBlockedPtr   *bool
		wantCommentKind  string
		wantHasBlockSet  bool // whether a blocked value should be set at all
		wantPostsComment bool
	}{
		{name: "neither", wantHasBlockSet: false},
		{name: "blocked only", blocked: true, wantHasBlockSet: true, wantPostsComment: false},
		{name: "unblocked only", unblocked: true, wantHasBlockSet: true, wantPostsComment: false},
		{name: "blocked with reason", blocked: true, reason: "waiting", wantHasBlockSet: true, wantPostsComment: true, wantCommentKind: "block"},
		{name: "unblocked with reason", unblocked: true, reason: "resolved", wantHasBlockSet: true, wantPostsComment: true, wantCommentKind: "unblock"},
		{name: "both flags errors", blocked: true, unblocked: true, wantErr: true},
		{name: "reason without flag errors", reason: "why", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := resolveBlockedFlags(c.blocked, c.unblocked, c.reason)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (res=%+v)", res)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.wantHasBlockSet {
				if res.BlockedValue == nil {
					t.Fatalf("expected BlockedValue to be set")
				}
				wantBlocked := c.blocked
				if *res.BlockedValue != wantBlocked {
					t.Errorf("BlockedValue = %v, want %v", *res.BlockedValue, wantBlocked)
				}
			} else if res.BlockedValue != nil {
				t.Errorf("expected BlockedValue nil, got %v", *res.BlockedValue)
			}
			if c.wantPostsComment {
				if res.CommentKind != c.wantCommentKind {
					t.Errorf("CommentKind = %q, want %q", res.CommentKind, c.wantCommentKind)
				}
				if res.CommentBody != c.reason {
					t.Errorf("CommentBody = %q, want %q", res.CommentBody, c.reason)
				}
			} else if res.CommentKind != "" {
				t.Errorf("expected no comment, got kind %q", res.CommentKind)
			}
		})
	}
}

// --- integration: card update --blocked/--unblocked against a live server ---

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}
	if err := db.Seed(database); err != nil {
		t.Fatalf("db.Seed: %v", err)
	}
	s := store.New(database)
	srv := api.NewServer(s)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, s
}

// resetCardFlags restores the package-level cobra flag state mutated by Execute.
func resetCardFlags() {
	cardUpdateBlocked = false
	cardUpdateUnblocked = false
	cardUpdateReason = ""
	cardUpdateTitle = ""
	cardUpdateBody = ""
	cardUpdateStatus = ""
	cardUpdateAssignees = nil
	cardUpdateTags = nil
	cardUpdateBlockedBy = nil
	cardUpdateBelongsTo = nil
	cardUpdateInterestedIn = nil
}

func runRoot(t *testing.T, args ...string) error {
	t.Helper()
	resetCardFlags()
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

func TestCardUpdateBlockedWithReason(t *testing.T) {
	ts, s := newTestServer(t)
	proj, err := s.CreateProject("p", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	agent := mustAgent(t, s, "worker", proj.ID)
	card, err := s.CreateCard(store.CardCreateParams{
		Title: "task", ProjectID: proj.ID, Status: "todo", Assignees: []string{agent.Name},
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	err = runRoot(t, "card", "update", itoa(card.ID),
		"--blocked", "--reason", "waiting on upstream",
		"--server", ts.URL, "--as", "worker")
	if err != nil {
		t.Fatalf("card update --blocked: %v", err)
	}

	got, err := s.GetCard(card.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if !got.Blocked {
		t.Error("expected card blocked after --blocked")
	}
	if got.Status != "todo" {
		t.Errorf("status = %q, want unchanged \"todo\"", got.Status)
	}
	if len(got.Assignees) != 1 || got.Assignees[0] != "worker" {
		t.Errorf("assignees = %v, want unchanged [worker]", got.Assignees)
	}

	comments, err := s.ListComments(card.ID)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1 block comment", len(comments))
	}
	if comments[0].Kind != "block" {
		t.Errorf("comment kind = %q, want \"block\"", comments[0].Kind)
	}
	if comments[0].Body != "waiting on upstream" {
		t.Errorf("comment body = %q, want \"waiting on upstream\"", comments[0].Body)
	}
	if comments[0].Agent != "worker" {
		t.Errorf("comment author = %q, want \"worker\"", comments[0].Agent)
	}
}

func TestCardUpdateUnblockedClearsFlag(t *testing.T) {
	ts, s := newTestServer(t)
	proj, _ := s.CreateProject("p", "")
	mustAgent(t, s, "worker", proj.ID)
	card, _ := s.CreateCard(store.CardCreateParams{Title: "task", ProjectID: proj.ID, Status: "todo"})
	if _, err := s.UpdateCard(card.ID, store.CardUpdateParams{Blocked: boolFlagPtr(true)}); err != nil {
		t.Fatalf("pre-block: %v", err)
	}

	err := runRoot(t, "card", "update", itoa(card.ID),
		"--unblocked", "--server", ts.URL, "--as", "worker")
	if err != nil {
		t.Fatalf("card update --unblocked: %v", err)
	}

	got, _ := s.GetCard(card.ID)
	if got.Blocked {
		t.Error("expected card unblocked after --unblocked")
	}
}

func TestCardUpdateBlockedAndUnblockedErrors(t *testing.T) {
	ts, s := newTestServer(t)
	proj, _ := s.CreateProject("p", "")
	mustAgent(t, s, "worker", proj.ID)
	card, _ := s.CreateCard(store.CardCreateParams{Title: "task", ProjectID: proj.ID, Status: "todo"})

	err := runRoot(t, "card", "update", itoa(card.ID),
		"--blocked", "--unblocked", "--server", ts.URL, "--as", "worker")
	if err == nil {
		t.Fatal("expected error when --blocked and --unblocked both given")
	}
}

func TestCardUpdateReasonWithoutFlagErrors(t *testing.T) {
	ts, s := newTestServer(t)
	proj, _ := s.CreateProject("p", "")
	mustAgent(t, s, "worker", proj.ID)
	card, _ := s.CreateCard(store.CardCreateParams{Title: "task", ProjectID: proj.ID, Status: "todo"})

	err := runRoot(t, "card", "update", itoa(card.ID),
		"--reason", "why", "--server", ts.URL, "--as", "worker")
	if err == nil {
		t.Fatal("expected error when --reason given without --blocked/--unblocked")
	}
}

// --- helpers ---

func boolFlagPtr(b bool) *bool { return &b }

func itoa(i int) string { return strconv.Itoa(i) }

func mustAgent(t *testing.T, s *store.Store, name string, projectID int) *model.Agent {
	t.Helper()
	a, err := s.CreateAgent(name, projectID, "")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	return a
}
