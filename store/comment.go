package store

import (
	"fmt"

	"github.com/joelhelbling/kkullm/model"
)

func (s *Store) CreateComment(cardID, agentID int, body string) (*model.Comment, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var agentName string
	if err := tx.QueryRow(`SELECT name FROM agents WHERE id = ?`, agentID).Scan(&agentName); err != nil {
		return nil, fmt.Errorf("lookup agent %d: %w", agentID, err)
	}

	result, err := tx.Exec(
		"INSERT INTO comments (card_id, agent_id, author_name, body) VALUES (?, ?, ?, ?)",
		cardID, agentID, agentName, body,
	)
	if err != nil {
		return nil, fmt.Errorf("insert comment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	c := &model.Comment{}
	err = s.db.QueryRow(`
		SELECT c.id, c.card_id, COALESCE(c.agent_id, 0), COALESCE(a.name, c.author_name, ''), c.body, c.created_at
		FROM comments c LEFT JOIN agents a ON c.agent_id = a.id
		WHERE c.id = ?
	`, id).Scan(&c.ID, &c.CardID, &c.AgentID, &c.Agent, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get comment %d: %w", id, err)
	}
	return c, nil
}

func (s *Store) ListComments(cardID int) ([]model.Comment, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.card_id, COALESCE(c.agent_id, 0), COALESCE(a.name, c.author_name, ''), c.body, c.created_at
		FROM comments c LEFT JOIN agents a ON c.agent_id = a.id
		WHERE c.card_id = ?
		ORDER BY c.created_at ASC
	`, cardID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var comments []model.Comment
	for rows.Next() {
		var c model.Comment
		if err := rows.Scan(&c.ID, &c.CardID, &c.AgentID, &c.Agent, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}
