-- Append-only audit trail of card changes. v1 records status transitions and
-- assignee add/remove. The schema is intentionally generic (event_type +
-- from_value/to_value) so new event types can be added without a migration.
CREATE TABLE IF NOT EXISTS card_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    actor TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL,
    from_value TEXT,
    to_value TEXT,
    forced INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_card_events_card_id ON card_events(card_id);
