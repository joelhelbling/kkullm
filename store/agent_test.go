package store

import (
	"testing"
)

func TestCreateAndListAgents(t *testing.T) {
	s := setupTestDB(t)

	agents, err := s.ListAgents("")
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1 (user)", len(agents))
	}
	if agents[0].Name != "user" {
		t.Errorf("seeded agent name = %q, want 'user'", agents[0].Name)
	}

	proj, err := s.CreateProject("acme", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	agent, err := s.CreateAgent("dev-agent", proj.ID, "Writes Go code")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if agent.Name != "dev-agent" {
		t.Errorf("name = %q, want 'dev-agent'", agent.Name)
	}
	if agent.Bio != "Writes Go code" {
		t.Errorf("bio = %q, want 'Writes Go code'", agent.Bio)
	}

	agents, err = s.ListAgents("")
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("got %d agents, want 2", len(agents))
	}

	agents, err = s.ListAgents("acme")
	if err != nil {
		t.Fatalf("ListAgents(acme): %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("got %d agents for acme, want 1", len(agents))
	}
}

func TestGetAgentByName(t *testing.T) {
	s := setupTestDB(t)

	agent, err := s.GetAgentByName("user")
	if err != nil {
		t.Fatalf("GetAgentByName: %v", err)
	}
	if agent.Name != "user" {
		t.Errorf("name = %q, want 'user'", agent.Name)
	}
	if agent.Project != "orchestration" {
		t.Errorf("project = %q, want 'orchestration'", agent.Project)
	}
}
