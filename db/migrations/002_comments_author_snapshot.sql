-- Add nullable author_name snapshot and make agent_id nullable.
-- SQLite requires a table rebuild to alter column nullability while
-- preserving the FK cascade to cards.
ALTER TABLE comments ADD COLUMN author_name TEXT;

UPDATE comments
SET author_name = (SELECT name FROM agents WHERE agents.id = comments.agent_id)
WHERE author_name IS NULL;

CREATE TABLE comments_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    agent_id INTEGER REFERENCES agents(id),
    author_name TEXT,
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO comments_new (id, card_id, agent_id, author_name, body, created_at)
SELECT id, card_id, agent_id, author_name, body, created_at FROM comments;

DROP TABLE comments;
ALTER TABLE comments_new RENAME TO comments;

CREATE INDEX IF NOT EXISTS idx_comments_card ON comments(card_id);
