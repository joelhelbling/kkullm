package cmd

import (
	"fmt"

	"github.com/joelhelbling/kkullm/client"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
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
		for _, p := range projects {
			desc := p.Description
			if desc == "" {
				desc = "(no description)"
			}
			fmt.Printf("%s — %s\n", p.Name, desc)
		}
		return nil
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
		c := client.New(serverURL)
		project, err := c.CreateProject(projectCreateName, projectCreateDesc)
		if err != nil {
			return err
		}
		fmt.Printf("Created project: %s (id=%d)\n", project.Name, project.ID)
		return nil
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
