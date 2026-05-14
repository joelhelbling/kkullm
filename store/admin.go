package store

import (
	"fmt"

	"github.com/joelhelbling/kkullm/db"
)

// Purge wipes all user data tables and re-runs the baseline Seed so the
// post-purge state matches a fresh install. Migrations table is untouched.
func (s *Store) Purge() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin purge: %w", err)
	}
	defer tx.Rollback()

	for _, q := range []string{
		"DELETE FROM comments",
		"DELETE FROM card_assignees",
		"DELETE FROM card_tags",
		"DELETE FROM card_relations",
		"DELETE FROM cards",
		"DELETE FROM project_assets",
		"DELETE FROM agents",
		"DELETE FROM projects",
		"DELETE FROM sqlite_sequence",
	} {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("purge %q: %w", q, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit purge: %w", err)
	}

	if err := db.Seed(s.db); err != nil {
		return fmt.Errorf("re-seed after purge: %w", err)
	}
	return nil
}
