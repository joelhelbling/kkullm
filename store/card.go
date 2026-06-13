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

	// Actor is the acting-agent identity recorded into the audit trail.
	// Defaults to "" until the CLI/API thread --as/KKULLM_AGENT through (#37).
	Actor string
}

type CardListParams struct {
	Project  string
	Assignee string
	Status   string
	Tag      string

	// Blocked, when non-nil, filters by the orthogonal blocked flag:
	// true returns only blocked cards, false only unblocked. nil = no filter.
	Blocked *bool

	// ArchiveLimit, when > 0, caps the number of 'completed' and 'tabled'
	// cards returned. Each terminal status keeps its N most-recently-updated
	// rows; older ones are considered archived.
	ArchiveLimit int

	// ArchiveView controls which slice of the completed/tabled rows is returned.
	// "" (default) returns the active view: all non-terminal statuses plus the
	// most-recent N of completed/tabled.
	// "archived" returns only the overflow: completed/tabled rows beyond the
	// first N. Has no effect when ArchiveLimit is 0.
	ArchiveView string
}

type CardUpdateParams struct {
	Title     *string
	Body      *string
	Status    *string
	Blocked   *bool
	Assignees []string
	Tags      []string
	Relations []model.CardRelation

	// Force bypasses the status-transition matrix when the status is changing.
	// Status validity (model.ValidStatuses) is still enforced; only the
	// transition rule is skipped. A forced move is recorded as Forced in the
	// audit trail. Available to anyone (#35).
	Force bool

	// Actor is the acting-agent identity recorded into the audit trail.
	// Defaults to "" until the CLI/API thread --as/KKULLM_AGENT through (#37).
	Actor string
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
		SELECT c.id, c.title, COALESCE(c.body, ''), c.status, c.blocked, c.project_id, p.name,
		       (SELECT COUNT(*) FROM comments WHERE card_id = c.id),
		       c.created_at, c.updated_at
		FROM cards c JOIN projects p ON c.project_id = p.id
		WHERE c.id = ?
	`, id).Scan(
		&c.ID, &c.Title, &c.Body, &c.Status, &c.Blocked, &c.ProjectID, &c.Project,
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
	// 'archived' is a slice of completed/tabled only; requesting it without
	// a positive ArchiveLimit asks for "everything past row 0", which is
	// every completed/tabled row — that's not what callers mean. Return
	// empty so the API is unambiguous.
	if params.ArchiveView == "archived" && params.ArchiveLimit <= 0 {
		return []model.Card{}, nil
	}

	baseQuery := `
		SELECT DISTINCT
			c.id AS id, c.title AS title, COALESCE(c.body, '') AS body,
			c.status AS status, c.blocked AS blocked, c.project_id AS project_id, p.name AS project,
			(SELECT COUNT(*) FROM comments WHERE card_id = c.id) AS comment_count,
			c.created_at AS created_at, c.updated_at AS updated_at
		FROM cards c
		JOIN projects p ON c.project_id = p.id
	`
	var args []any
	var conditions []string

	if params.Assignee != "" {
		baseQuery += " JOIN card_assignees ca ON c.id = ca.card_id JOIN agents a ON ca.agent_id = a.id"
		conditions = append(conditions, "a.name = ?")
		args = append(args, params.Assignee)
	}

	if params.Tag != "" {
		baseQuery += " JOIN card_tags ct ON c.id = ct.card_id"
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

	if params.Blocked != nil {
		conditions = append(conditions, "c.blocked = ?")
		if *params.Blocked {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}

	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Wrap the base query in a CTE so the outer SELECT (and its ORDER BY)
	// sees only the explicit column aliases — no ambiguity from joined
	// tables that happen to share column names like 'updated_at'.
	// When ArchiveLimit is set, an inner CTE adds rn per completed/tabled
	// partition so we can keep the top N (active) or skip them (archived).
	var query string
	if params.ArchiveLimit > 0 {
		archiveFilter := "(status NOT IN ('completed','tabled') OR rn <= ?)"
		if params.ArchiveView == "archived" {
			archiveFilter = "(status IN ('completed','tabled') AND rn > ?)"
		}
		query = `
			WITH base AS (` + baseQuery + `),
			ranked AS (
				SELECT base.*,
					CASE WHEN status IN ('completed','tabled')
					     THEN ROW_NUMBER() OVER (
					         PARTITION BY status
					         ORDER BY updated_at DESC, id DESC
					     )
					     ELSE NULL
					END AS rn
				FROM base
			)
			SELECT id, title, body, status, blocked, project_id, project, comment_count, created_at, updated_at
			FROM ranked
			WHERE ` + archiveFilter
		args = append(args, params.ArchiveLimit)
	} else {
		query = `
			WITH base AS (` + baseQuery + `)
			SELECT id, title, body, status, blocked, project_id, project, comment_count, created_at, updated_at
			FROM base
		`
	}

	// Prioritized ordering:
	//   1. in_flight   - most recently updated first (active work surfaces fast)
	//   2. todo        - most recently updated first (placeholder until per-column ordinals exist)
	//   3. considering - by max(created_at, updated_at) DESC (most-recent activity wins)
	//   4. completed   - most recently updated first
	//   5. tabled      - most recently updated first
	// Column names are unqualified so this clause works for both the
	// plain base query and the windowed CTE wrapper.
	query += `
		ORDER BY
			CASE status
				WHEN 'in_flight'   THEN 1
				WHEN 'todo'        THEN 2
				WHEN 'considering' THEN 3
				WHEN 'completed'   THEN 4
				WHEN 'tabled'      THEN 5
				ELSE 99
			END,
			CASE status
				WHEN 'considering' THEN MAX(created_at, updated_at)
				ELSE updated_at
			END DESC,
			id DESC
	`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	defer rows.Close()

	var cards []model.Card
	for rows.Next() {
		var c model.Card
		if err := rows.Scan(
			&c.ID, &c.Title, &c.Body, &c.Status, &c.Blocked, &c.ProjectID, &c.Project,
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

// ListFormerlyAssignedBlockedCards returns cards that are still blocked, are
// NOT currently assigned to agentName, but were previously assigned to that
// agent and reassigned away (an "assignee_removed" audit event names them in
// from_value). These surface in the agent's view so a blocked card taken from
// them while in flight doesn't get lost. Ordered by updated_at desc.
func (s *Store) ListFormerlyAssignedBlockedCards(agentName string) ([]model.Card, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT c.id
		FROM cards c
		JOIN card_events e
			ON e.card_id = c.id
			AND e.event_type = 'assignee_removed'
			AND e.from_value = ?
		WHERE c.blocked = 1
		  AND NOT EXISTS (
		      SELECT 1 FROM card_assignees ca
		      JOIN agents a ON a.id = ca.agent_id
		      WHERE ca.card_id = c.id AND a.name = ?
		  )
		ORDER BY c.updated_at DESC, c.id DESC
	`, agentName, agentName)
	if err != nil {
		return nil, fmt.Errorf("list formerly-assigned blocked cards: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan card id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	cards := make([]model.Card, 0, len(ids))
	for _, id := range ids {
		c, err := s.GetCard(id)
		if err != nil {
			return nil, err
		}
		cards = append(cards, *c)
	}
	return cards, nil
}

func (s *Store) UpdateCard(id int, p CardUpdateParams) (*model.Card, error) {
	// Validate status transition if status is changing. oldStatus is captured
	// for the audit trail (a status_changed event records from->to).
	var oldStatus string
	if p.Status != nil {
		err := s.db.QueryRow("SELECT status FROM cards WHERE id = ?", id).Scan(&oldStatus)
		if err != nil {
			return nil, fmt.Errorf("get current status for card %d: %w", id, err)
		}
		if oldStatus != *p.Status {
			if p.Force {
				// Force bypasses the transition matrix but NOT status validity:
				// a target that isn't a real status (e.g. a typo) still errors.
				if !model.ValidStatuses[*p.Status] {
					return nil, fmt.Errorf("invalid status %q", *p.Status)
				}
			} else if !model.CanTransition(oldStatus, *p.Status) {
				allowed := model.AllowedTransitions(oldStatus)
				return nil, fmt.Errorf(
					"invalid status transition %q -> %q; allowed transitions from %q: %v",
					oldStatus, *p.Status, oldStatus, allowed,
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
	if p.Blocked != nil {
		setClauses = append(setClauses, "blocked = ?")
		if *p.Blocked {
			setArgs = append(setArgs, 1)
		} else {
			setArgs = append(setArgs, 0)
		}
	}

	if len(setClauses) > 0 {
		setClauses = append(setClauses, "updated_at = datetime('now')")
		query := "UPDATE cards SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
		setArgs = append(setArgs, id)
		if _, err := tx.Exec(query, setArgs...); err != nil {
			return nil, fmt.Errorf("update card %d: %w", id, err)
		}
	}

	// Replace assignees if provided, capturing the old set first so the audit
	// trail can diff added/removed names.
	var oldAssignees []string
	if p.Assignees != nil {
		rows, err := tx.Query(`
			SELECT a.name FROM agents a
			JOIN card_assignees ca ON a.id = ca.agent_id
			WHERE ca.card_id = ?
			ORDER BY a.name
		`, id)
		if err != nil {
			return nil, fmt.Errorf("load current assignees: %w", err)
		}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan assignee: %w", err)
			}
			oldAssignees = append(oldAssignees, name)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()

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

	// Record audit events within the same transaction so the trail commits
	// atomically with the mutation. v1 records status transitions and assignee
	// add/remove only.
	if p.Status != nil && oldStatus != *p.Status {
		if err := appendCardEvent(tx, model.CardEvent{
			CardID:    id,
			Actor:     p.Actor,
			EventType: "status_changed",
			FromValue: oldStatus,
			ToValue:   *p.Status,
			Forced:    p.Force,
		}); err != nil {
			return nil, err
		}
	}
	if p.Assignees != nil {
		added, removed := diffAssignees(oldAssignees, p.Assignees)
		for _, name := range added {
			if err := appendCardEvent(tx, model.CardEvent{
				CardID:    id,
				Actor:     p.Actor,
				EventType: "assignee_added",
				ToValue:   name,
			}); err != nil {
				return nil, err
			}
		}
		for _, name := range removed {
			if err := appendCardEvent(tx, model.CardEvent{
				CardID:    id,
				Actor:     p.Actor,
				EventType: "assignee_removed",
				FromValue: name,
			}); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return s.GetCard(id)
}

// diffAssignees returns names present in next but not old (added) and names
// present in old but not next (removed).
func diffAssignees(old, next []string) (added, removed []string) {
	oldSet := make(map[string]bool, len(old))
	for _, n := range old {
		oldSet[n] = true
	}
	nextSet := make(map[string]bool, len(next))
	for _, n := range next {
		nextSet[n] = true
	}
	for _, n := range next {
		if !oldSet[n] {
			added = append(added, n)
		}
	}
	for _, n := range old {
		if !nextSet[n] {
			removed = append(removed, n)
		}
	}
	return added, removed
}

func (s *Store) DeleteCard(id int) error {
	res, err := s.db.Exec("DELETE FROM cards WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete card %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("card %d not found", id)
	}
	return nil
}
