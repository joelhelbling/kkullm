package cmd

import (
	"fmt"

	"github.com/joelhelbling/kkullm/client"
	"github.com/spf13/cobra"
)

var assetCmd = &cobra.Command{
	Use:   "asset",
	Short: "Manage project assets",
}

var (
	assetListName string
	assetListURL  string
)

var assetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List assets",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(serverURL)
		assets, err := c.ListAssets(projectName, assetListName, assetListURL)
		if err != nil {
			return err
		}
		for _, a := range assets {
			url := a.URL
			if url == "" {
				url = "(no url)"
			}
			fmt.Printf("%s [%s] %s\n", a.Name, a.Project, url)
		}
		return nil
	},
}

var (
	assetCreateName string
	assetCreateDesc string
	assetCreateURL  string
)

var assetCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new asset",
	RunE: func(cmd *cobra.Command, args []string) error {
		project := projectName
		if project == "" {
			return fmt.Errorf("project is required: use --project flag or set KKULLM_PROJECT")
		}
		c := client.New(serverURL)
		asset, err := c.CreateAsset(project, assetCreateName, assetCreateDesc, assetCreateURL)
		if err != nil {
			return err
		}
		fmt.Printf("Created asset: %s (id=%d)\n", asset.Name, asset.ID)
		return nil
	},
}

var assetShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show asset details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		c := client.New(serverURL)
		asset, err := c.GetAsset(id)
		if err != nil {
			return err
		}
		fmt.Printf("ID:          %d\n", asset.ID)
		fmt.Printf("Name:        %s\n", asset.Name)
		fmt.Printf("Project:     %s\n", asset.Project)
		if asset.Description != "" {
			fmt.Printf("Description: %s\n", asset.Description)
		}
		if asset.URL != "" {
			fmt.Printf("URL:         %s\n", asset.URL)
		}
		fmt.Printf("Created:     %s\n", asset.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated:     %s\n", asset.UpdatedAt.Format("2006-01-02 15:04:05"))
		return nil
	},
}

func init() {
	assetListCmd.Flags().StringVar(&assetListName, "name", "", "Filter by name glob")
	assetListCmd.Flags().StringVar(&assetListURL, "url", "", "Filter by URL glob")

	assetCreateCmd.Flags().StringVar(&assetCreateName, "name", "", "Asset name (required)")
	assetCreateCmd.MarkFlagRequired("name")
	assetCreateCmd.Flags().StringVar(&assetCreateDesc, "description", "", "Asset description")
	assetCreateCmd.Flags().StringVar(&assetCreateURL, "url", "", "Asset URL")

	assetCmd.AddCommand(assetListCmd)
	assetCmd.AddCommand(assetCreateCmd)
	assetCmd.AddCommand(assetShowCmd)
	rootCmd.AddCommand(assetCmd)
}
