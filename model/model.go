package model

import "time"

type Project struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Agent struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	ProjectID int       `json:"project_id"`
	Project   string    `json:"project,omitempty"`
	Bio       string    `json:"bio,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Card struct {
	ID           int            `json:"id"`
	Title        string         `json:"title"`
	Body         string         `json:"body,omitempty"`
	Status       string         `json:"status"`
	Blocked      bool           `json:"blocked"`
	ProjectID    int            `json:"project_id"`
	Project      string         `json:"project,omitempty"`
	Assignees    []string       `json:"assignees,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Relations    []CardRelation `json:"relations,omitempty"`
	CommentCount int            `json:"comment_count,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type CardRelation struct {
	RelatedCardID int    `json:"related_card_id"`
	RelationType  string `json:"relation_type"`
}

type Comment struct {
	ID        int       `json:"id"`
	CardID    int       `json:"card_id"`
	AgentID   int       `json:"agent_id"`
	Agent     string    `json:"agent,omitempty"`
	Body      string    `json:"body"`
	Kind      string    `json:"kind,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CardEvent is one entry in a card's append-only audit trail. v1 records
// status transitions (event_type "status_changed") and assignee changes
// ("assignee_added"/"assignee_removed"). The from/to fields carry the relevant
// values per event type; the schema is generic so new event types need no
// migration.
type CardEvent struct {
	ID        int       `json:"id"`
	CardID    int       `json:"card_id"`
	Actor     string    `json:"actor,omitempty"`
	EventType string    `json:"event_type"`
	FromValue string    `json:"from_value,omitempty"`
	ToValue   string    `json:"to_value,omitempty"`
	Forced    bool      `json:"forced"`
	CreatedAt time.Time `json:"created_at"`
}

type ProjectAsset struct {
	ID          int       `json:"id"`
	ProjectID   int       `json:"project_id"`
	Project     string    `json:"project,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	URL         string    `json:"url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

var ValidStatuses = map[string]bool{
	"considering": true,
	"todo":        true,
	"in_flight":   true,
	"completed":   true,
	"tabled":      true,
}

// AllStatuses lists every valid card status in canonical display order
// (left-to-right column order on the board).
var AllStatuses = []string{
	"considering",
	"todo",
	"in_flight",
	"completed",
	"tabled",
}

var ValidTransitions = map[string]map[string]bool{
	"considering": {"todo": true, "tabled": true},
	"todo":        {"in_flight": true, "tabled": true},
	"in_flight":   {"completed": true, "tabled": true},
	"completed":   {"in_flight": true, "tabled": true},
	"tabled":      {"considering": true, "todo": true},
}

// ValidCommentKinds is the set of accepted Comment.Kind values. "" is a normal
// comment; "block"/"unblock" tag the note left when a card's blocked flag is
// toggled, so the "why" lives in the card timeline.
var ValidCommentKinds = map[string]bool{
	"":        true,
	"block":   true,
	"unblock": true,
}

// ValidCommentKind reports whether k is an accepted comment kind.
func ValidCommentKind(k string) bool {
	return ValidCommentKinds[k]
}

func CanTransition(from, to string) bool {
	targets, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	return targets[to]
}

func AllowedTransitions(from string) []string {
	targets, ok := ValidTransitions[from]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(targets))
	for s := range targets {
		result = append(result, s)
	}
	return result
}
