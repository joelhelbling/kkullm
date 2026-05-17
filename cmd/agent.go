package cmd

import (
	"fmt"

	"github.com/joelhelbling/kkullm/client"
	"github.com/joelhelbling/kkullm/model"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
	RunE:  rejectUnknownSubcommand,
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
		return emitList(agents, func(a model.Agent) {
			bio := a.Bio
			if bio == "" {
				bio = "(no bio)"
			}
			fmt.Printf("%s [%s] — %s\n", a.Name, a.Project, bio)
		})
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
		if dryRun {
			req := map[string]string{"name": agentCreateName, "project": project, "bio": agentCreateBio}
			return emitDryRun(fmt.Sprintf("would create agent %q in project %q", agentCreateName, project), req)
		}
		c := client.New(serverURL)
		agent, err := c.CreateAgent(agentCreateName, project, agentCreateBio)
		if err != nil {
			return err
		}
		return emitResult(fmt.Sprintf("Created agent: %s (id=%d)", agent.Name, agent.ID), agent)
	},
}

var agentGetCmd = &cobra.Command{
	Use:   "get <name>",
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
				if jsonOutput {
					return emitJSON(a)
				}
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
	agentCmd.AddCommand(agentGetCmd)
	rootCmd.AddCommand(agentCmd)
}
