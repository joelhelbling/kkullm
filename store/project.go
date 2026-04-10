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
