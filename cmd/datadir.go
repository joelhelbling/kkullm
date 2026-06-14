package cmd

import (
	"os"
	"path/filepath"
)

// defaultDBPath returns the canonical location for the server's SQLite database:
// $XDG_DATA_HOME/kkullm/kkullm.db, falling back to ~/.local/share/kkullm/kkullm.db.
// It is pure — directory creation happens in the serve command, not here.
func defaultDBPath() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Last-resort fallback: relative path, preserving old behavior.
			return "kkullm.db"
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "kkullm", "kkullm.db")
}
