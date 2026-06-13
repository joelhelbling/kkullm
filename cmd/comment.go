package cmd

import (
	"fmt"

	"github.com/joelhelbling/kkullm/client"
	"github.com/joelhelbling/kkullm/model"
	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Manage card comments",
	RunE:  rejectUnknownSubcommand,
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
		return emitList(comments, func(comment model.Comment) {
			fmt.Printf("[%s] %s: %s\n", comment.CreatedAt.Format("2006-01-02 15:04:05"), comment.Agent, comment.Body)
		})
	},
}

var commentCreateBody string

var commentCreateCmd = &cobra.Command{
	Use:   "create <card-id>",
	Short: "Add a comment to a card",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agent := requireAgent()
		cardID, err := parseID(args[0])
		if err != nil {
			return err
		}
		if dryRun {
			req := map[string]any{"card_id": cardID, "agent": agent, "body": commentCreateBody}
			return emitDryRun(fmt.Sprintf("would add comment to card #%d as %q", cardID, agent), req)
		}
		c := client.New(serverURL)
		comment, err := c.CreateComment(cardID, agent, commentCreateBody, "")
		if err != nil {
			return err
		}
		return emitResult(fmt.Sprintf("Added comment #%d to card #%d", comment.ID, comment.CardID), comment)
	},
}

func init() {
	commentCreateCmd.Flags().StringVar(&commentCreateBody, "body", "", "Comment body (required)")
	commentCreateCmd.MarkFlagRequired("body")

	commentCmd.AddCommand(commentListCmd)
	commentCmd.AddCommand(commentCreateCmd)
	rootCmd.AddCommand(commentCmd)
}
