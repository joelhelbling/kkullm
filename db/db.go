package db

import (
	"database/sql"
	"embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec %q: %w", pragma, err)
		}
	}

	return db, nil
}

func Migrate(db *sql.DB) error {
	data, err := migrations.ReadFile("migrations/001_initial.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := db.Exec(string(data)); err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}
	return nil
}

func Seed(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT OR IGNORE INTO projects (name, description) VALUES ('orchestration', 'Oversight of the Kkullm board')`)
	if err != nil {
		return fmt.Errorf("seed orchestration project: %w", err)
	}

	_, err = tx.Exec(`INSERT OR IGNORE INTO agents (name, project_id, bio) VALUES ('user', (SELECT id FROM projects WHERE name = 'orchestration'), 'The human operator')`)
	if err != nil {
		return fmt.Errorf("seed user agent: %w", err)
	}

	return tx.Commit()
}
