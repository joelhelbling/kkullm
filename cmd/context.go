package cmd

import (
	"os"

	"github.com/joelhelbling/kkullm/model"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// agentContextSchemaVersion is bumped whenever the shape of `agent-context`
// output changes, so agents can detect incompatibilities.
//
// v3: status-transition rules removed. status_transitions is gone from enums;
// any status may move to any other (the target must still be a valid status).
// v2: blocked is an orthogonal flag, not a status. It is gone from
// card_statuses and status_transitions; `card update` gains
// --blocked/--unblocked/--reason; comments carry a kind ("block"/"unblock").
const agentContextSchemaVersion = 3

type flagInfo struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default,omitempty"`
	Required  bool   `json:"required,omitempty"`
	Usage     string `json:"usage"`
}

type commandInfo struct {
	Name        string        `json:"name"`
	Usage       string        `json:"usage"`
	Summary     string        `json:"summary"`
	Flags       []flagInfo    `json:"flags,omitempty"`
	Subcommands []commandInfo `json:"subcommands,omitempty"`
}

type envVarInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type workflowInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
}

type agentContext struct {
	SchemaVersion int               `json:"schema_version"`
	CLI           string            `json:"cli"`
	Server        map[string]string `json:"server"`
	GlobalFlags   []flagInfo        `json:"global_flags"`
	Commands      []commandInfo     `json:"commands"`
	Enums         map[string]any    `json:"enums"`
	EnvVars       []envVarInfo      `json:"env_vars"`
	Workflows     []workflowInfo    `json:"workflows"`
}

func collectFlags(fs *pflag.FlagSet) []flagInfo {
	var flags []flagInfo
	fs.VisitAll(func(f *pflag.Flag) {
		_, required := f.Annotations[cobra.BashCompOneRequiredFlag]
		flags = append(flags, flagInfo{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Type:      f.Value.Type(),
			Default:   f.DefValue,
			Required:  required,
			Usage:     f.Usage,
		})
	})
	return flags
}

func describeCommand(c *cobra.Command) commandInfo {
	info := commandInfo{
		Name:    c.CommandPath(),
		Usage:   c.Use,
		Summary: c.Short,
		Flags:   collectFlags(c.LocalFlags()),
	}
	for _, sub := range c.Commands() {
		if !sub.IsAvailableCommand() || sub.Name() == "help" || sub.Name() == "completion" {
			continue
		}
		info.Subcommands = append(info.Subcommands, describeCommand(sub))
	}
	return info
}

func resolveServer() map[string]string {
	if rootCmd.PersistentFlags().Changed("server") {
		return map[string]string{"url": serverURL, "source": "flag"}
	}
	if os.Getenv("KKULLM_SERVER") != "" {
		return map[string]string{"url": serverURL, "source": "env (KKULLM_SERVER)"}
	}
	return map[string]string{"url": serverURL, "source": "default"}
}

var agentContextCmd = &cobra.Command{
	Use:   "agent-context",
	Short: "Emit a machine-readable description of the CLI for agents",
	Long: "Emit a versioned JSON document describing every command, flag, enum, " +
		"environment variable, and common workflow. Intended for AI agents to " +
		"discover the CLI's shape without parsing --help text.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := agentContext{
			SchemaVersion: agentContextSchemaVersion,
			CLI:           "kkullm",
			Server:        resolveServer(),
			GlobalFlags:   collectFlags(rootCmd.PersistentFlags()),
			Enums: map[string]any{
				"card_statuses":  model.AllStatuses,
				"relation_types": []string{"blocked_by", "belongs_to", "interested_in"},
				// blocked is an orthogonal flag, not a status: toggle it with
				// `card update --blocked/--unblocked` (optionally --reason).
				"comment_kinds": []string{"", "block", "unblock"},
			},
			EnvVars: []envVarInfo{
				{"KKULLM_SERVER", "Server URL. Precedence: --server flag > KKULLM_SERVER > default http://localhost:7722"},
				{"KKULLM_AGENT", "Agent identity. Precedence: --as flag > KKULLM_AGENT. Required for mutating commands."},
				{"KKULLM_PROJECT", "Default project. Precedence: --project flag > KKULLM_PROJECT."},
			},
			Workflows: []workflowInfo{
				{
					Name:        "pull-and-execute",
					Description: "Two-session blackboard loop: prioritize actionable cards, then claim and execute one.",
					Steps: []string{
						"kkullm card list --status todo --format full --json",
						"kkullm card update <id> --status in_flight --as <agent>",
						"kkullm comment create <id> --body \"...\" --as <agent>",
						"kkullm card update <id> --status completed --as <agent>",
					},
				},
				{
					Name:        "preview-a-mutation",
					Description: "Validate a create/update without sending it.",
					Steps: []string{
						"kkullm card create --title \"...\" --status todo --dry-run --json --as <agent>",
					},
				},
			},
		}
		for _, c := range rootCmd.Commands() {
			if !c.IsAvailableCommand() || c.Name() == "help" || c.Name() == "completion" {
				continue
			}
			ctx.Commands = append(ctx.Commands, describeCommand(c))
		}
		return emitJSON(ctx)
	},
}

func init() {
	rootCmd.AddCommand(agentContextCmd)
}
