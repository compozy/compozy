package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newMemoryDecisionsCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   memoryDecisionsKey,
		Short: "Inspect and revert Memory v2 controller decisions",
	}
	cmd.AddCommand(newMemoryDecisionsListCommand(deps))
	cmd.AddCommand(newMemoryDecisionsShowCommand(deps))
	cmd.AddCommand(newMemoryDecisionsRevertCommand(deps))
	return cmd
}

func newMemoryDecisionsListCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags
	var op string
	var targetFilename string
	var sinceRaw string
	var reason string
	var limit int

	cmd := &cobra.Command{
		Use:   memoryListKey,
		Short: "List Memory v2 controller decisions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			selector, err := resolveMemorySelectorFlags(deps, flags, memorySelectorOptions{})
			if err != nil {
				return err
			}
			since, err := parseSinceFlag(sinceRaw, deps.now)
			if err != nil {
				return err
			}
			response, err := client.ListMemoryDecisions(cmd.Context(), MemoryDecisionListQuery{
				Scope:          selector.Scope,
				WorkspaceID:    selector.WorkspaceID,
				AgentName:      selector.AgentName,
				AgentTier:      selector.AgentTier,
				Operation:      op,
				TargetFilename: targetFilename,
				Since:          since,
				Reason:         reason,
				Limit:          limit,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryDecisionListBundle(response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	cmd.Flags().StringVar(&op, "op", "", "Decision operation filter")
	cmd.Flags().StringVar(&targetFilename, "filename", "", "Target memory filename filter")
	cmd.Flags().StringVar(&sinceRaw, "since", "", "Show decisions since an RFC3339 timestamp or relative duration")
	cmd.Flags().StringVar(&reason, memoryReasonKey, "", "Reason substring filter")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of decisions to return")
	return cmd
}

func newMemoryDecisionsShowCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show one Memory v2 controller decision",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.GetMemoryDecision(cmd.Context(), strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryDecisionBundle("Memory Decision", response.Decision, response))
		},
	}
}

func newMemoryDecisionsRevertCommand(deps commandDeps) *cobra.Command {
	var reason string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "revert <id>",
		Short: "Revert one Memory v2 controller decision",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.RevertMemoryDecision(
				cmd.Context(),
				strings.TrimSpace(args[0]),
				MemoryDecisionRevertRequest{
					Reason: strings.TrimSpace(reason),
					DryRun: dryRun,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryDecisionRevertBundle(response))
		},
	}
	cmd.Flags().StringVar(&reason, memoryReasonKey, "", "Operator-visible revert reason")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Return the revert decision without applying it")
	return cmd
}
