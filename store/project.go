package store

import (
	"database/sql"
	"fmt"

	"github.com/joelhelbling/kkullm/model"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateProject(name, description string) (*model.Project, error) {
	result, err := s.db.Exec(
		"INSERT INTO projects (name, description) VALUES (?, ?)",
		name, description,
	)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return s.GetProject(int(id))
}

func (s *Store) GetProject(id int) (*model.Project, error) {
	p := &model.Project{}
	err := s.db.QueryRow(
		"SELECT id, name, COALESCE(description, ''), created_at, updated_at FROM projects WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project %d: %w", id, err)
	}
	return p, nil
}

func (s *Store) GetProjectByName(name string) (*model.Project, error) {
	p := &model.Project{}
	err := s.db.QueryRow(
		"SELECT id, name, COALESCE(description, ''), created_at, updated_at FROM projects WHERE name = ?", name,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project %q: %w", name, err)
	}
	return p, nil
}

func (s *Store) RenameProject(id int, name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	_, err := s.db.Exec(
		"UPDATE projects SET name = ?, updated_at = datetime('now') WHERE id = ?",
		name, id,
	)
	if err != nil {
		return fmt.Errorf("rename project %d: %w", id, err)
	}
	return nil
}

func (s *Store) UpdateProject(id int, name, description string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	_, err := s.db.Exec(
		"UPDATE projects SET name = ?, description = ?, updated_at = datetime('now') WHERE id = ?",
		name, description, id,
	)
	if err != nil {
		return fmt.Errorf("update project %d: %w", id, err)
	}
	return nil
}

func (s *Store) DeleteProject(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM cards WHERE project_id = ?", id); err != nil {
		return fmt.Errorf("delete cards: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM project_assets WHERE project_id = ?", id); err != nil {
		return fmt.Errorf("delete assets: %w", err)
	}
	if _, err := tx.Exec(
		"UPDATE comments SET agent_id = NULL WHERE agent_id IN (SELECT id FROM agents WHERE project_id = ?)",
		id,
	); err != nil {
		return fmt.Errorf("null cross-project comment refs: %w", err)
	}
	if _, err := tx.Exec(
		"DELETE FROM card_assignees WHERE agent_id IN (SELECT id FROM agents WHERE project_id = ?)",
		id,
	); err != nil {
		return fmt.Errorf("clear cross-project assignees: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM agents WHERE project_id = ?", id); err != nil {
		return fmt.Errorf("delete agents: %w", err)
	}
	res, err := tx.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("project %d not found", id)
	}
	return tx.Commit()
}

func (s *Store) CountCardsForProject(projectID int) (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM cards WHERE project_id = ?", projectID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count cards for project %d: %w", projectID, err)
	}
	return n, nil
}

func (s *Store) CountAgentsForProject(projectID int) (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM agents WHERE project_id = ?", projectID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count agents for project %d: %w", projectID, err)
	}
	return n, nil
}

func (s *Store) ListProjects() ([]model.Project, error) {
	rows, err := s.db.Query("SELECT id, name, COALESCE(description, ''), created_at, updated_at FROM projects ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}
