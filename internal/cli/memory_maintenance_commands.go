package cli

import (
	"errors"

	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newMemoryReindexCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags
	var includeSystem bool

	cmd := &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild the derived Memory v2 search catalog",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			selector, err := resolveMemorySelectorFlags(deps, flags, memorySelectorOptions{DefaultWorkspace: true})
			if err != nil {
				return err
			}
			response, err := client.ReindexMemory(cmd.Context(), MemoryReindexRequest{
				Scope:         selector.Scope,
				WorkspaceID:   selector.WorkspaceID,
				AgentName:     selector.AgentName,
				AgentTier:     selector.AgentTier,
				IncludeSystem: includeSystem,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryReindexBundle(response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	cmd.Flags().BoolVar(&includeSystem, "include-system", false, "Include _system memory entries")
	return cmd
}

func newMemoryHistoryCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags
	var operation string
	var sinceRaw string
	var limit int

	cmd := &cobra.Command{
		Use:   memoryHistoryKey,
		Short: "Show redaction-safe Memory v2 operation history",
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
			records, err := client.MemoryHistory(cmd.Context(), MemoryHistoryQuery{
				Scope:       selector.Scope,
				WorkspaceID: selector.WorkspaceID,
				AgentName:   selector.AgentName,
				AgentTier:   selector.AgentTier,
				Operation:   strings.TrimSpace(operation),
				Since:       since,
				Limit:       limit,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryHistoryBundle(records, deps.now))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	cmd.Flags().StringVar(&operation, memoryOperationKey, "", "Memory operation type, for example memory.write")
	cmd.Flags().StringVar(&sinceRaw, "since", "", "Show operations since an RFC3339 timestamp or relative duration")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of operations to return")
	return cmd
}

func newMemoryHealthCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   memoryHealthKey,
		Short: "Show Memory v2 health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := currentWorkingDirectory(deps)
			if err != nil {
				return err
			}
			health, err := client.MemoryHealth(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryHealthBundle(health))
		},
	}
}

func newMemoryPromoteCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags
	var fromRaw string
	var toRaw string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "promote <filename> --from <scope[:tier]> --to <scope[:tier]>",
		Short: "Promote a memory entry across Memory v2 scopes",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			from, err := parseMemoryPromotionSelector(deps, flags, fromRaw)
			if err != nil {
				return err
			}
			to, err := parseMemoryPromotionSelector(deps, flags, toRaw)
			if err != nil {
				return err
			}
			response, err := client.PromoteMemory(cmd.Context(), MemoryPromoteRequest{
				Filename: strings.TrimSpace(args[0]),
				From:     from,
				To:       to,
				DryRun:   dryRun,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryPromoteBundle(response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	cmd.Flags().StringVar(&fromRaw, "from", "", "Source scope: global, workspace, agent:workspace, or agent:global")
	cmd.Flags().StringVar(&toRaw, "to", "", "Destination scope: global, workspace, agent:workspace, or agent:global")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Ask for a promotion decision without applying it")
	mustMarkFlagRequired(cmd, "from")
	mustMarkFlagRequired(cmd, "to")
	return cmd
}

func newMemoryResetCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags
	var includeDaily bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset derived Memory v2 state through the daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			selector, err := resolveMemorySelectorFlags(deps, flags, memorySelectorOptions{DefaultWorkspace: true})
			if err != nil {
				return err
			}
			response, err := client.ResetMemory(cmd.Context(), MemoryResetRequest{
				Scope:       selector.Scope,
				WorkspaceID: selector.WorkspaceID,
				AgentName:   selector.AgentName,
				AgentTier:   selector.AgentTier,
				DerivedOnly: !includeDaily,
				Confirm:     !dryRun,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Reset", response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	cmd.Flags().BoolVar(&includeDaily, "include-daily", false, "Include daily memory artifacts")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show reset work without applying it")
	return cmd
}

func newMemoryReloadCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags

	cmd := &cobra.Command{
		Use:   configReloadCommandName,
		Short: "Invalidate frozen memory snapshots for future session boots",
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
			response, err := client.ReloadMemory(cmd.Context(), selector)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Reload", response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	return cmd
}

func newMemoryScopeShowCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags

	cmd := &cobra.Command{
		Use:   memoryScopeShowValue,
		Short: "Show resolved Memory v2 precedence for a selector",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			selector, err := resolveMemorySelectorFlags(deps, flags, memorySelectorOptions{DefaultWorkspace: true})
			if err != nil {
				return err
			}
			response, err := client.MemoryScopeShow(cmd.Context(), selector)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryScopeShowBundle(response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	return cmd
}

func newMemoryRecallCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recall",
		Short: "Inspect Memory v2 recall traces",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "trace <session_id> <turn_seq>",
		Short: "Show one redaction-safe recall trace",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			turnSeq, err := strconv.ParseInt(strings.TrimSpace(args[1]), 10, 64)
			if err != nil || turnSeq <= 0 {
				return errors.New("memory.recall.turn_seq_invalid: turn_seq must be a positive integer")
			}
			response, err := client.GetMemoryRecallTrace(cmd.Context(), strings.TrimSpace(args[0]), turnSeq)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Recall Trace", response))
		},
	})
	return cmd
}
