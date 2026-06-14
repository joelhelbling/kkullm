package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the build version. Overridden at release time via ldflags:
//
//	-X github.com/joelhelbling/kkullm/cmd.Version={{.Version}}
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the kkullm version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), Version)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
