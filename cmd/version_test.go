package cmd

import (
	"bytes"
	"testing"
)

func TestVersionOutput(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		wantStr string
	}{
		{[]string{"version"}, "1.2.3"},
		{[]string{"--version"}, "dev"},
	} {
		Version = "1.2.3"
		var out bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&out)
		rootCmd.SetArgs(tc.args)
		if err := rootCmd.Execute(); err != nil {
			Version = "dev"
			t.Fatalf("execute %v: %v", tc.args, err)
		}
		Version = "dev"
		if !bytes.Contains(out.Bytes(), []byte(tc.wantStr)) {
			t.Errorf("%v output = %q; want it to contain %q", tc.args, out.String(), tc.wantStr)
		}
	}
}
