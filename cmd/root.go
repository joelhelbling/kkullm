package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	serverURL   string
	agentName   string
	projectName string
)

var rootCmd = &cobra.Command{
	Use:   "kkullm",
	Short: "Agent orchestration system based on the blackboard pattern",
	// Execute() is the single place that reports errors. Silencing Cobra's own
	// error/usage output keeps failures to one clean line on stderr — agents
	// (and humans) get the error, not a usage dump duplicated twice.
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", envOrDefault("KKULLM_SERVER", "http://localhost:7722"), "Kkullm server URL")
	rootCmd.PersistentFlags().StringVar(&agentName, "as", os.Getenv("KKULLM_AGENT"), "Agent identity")
	rootCmd.PersistentFlags().StringVar(&projectName, "project", os.Getenv("KKULLM_PROJECT"), "Default project")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON instead of text")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Validate and preview a mutation without sending it")
	rootCmd.PersistentFlags().IntVar(&limitFlag, "limit", 50, "Max rows for list commands (0 = unlimited)")
	rootCmd.Version = Version
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// rejectUnknownSubcommand is the RunE for command groups (card, agent, ...).
// With no args it prints help; with an unrecognized subcommand it fails with a
// non-zero exit instead of Cobra's default silent exit-0 help dump — so an
// agent calling a renamed/typo'd verb gets an error, not a false success.
func rejectUnknownSubcommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	return fmt.Errorf("unknown command %q for %q; run \"%s --help\" to list commands",
		args[0], cmd.CommandPath(), cmd.CommandPath())
}

func requireAgent() string {
	if agentName == "" {
		fmt.Fprintln(os.Stderr, "Error: agent identity required. Set KKULLM_AGENT or use --as flag.")
		os.Exit(1)
	}
	return agentName
}
