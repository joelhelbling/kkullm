package store

import (
	"fmt"
	"strings"

	"github.com/joelhelbling/kkullm/model"
)

type AssetListParams struct {
	Project  string
	NameGlob string
	URLGlob  string
}

func (s *Store) CreateAsset(projectID int, name, description, url string) (*model.ProjectAsset, error) {
	result, err := s.db.Exec(
		"INSERT INTO project_assets (project_id, name, description, url) VALUES (?, ?, ?, ?)",
		projectID, name, nilIfEmpty(description), nilIfEmpty(url),
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return s.GetAsset(int(id))
}

func (s *Store) GetAsset(id int) (*model.ProjectAsset, error) {
	a := &model.ProjectAsset{}
	err := s.db.QueryRow(`
		SELECT pa.id, pa.project_id, p.name, pa.name, COALESCE(pa.description, ''),
			COALESCE(pa.url, ''), pa.created_at, pa.updated_at
		FROM project_assets pa
		JOIN projects p ON pa.project_id = p.id
		WHERE pa.id = ?
	`, id).Scan(&a.ID, &a.ProjectID, &a.Project, &a.Name, &a.Description, &a.URL, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get asset %d: %w", id, err)
	}
	return a, nil
}

func (s *Store) ListAssets(params AssetListParams) ([]model.ProjectAsset, error) {
	query := `
		SELECT pa.id, pa.project_id, p.name, pa.name, COALESCE(pa.description, ''),
			COALESCE(pa.url, ''), pa.created_at, pa.updated_at
		FROM project_assets pa
		JOIN projects p ON pa.project_id = p.id
	`
	var conditions []string
	var args []any

	if params.Project != "" {
		conditions = append(conditions, "p.name = ?")
		args = append(args, params.Project)
	}
	if params.NameGlob != "" {
		conditions = append(conditions, "pa.name GLOB ?")
		args = append(args, params.NameGlob)
	}
	if params.URLGlob != "" {
		conditions = append(conditions, "pa.url GLOB ?")
		args = append(args, params.URLGlob)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY pa.name"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	defer rows.Close()

	var assets []model.ProjectAsset
	for rows.Next() {
		var a model.ProjectAsset
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Project, &a.Name, &a.Description,
			&a.URL, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
