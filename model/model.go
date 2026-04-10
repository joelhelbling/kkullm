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
	"done":        true,
	"tabled":      true,
	"blocked":     true,
}

var ValidTransitions = map[string]map[string]bool{
	"considering": {"todo": true, "tabled": true},
	"todo":        {"in_flight": true, "blocked": true, "tabled": true},
	"in_flight":   {"completed": true, "blocked": true, "tabled": true},
	"completed":   {"done": true, "in_flight": true, "tabled": true},
	"blocked":     {"todo": true, "in_flight": true, "tabled": true},
	"tabled":      {"considering": true, "todo": true},
	"done":        {},
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
