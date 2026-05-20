package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/joelhelbling/kkullm/client"
	"github.com/joelhelbling/kkullm/model"
	"github.com/spf13/cobra"
)

var cardCmd = &cobra.Command{
	Use:   "card",
	Short: "Manage cards",
	RunE:  rejectUnknownSubcommand,
}

// --- card list ---

var (
	cardListStatus   string
	cardListAssignee string
	cardListTag      string
	cardListFormat   string
	cardListArchived bool
)

// cliArchiveLimit caps completed/tabled cards in CLI output.
// Active view shows the most-recent N; --archived shows the overflow.
const cliArchiveLimit = 3

var cardListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cards",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateFormat(cardListFormat); err != nil {
			return err
		}
		if cardListStatus != "" {
			if err := validateStatus(cardListStatus); err != nil {
				return err
			}
		}

		c := client.New(serverURL)
		opts := client.CardListOptions{
			Project:      projectName,
			Assignee:     cardListAssignee,
			Status:       cardListStatus,
			Tag:          cardListTag,
			ArchiveLimit: cliArchiveLimit,
		}
		if cardListArchived {
			opts.ArchiveView = "archived"
		}
		cards, err := c.ListCards(opts)
		if err != nil {
			return err
		}

		return emitList(cards, func(card model.Card) {
			if cardListFormat == "full" {
				printCardFull(&card)
				return
			}
			tags := ""
			if len(card.Tags) > 0 {
				tags = " [" + strings.Join(card.Tags, ", ") + "]"
			}
			assignee := ""
			if len(card.Assignees) > 0 {
				assignee = strings.Join(card.Assignees, ",")
			}
			fmt.Printf("#%-5d %-12s %-12s %s%s\n", card.ID, card.Status, assignee, card.Title, tags)
		})
	},
}

// --- card get ---

var cardGetCmd = &cobra.Command{
	Use:   "get <id>",
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
		if jsonOutput {
			return emitJSON(card)
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
		if err := validateStatus(cardCreateStatus); err != nil {
			return err
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

		if dryRun {
			return emitDryRun(fmt.Sprintf("would create card %q in project %q", req.Title, project), req)
		}

		c := client.New(serverURL)
		card, err := c.CreateCard(req)
		if err != nil {
			return err
		}
		return emitResult(fmt.Sprintf("Created card #%d: %s", card.ID, card.Title), card)
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
			if err := validateStatus(cardUpdateStatus); err != nil {
				return err
			}
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

		if dryRun {
			return emitDryRun(fmt.Sprintf("would update card #%d", id), req)
		}

		c := client.New(serverURL)
		card, err := c.UpdateCard(id, req)
		if err != nil {
			return err
		}
		return emitResult(fmt.Sprintf("Updated card #%d: %s", card.ID, card.Title), card)
	},
}

// --- helpers ---

func printCardFull(card *model.Card) {
	fmt.Println("---")
	fmt.Printf("id: %d\n", card.ID)
	fmt.Printf("title: %s\n", card.Title)
	fmt.Printf("status: %s\n", card.Status)
	fmt.Printf("project: %s\n", card.Project)
	if len(card.Assignees) > 0 {
		fmt.Printf("assignees: %s\n", strings.Join(card.Assignees, ", "))
	}
	if len(card.Tags) > 0 {
		fmt.Printf("tags: %s\n", strings.Join(card.Tags, ", "))
	}
	if len(card.Relations) > 0 {
		parts := make([]string, 0, len(card.Relations))
		for _, r := range card.Relations {
			parts = append(parts, fmt.Sprintf("%s #%d", r.RelationType, r.RelatedCardID))
		}
		fmt.Printf("relations: %s\n", strings.Join(parts, ", "))
	}
	if card.CommentCount > 0 {
		fmt.Printf("comment_count: %d\n", card.CommentCount)
	}
	fmt.Printf("created_at: %s\n", card.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("updated_at: %s\n", card.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println("---")
	if card.Body != "" {
		fmt.Println()
		fmt.Println(card.Body)
	}
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
	trimmed := strings.TrimPrefix(s, "#")
	id, err := strconv.Atoi(trimmed)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id %q: expected a positive integer (e.g. 42 or #42)", s)
	}
	return id, nil
}

func init() {
	// card list flags
	cardListCmd.Flags().StringVar(&cardListStatus, "status", "", "Filter by status")
	cardListCmd.Flags().StringVar(&cardListAssignee, "assignee", "", "Filter by assignee")
	cardListCmd.Flags().StringVar(&cardListTag, "tag", "", "Filter by tag")
	cardListCmd.Flags().StringVar(&cardListFormat, "format", "brief", "Output verbosity: brief or full")
	cardListCmd.Flags().BoolVar(&cardListArchived, "archived", false, "Show archived completed/tabled cards instead of the active set")

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
	cardCmd.AddCommand(cardGetCmd)
	cardCmd.AddCommand(cardCreateCmd)
	cardCmd.AddCommand(cardUpdateCmd)
	rootCmd.AddCommand(cardCmd)
}
