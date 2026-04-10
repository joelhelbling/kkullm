package store

import (
	"fmt"

	"github.com/joelhelbling/kkullm/model"
)

func (s *Store) CreateAgent(name string, projectID int, bio string) (*model.Agent, error) {
	result, err := s.db.Exec(
		"INSERT INTO agents (name, project_id, bio) VALUES (?, ?, ?)",
		name, projectID, bio,
	)
	if err != nil {
		return nil, fmt.Errorf("insert agent: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return s.GetAgent(int(id))
}

func (s *Store) GetAgent(id int) (*model.Agent, error) {
	a := &model.Agent{}
	err := s.db.QueryRow(`
		SELECT a.id, a.name, a.project_id, p.name, COALESCE(a.bio, ''), a.created_at, a.updated_at
		FROM agents a JOIN projects p ON a.project_id = p.id
		WHERE a.id = ?
	`, id).Scan(&a.ID, &a.Name, &a.ProjectID, &a.Project, &a.Bio, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get agent %d: %w", id, err)
	}
	return a, nil
}

func (s *Store) GetAgentByName(name string) (*model.Agent, error) {
	a := &model.Agent{}
	err := s.db.QueryRow(`
		SELECT a.id, a.name, a.project_id, p.name, COALESCE(a.bio, ''), a.created_at, a.updated_at
		FROM agents a JOIN projects p ON a.project_id = p.id
		WHERE a.name = ?
	`, name).Scan(&a.ID, &a.Name, &a.ProjectID, &a.Project, &a.Bio, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get agent %q: %w", name, err)
	}
	return a, nil
}

func (s *Store) ListAgents(projectName string) ([]model.Agent, error) {
	query := `
		SELECT a.id, a.name, a.project_id, p.name, COALESCE(a.bio, ''), a.created_at, a.updated_at
		FROM agents a JOIN projects p ON a.project_id = p.id
	`
	var args []any
	if projectName != "" {
		query += " WHERE p.name = ?"
		args = append(args, projectName)
	}
	query += " ORDER BY a.name"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []model.Agent
	for rows.Next() {
		var a model.Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.ProjectID, &a.Project, &a.Bio, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}
