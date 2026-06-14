package cmd

import (
	"path/filepath"
	"testing"
)

func TestDefaultDBPathUsesXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdghome")
	got := defaultDBPath()
	want := filepath.Join("/tmp/xdghome", "kkullm", "kkullm.db")
	if got != want {
		t.Errorf("defaultDBPath() = %q; want %q", got, want)
	}
}

func TestDefaultDBPathFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/tmp/fakehome")
	got := defaultDBPath()
	want := filepath.Join("/tmp/fakehome", ".local", "share", "kkullm", "kkullm.db")
	if got != want {
		t.Errorf("defaultDBPath() = %q; want %q", got, want)
	}
}
