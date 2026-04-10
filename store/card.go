package store

import (
	"fmt"
	"strings"

	"github.com/joelhelbling/kkullm/model"
)

type CardCreateParams struct {
	Title     string
	Body      string
	Status    string
	ProjectID int
	Assignees []string
	Tags      []string
	Relations []model.CardRelation
}

type CardListParams struct {
	Project  string
	Assignee string
	Status   string
	Tag      string
}

type CardUpdateParams struct {
	Title     *string
	Body      *string
	Status    *string
	Assignees []string
	Tags      []string
	Relations []model.CardRelation
}

func (s *Store) CreateCard(p CardCreateParams) (*model.Card, error) {
	if p.Status == "" {
		p.Status = "considering"
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO cards (title, body, status, project_id) VALUES (?, ?, ?, ?)",
		p.Title, p.Body, p.Status, p.ProjectID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert card: %w", err)
	}

	cardID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	for _, assignee := range p.Assignees {
		_, err := tx.Exec(`
			INSERT INTO card_assignees (card_id, agent_id)
			SELECT ?, id FROM agents WHERE name = ?
		`, cardID, assignee)
		if err != nil {
			return nil, fmt.Errorf("insert assignee %q: %w", assignee, err)
		}
	}

	for _, tag := range p.Tags {
		_, err := tx.Exec(
			"INSERT INTO card_tags (card_id, tag) VALUES (?, ?)",
			cardID, tag,
		)
		if err != nil {
			return nil, fmt.Errorf("insert tag %q: %w", tag, err)
		}
	}

	for _, rel := range p.Relations {
		_, err := tx.Exec(
			"INSERT OR IGNORE INTO card_relations (card_id, related_card_id, relation_type) VALUES (?, ?, ?)",
			cardID, rel.RelatedCardID, rel.RelationType,
		)
		if err != nil {
			return nil, fmt.Errorf("insert relation: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return s.GetCard(int(cardID))
}

func (s *Store) GetCard(id int) (*model.Card, error) {
	c := &model.Card{}
	err := s.db.QueryRow(`
		SELECT c.id, c.title, COALESCE(c.body, ''), c.status, c.project_id, p.name,
		       (SELECT COUNT(*) FROM comments WHERE card_id = c.id),
		       c.created_at, c.updated_at
		FROM cards c JOIN projects p ON c.project_id = p.id
		WHERE c.id = ?
	`, id).Scan(
		&c.ID, &c.Title, &c.Body, &c.Status, &c.ProjectID, &c.Project,
		&c.CommentCount, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get card %d: %w", id, err)
	}

	assignees, err := s.loadCardAssignees(id)
	if err != nil {
		return nil, err
	}
	c.Assignees = assignees

	tags, err := s.loadCardTags(id)
	if err != nil {
		return nil, err
	}
	c.Tags = tags

	relations, err := s.loadCardRelations(id)
	if err != nil {
		return nil, err
	}
	c.Relations = relations

	return c, nil
}

func (s *Store) loadCardAssignees(cardID int) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT a.name FROM agents a
		JOIN card_assignees ca ON a.id = ca.agent_id
		WHERE ca.card_id = ?
		ORDER BY a.name
	`, cardID)
	if err != nil {
		return nil, fmt.Errorf("load assignees for card %d: %w", cardID, err)
	}
	defer rows.Close()

	var assignees []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan assignee: %w", err)
		}
		assignees = append(assignees, name)
	}
	return assignees, rows.Err()
}

func (s *Store) loadCardTags(cardID int) ([]string, error) {
	rows, err := s.db.Query(
		"SELECT tag FROM card_tags WHERE card_id = ? ORDER BY tag",
		cardID,
	)
	if err != nil {
		return nil, fmt.Errorf("load tags for card %d: %w", cardID, err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (s *Store) loadCardRelations(cardID int) ([]model.CardRelation, error) {
	rows, err := s.db.Query(
		"SELECT related_card_id, relation_type FROM card_relations WHERE card_id = ?",
		cardID,
	)
	if err != nil {
		return nil, fmt.Errorf("load relations for card %d: %w", cardID, err)
	}
	defer rows.Close()

	var relations []model.CardRelation
	for rows.Next() {
		var rel model.CardRelation
		if err := rows.Scan(&rel.RelatedCardID, &rel.RelationType); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		relations = append(relations, rel)
	}
	return relations, rows.Err()
}

func (s *Store) ListCards(params CardListParams) ([]model.Card, error) {
	query := `
		SELECT DISTINCT c.id, c.title, COALESCE(c.body, ''), c.status, c.project_id, p.name,
		       (SELECT COUNT(*) FROM comments WHERE card_id = c.id),
		       c.created_at, c.updated_at
		FROM cards c
		JOIN projects p ON c.project_id = p.id
	`
	var args []any
	var conditions []string

	if params.Assignee != "" {
		query += " JOIN card_assignees ca ON c.id = ca.card_id JOIN agents a ON ca.agent_id = a.id"
		conditions = append(conditions, "a.name = ?")
		args = append(args, params.Assignee)
	}

	if params.Tag != "" {
		query += " JOIN card_tags ct ON c.id = ct.card_id"
		conditions = append(conditions, "ct.tag = ?")
		args = append(args, params.Tag)
	}

	if params.Project != "" {
		conditions = append(conditions, "p.name = ?")
		args = append(args, params.Project)
	}

	if params.Status != "" {
		statuses := strings.Split(params.Status, ",")
		placeholders := make([]string, len(statuses))
		for i, st := range statuses {
			placeholders[i] = "?"
			args = append(args, strings.TrimSpace(st))
		}
		conditions = append(conditions, "c.status IN ("+strings.Join(placeholders, ", ")+")")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY c.id"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	defer rows.Close()

	var cards []model.Card
	for rows.Next() {
		var c model.Card
		if err := rows.Scan(
			&c.ID, &c.Title, &c.Body, &c.Status, &c.ProjectID, &c.Project,
			&c.CommentCount, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan card: %w", err)
		}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range cards {
		assignees, err := s.loadCardAssignees(cards[i].ID)
		if err != nil {
			return nil, err
		}
		cards[i].Assignees = assignees

		tags, err := s.loadCardTags(cards[i].ID)
		if err != nil {
			return nil, err
		}
		cards[i].Tags = tags
	}

	return cards, nil
}

func (s *Store) UpdateCard(id int, p CardUpdateParams) (*model.Card, error) {
	// Validate status transition if status is changing
	if p.Status != nil {
		current := &struct{ Status string }{}
		err := s.db.QueryRow("SELECT status FROM cards WHERE id = ?", id).Scan(&current.Status)
		if err != nil {
			return nil, fmt.Errorf("get current status for card %d: %w", id, err)
		}
		if current.Status != *p.Status {
			if !model.CanTransition(current.Status, *p.Status) {
				allowed := model.AllowedTransitions(current.Status)
				return nil, fmt.Errorf(
					"invalid status transition %q -> %q; allowed transitions from %q: %v",
					current.Status, *p.Status, current.Status, allowed,
				)
			}
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Build dynamic SET clause
	var setClauses []string
	var setArgs []any

	if p.Title != nil {
		setClauses = append(setClauses, "title = ?")
		setArgs = append(setArgs, *p.Title)
	}
	if p.Body != nil {
		setClauses = append(setClauses, "body = ?")
		setArgs = append(setArgs, *p.Body)
	}
	if p.Status != nil {
		setClauses = append(setClauses, "status = ?")
		setArgs = append(setArgs, *p.Status)
	}

	if len(setClauses) > 0 {
		setClauses = append(setClauses, "updated_at = datetime('now')")
		query := "UPDATE cards SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
		setArgs = append(setArgs, id)
		if _, err := tx.Exec(query, setArgs...); err != nil {
			return nil, fmt.Errorf("update card %d: %w", id, err)
		}
	}

	// Replace assignees if provided
	if p.Assignees != nil {
		if _, err := tx.Exec("DELETE FROM card_assignees WHERE card_id = ?", id); err != nil {
			return nil, fmt.Errorf("delete assignees: %w", err)
		}
		for _, assignee := range p.Assignees {
			_, err := tx.Exec(`
				INSERT INTO card_assignees (card_id, agent_id)
				SELECT ?, id FROM agents WHERE name = ?
			`, id, assignee)
			if err != nil {
				return nil, fmt.Errorf("insert assignee %q: %w", assignee, err)
			}
		}
	}

	// Replace tags if provided
	if p.Tags != nil {
		if _, err := tx.Exec("DELETE FROM card_tags WHERE card_id = ?", id); err != nil {
			return nil, fmt.Errorf("delete tags: %w", err)
		}
		for _, tag := range p.Tags {
			_, err := tx.Exec(
				"INSERT INTO card_tags (card_id, tag) VALUES (?, ?)",
				id, tag,
			)
			if err != nil {
				return nil, fmt.Errorf("insert tag %q: %w", tag, err)
			}
		}
	}

	// Append relations (INSERT OR IGNORE)
	for _, rel := range p.Relations {
		_, err := tx.Exec(
			"INSERT OR IGNORE INTO card_relations (card_id, related_card_id, relation_type) VALUES (?, ?, ?)",
			id, rel.RelatedCardID, rel.RelationType,
		)
		if err != nil {
			return nil, fmt.Errorf("insert relation: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return s.GetCard(id)
}

func (s *Store) DeleteCard(id int) error {
	_, err := s.db.Exec("DELETE FROM cards WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete card %d: %w", id, err)
	}
	return nil
}
