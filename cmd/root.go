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
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", envOrDefault("KKULLM_SERVER", "http://localhost:8080"), "Kkullm server URL")
	rootCmd.PersistentFlags().StringVar(&agentName, "as", os.Getenv("KKULLM_AGENT"), "Agent identity")
	rootCmd.PersistentFlags().StringVar(&projectName, "project", os.Getenv("KKULLM_PROJECT"), "Default project")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireAgent() string {
	if agentName == "" {
		fmt.Fprintln(os.Stderr, "Error: agent identity required. Set KKULLM_AGENT or use --as flag.")
		os.Exit(1)
	}
	return agentName
}
