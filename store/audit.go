package store

import (
	"database/sql"
	"fmt"

	"github.com/joelhelbling/kkullm/model"
)

// execer is satisfied by both *sql.DB and *sql.Tx so audit events can be
// appended either standalone or inside a card mutation's transaction.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// AppendCardEvent records a single audit event for a card and returns it with
// id and created_at populated. The trail is append-only; events are never
// updated or deleted.
func (s *Store) AppendCardEvent(e model.CardEvent) (*model.CardEvent, error) {
	if err := appendCardEvent(s.db, e); err != nil {
		return nil, err
	}
	var id int
	if err := s.db.QueryRow("SELECT last_insert_rowid()").Scan(&id); err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.getCardEvent(id)
}

// appendCardEvent inserts an event using the given execer (DB or Tx). It does
// not read the row back, so it is safe to call inside an open transaction.
func appendCardEvent(q execer, e model.CardEvent) error {
	_, err := q.Exec(
		"INSERT INTO card_events (card_id, actor, event_type, from_value, to_value) VALUES (?, ?, ?, ?, ?)",
		e.CardID, e.Actor, e.EventType, e.FromValue, e.ToValue,
	)
	if err != nil {
		return fmt.Errorf("insert card event: %w", err)
	}
	return nil
}

func (s *Store) getCardEvent(id int) (*model.CardEvent, error) {
	e := &model.CardEvent{}
	var from, to sql.NullString
	err := s.db.QueryRow(`
		SELECT id, card_id, actor, event_type, from_value, to_value, created_at
		FROM card_events WHERE id = ?
	`, id).Scan(&e.ID, &e.CardID, &e.Actor, &e.EventType, &from, &to, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get card event %d: %w", id, err)
	}
	e.FromValue = from.String
	e.ToValue = to.String
	return e, nil
}

// ListCardEvents returns a card's audit events in chronological order.
func (s *Store) ListCardEvents(cardID int) ([]model.CardEvent, error) {
	rows, err := s.db.Query(`
		SELECT id, card_id, actor, event_type, from_value, to_value, created_at
		FROM card_events WHERE card_id = ?
		ORDER BY id ASC
	`, cardID)
	if err != nil {
		return nil, fmt.Errorf("list card events: %w", err)
	}
	defer rows.Close()

	var events []model.CardEvent
	for rows.Next() {
		var e model.CardEvent
		var from, to sql.NullString
		if err := rows.Scan(&e.ID, &e.CardID, &e.Actor, &e.EventType, &from, &to, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan card event: %w", err)
		}
		e.FromValue = from.String
		e.ToValue = to.String
		events = append(events, e)
	}
	return events, rows.Err()
}
