package cmd

import (
	"fmt"

	"github.com/joelhelbling/kkullm/client"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(serverURL)
		agents, err := c.ListAgents(projectName)
		if err != nil {
			return err
		}
		for _, a := range agents {
			bio := a.Bio
			if bio == "" {
				bio = "(no bio)"
			}
			fmt.Printf("%s [%s] — %s\n", a.Name, a.Project, bio)
		}
		return nil
	},
}

var (
	agentCreateName    string
	agentCreateProject string
	agentCreateBio     string
)

var agentCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		project := agentCreateProject
		if project == "" {
			project = projectName
		}
		if project == "" {
			return fmt.Errorf("project is required: use --project flag or set KKULLM_PROJECT")
		}
		c := client.New(serverURL)
		agent, err := c.CreateAgent(agentCreateName, project, agentCreateBio)
		if err != nil {
			return err
		}
		fmt.Printf("Created agent: %s (id=%d)\n", agent.Name, agent.ID)
		return nil
	},
}

var agentShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show agent details by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		c := client.New(serverURL)
		agents, err := c.ListAgents("")
		if err != nil {
			return err
		}
		for _, a := range agents {
			if a.Name == name {
				fmt.Printf("ID:      %d\n", a.ID)
				fmt.Printf("Name:    %s\n", a.Name)
				fmt.Printf("Project: %s\n", a.Project)
				fmt.Printf("Bio:     %s\n", a.Bio)
				fmt.Printf("Created: %s\n", a.CreatedAt.Format("2006-01-02 15:04:05"))
				return nil
			}
		}
		return fmt.Errorf("agent not found: %s", name)
	},
}

func init() {
	agentCreateCmd.Flags().StringVar(&agentCreateName, "name", "", "Agent name (required)")
	agentCreateCmd.MarkFlagRequired("name")
	agentCreateCmd.Flags().StringVar(&agentCreateProject, "project", "", "Project name (falls back to global --project)")
	agentCreateCmd.Flags().StringVar(&agentCreateBio, "bio", "", "Agent bio")

	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentCreateCmd)
	agentCmd.AddCommand(agentShowCmd)
	rootCmd.AddCommand(agentCmd)
}
