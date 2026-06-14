package cmd

import (
	"bytes"
	"testing"
)

func TestVersionCommandPrintsVersion(t *testing.T) {
	Version = "1.2.3"
	defer func() { Version = "dev" }()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}

	if got := out.String(); got == "" || !bytes.Contains(out.Bytes(), []byte("1.2.3")) {
		t.Errorf("version output = %q; want it to contain %q", got, "1.2.3")
	}
}
