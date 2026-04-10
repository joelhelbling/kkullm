package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/joelhelbling/kkullm/client"
	"github.com/joelhelbling/kkullm/model"
	"github.com/spf13/cobra"
)

var cardCmd = &cobra.Command{
	Use:   "card",
	Short: "Manage cards",
}

// --- card list ---

var (
	cardListStatus   string
	cardListAssignee string
	cardListTag      string
	cardListFormat   string
	cardListJSON     bool
)

var cardListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cards",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(serverURL)
		cards, err := c.ListCards(projectName, cardListAssignee, cardListStatus, cardListTag)
		if err != nil {
			return err
		}

		if cardListJSON {
			data, err := json.MarshalIndent(cards, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		if cardListFormat == "full" {
			for i, card := range cards {
				if i > 0 {
					fmt.Println("---")
				}
				printCardFull(&card)
			}
			return nil
		}

		// brief format
		for _, card := range cards {
			tags := ""
			if len(card.Tags) > 0 {
				tags = " [" + strings.Join(card.Tags, ", ") + "]"
			}
			assignee := ""
			if len(card.Assignees) > 0 {
				assignee = strings.Join(card.Assignees, ",")
			}
			fmt.Printf("#%-5d %-12s %-12s %s%s\n", card.ID, card.Status, assignee, card.Title, tags)
		}
		return nil
	},
}

// --- card show ---

var cardShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show card details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		c := client.New(serverURL)
		card, err := c.GetCard(id)
		if err != nil {
			return err
		}
		printCardFull(card)
		return nil
	},
}

// --- card create ---

var (
	cardCreateTitle        string
	cardCreateBody         string
	cardCreateStatus       string
	cardCreateAssignees    []string
	cardCreateTags         []string
	cardCreateBlockedBy    []int
	cardCreateBelongsTo    []int
	cardCreateInterestedIn []int
)

var cardCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new card",
	RunE: func(cmd *cobra.Command, args []string) error {
		requireAgent()
		project := projectName
		if project == "" {
			return fmt.Errorf("project is required: use --project flag or set KKULLM_PROJECT")
		}

		req := client.CardCreateRequest{
			Title:   cardCreateTitle,
			Body:    cardCreateBody,
			Status:  cardCreateStatus,
			Project: project,
		}
		if len(cardCreateAssignees) > 0 {
			req.Assignees = cardCreateAssignees
		}
		if len(cardCreateTags) > 0 {
			req.Tags = cardCreateTags
		}
		req.Relations = buildRelations(cardCreateBlockedBy, cardCreateBelongsTo, cardCreateInterestedIn)

		c := client.New(serverURL)
		card, err := c.CreateCard(req)
		if err != nil {
			return err
		}
		fmt.Printf("Created card #%d: %s\n", card.ID, card.Title)
		return nil
	},
}

// --- card update ---

var (
	cardUpdateTitle        string
	cardUpdateBody         string
	cardUpdateStatus       string
	cardUpdateAssignees    []string
	cardUpdateTags         []string
	cardUpdateBlockedBy    []int
	cardUpdateBelongsTo    []int
	cardUpdateInterestedIn []int
)

var cardUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a card",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		requireAgent()
		id, err := parseID(args[0])
		if err != nil {
			return err
		}

		req := client.CardUpdateRequest{}

		if cmd.Flags().Changed("title") {
			req.Title = &cardUpdateTitle
		}
		if cmd.Flags().Changed("body") {
			req.Body = &cardUpdateBody
		}
		if cmd.Flags().Changed("status") {
			req.Status = &cardUpdateStatus
		}
		if cmd.Flags().Changed("assignee") {
			req.Assignees = cardUpdateAssignees
		}
		if cmd.Flags().Changed("tag") {
			req.Tags = cardUpdateTags
		}

		relations := buildRelations(cardUpdateBlockedBy, cardUpdateBelongsTo, cardUpdateInterestedIn)
		if len(relations) > 0 {
			req.Relations = relations
		}

		c := client.New(serverURL)
		card, err := c.UpdateCard(id, req)
		if err != nil {
			return err
		}
		fmt.Printf("Updated card #%d: %s\n", card.ID, card.Title)
		return nil
	},
}

// --- helpers ---

func printCardFull(card *model.Card) {
	fmt.Printf("ID:        %d\n", card.ID)
	fmt.Printf("Title:     %s\n", card.Title)
	fmt.Printf("Status:    %s\n", card.Status)
	fmt.Printf("Project:   %s\n", card.Project)
	if card.Body != "" {
		fmt.Printf("Body:      %s\n", card.Body)
	}
	if len(card.Assignees) > 0 {
		fmt.Printf("Assignees: %s\n", strings.Join(card.Assignees, ", "))
	}
	if len(card.Tags) > 0 {
		fmt.Printf("Tags:      %s\n", strings.Join(card.Tags, ", "))
	}
	if len(card.Relations) > 0 {
		for _, r := range card.Relations {
			fmt.Printf("Relation:  %s #%d\n", r.RelationType, r.RelatedCardID)
		}
	}
	if card.CommentCount > 0 {
		fmt.Printf("Comments:  %d\n", card.CommentCount)
	}
	fmt.Printf("Created:   %s\n", card.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:   %s\n", card.UpdatedAt.Format("2006-01-02 15:04:05"))
}

func buildRelations(blockedBy, belongsTo, interestedIn []int) []model.CardRelation {
	var relations []model.CardRelation
	for _, id := range blockedBy {
		relations = append(relations, model.CardRelation{RelatedCardID: id, RelationType: "blocked_by"})
	}
	for _, id := range belongsTo {
		relations = append(relations, model.CardRelation{RelatedCardID: id, RelationType: "belongs_to"})
	}
	for _, id := range interestedIn {
		relations = append(relations, model.CardRelation{RelatedCardID: id, RelationType: "interested_in"})
	}
	return relations
}

func parseID(s string) (int, error) {
	// Strip leading # if present
	s = strings.TrimPrefix(s, "#")
	var id int
	_, err := fmt.Sscanf(s, "%d", &id)
	if err != nil {
		return 0, fmt.Errorf("invalid id: %s", s)
	}
	return id, nil
}

func init() {
	// card list flags
	cardListCmd.Flags().StringVar(&cardListStatus, "status", "", "Filter by status")
	cardListCmd.Flags().StringVar(&cardListAssignee, "assignee", "", "Filter by assignee")
	cardListCmd.Flags().StringVar(&cardListTag, "tag", "", "Filter by tag")
	cardListCmd.Flags().StringVar(&cardListFormat, "format", "brief", "Output format: brief or full")
	cardListCmd.Flags().BoolVar(&cardListJSON, "json", false, "Output as JSON")

	// card create flags
	cardCreateCmd.Flags().StringVar(&cardCreateTitle, "title", "", "Card title (required)")
	cardCreateCmd.MarkFlagRequired("title")
	cardCreateCmd.Flags().StringVar(&cardCreateBody, "body", "", "Card body")
	cardCreateCmd.Flags().StringVar(&cardCreateStatus, "status", "considering", "Initial status")
	cardCreateCmd.Flags().StringSliceVar(&cardCreateAssignees, "assignee", nil, "Assignee (repeatable)")
	cardCreateCmd.Flags().StringSliceVar(&cardCreateTags, "tag", nil, "Tag (repeatable)")
	cardCreateCmd.Flags().IntSliceVar(&cardCreateBlockedBy, "blocked-by", nil, "Blocked by card ID (repeatable)")
	cardCreateCmd.Flags().IntSliceVar(&cardCreateBelongsTo, "belongs-to", nil, "Belongs to card ID (repeatable)")
	cardCreateCmd.Flags().IntSliceVar(&cardCreateInterestedIn, "interested-in", nil, "Interested in card ID (repeatable)")

	// card update flags
	cardUpdateCmd.Flags().StringVar(&cardUpdateTitle, "title", "", "New title")
	cardUpdateCmd.Flags().StringVar(&cardUpdateBody, "body", "", "New body")
	cardUpdateCmd.Flags().StringVar(&cardUpdateStatus, "status", "", "New status")
	cardUpdateCmd.Flags().StringSliceVar(&cardUpdateAssignees, "assignee", nil, "Assignee (repeatable)")
	cardUpdateCmd.Flags().StringSliceVar(&cardUpdateTags, "tag", nil, "Tag (repeatable)")
	cardUpdateCmd.Flags().IntSliceVar(&cardUpdateBlockedBy, "blocked-by", nil, "Blocked by card ID (repeatable)")
	cardUpdateCmd.Flags().IntSliceVar(&cardUpdateBelongsTo, "belongs-to", nil, "Belongs to card ID (repeatable)")
	cardUpdateCmd.Flags().IntSliceVar(&cardUpdateInterestedIn, "interested-in", nil, "Interested in card ID (repeatable)")

	cardCmd.AddCommand(cardListCmd)
	cardCmd.AddCommand(cardShowCmd)
	cardCmd.AddCommand(cardCreateCmd)
	cardCmd.AddCommand(cardUpdateCmd)
	rootCmd.AddCommand(cardCmd)
}
