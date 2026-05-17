package cmd

import (
	"fmt"

	"github.com/joelhelbling/kkullm/client"
	"github.com/joelhelbling/kkullm/model"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
	RunE:  rejectUnknownSubcommand,
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(serverURL)
		projects, err := c.ListProjects()
		if err != nil {
			return err
		}
		return emitList(projects, func(p model.Project) {
			desc := p.Description
			if desc == "" {
				desc = "(no description)"
			}
			fmt.Printf("%s — %s\n", p.Name, desc)
		})
	},
}

var (
	projectCreateName string
	projectCreateDesc string
)

var projectCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new project",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			req := map[string]string{"name": projectCreateName, "description": projectCreateDesc}
			return emitDryRun(fmt.Sprintf("would create project %q", projectCreateName), req)
		}
		c := client.New(serverURL)
		project, err := c.CreateProject(projectCreateName, projectCreateDesc)
		if err != nil {
			return err
		}
		return emitResult(fmt.Sprintf("Created project: %s (id=%d)", project.Name, project.ID), project)
	},
}

func init() {
	projectCreateCmd.Flags().StringVar(&projectCreateName, "name", "", "Project name (required)")
	projectCreateCmd.MarkFlagRequired("name")
	projectCreateCmd.Flags().StringVar(&projectCreateDesc, "description", "", "Project description")

	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectCreateCmd)
	rootCmd.AddCommand(projectCmd)
}
