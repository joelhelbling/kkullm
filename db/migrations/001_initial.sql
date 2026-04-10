-- Projects
CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Agents
CREATE TABLE IF NOT EXISTS agents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    bio TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Project Assets
CREATE TABLE IF NOT EXISTS project_assets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    name TEXT NOT NULL,
    description TEXT,
    url TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Cards
CREATE TABLE IF NOT EXISTS cards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    body TEXT,
    status TEXT NOT NULL DEFAULT 'considering'
        CHECK(status IN ('considering', 'todo', 'in_flight', 'completed', 'done', 'tabled', 'blocked')),
    project_id INTEGER NOT NULL REFERENCES projects(id),
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Card Assignees (join table)
CREATE TABLE IF NOT EXISTS card_assignees (
    card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    agent_id INTEGER NOT NULL REFERENCES agents(id),
    PRIMARY KEY (card_id, agent_id)
);

-- Card Tags
CREATE TABLE IF NOT EXISTS card_tags (
    card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (card_id, tag)
);

-- Card Relations
CREATE TABLE IF NOT EXISTS card_relations (
    card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    related_card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    relation_type TEXT NOT NULL
        CHECK(relation_type IN ('blocked_by', 'belongs_to', 'interested_in')),
    PRIMARY KEY (card_id, related_card_id, relation_type)
);

-- Comments
CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    agent_id INTEGER NOT NULL REFERENCES agents(id),
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_cards_project_status ON cards(project_id, status);
CREATE INDEX IF NOT EXISTS idx_card_assignees_agent ON card_assignees(agent_id);
CREATE INDEX IF NOT EXISTS idx_comments_card ON comments(card_id);
CREATE INDEX IF NOT EXISTS idx_agents_project ON agents(project_id);
CREATE INDEX IF NOT EXISTS idx_project_assets_project ON project_assets(project_id);
