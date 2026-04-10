package cmd

import (
	"fmt"

	"github.com/joelhelbling/kkullm/client"
	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Manage card comments",
}

var commentListCmd = &cobra.Command{
	Use:   "list <card-id>",
	Short: "List comments on a card",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cardID, err := parseID(args[0])
		if err != nil {
			return err
		}
		c := client.New(serverURL)
		comments, err := c.ListComments(cardID)
		if err != nil {
			return err
		}
		for _, comment := range comments {
			fmt.Printf("[%s] %s: %s\n", comment.CreatedAt.Format("2006-01-02 15:04:05"), comment.Agent, comment.Body)
		}
		return nil
	},
}

var commentAddBody string

var commentAddCmd = &cobra.Command{
	Use:   "add <card-id>",
	Short: "Add a comment to a card",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agent := requireAgent()
		cardID, err := parseID(args[0])
		if err != nil {
			return err
		}
		c := client.New(serverURL)
		comment, err := c.CreateComment(cardID, agent, commentAddBody)
		if err != nil {
			return err
		}
		fmt.Printf("Added comment #%d to card #%d\n", comment.ID, comment.CardID)
		return nil
	},
}

func init() {
	commentAddCmd.Flags().StringVar(&commentAddBody, "body", "", "Comment body (required)")
	commentAddCmd.MarkFlagRequired("body")

	commentCmd.AddCommand(commentListCmd)
	commentCmd.AddCommand(commentAddCmd)
	rootCmd.AddCommand(commentCmd)
}
