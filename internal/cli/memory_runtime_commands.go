package cli

import (
	"context"

	"errors"
	"fmt"

	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newMemoryDreamCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   memoryDreamKey,
		Short: "Operate Memory v2 dreaming runs",
	}
	cmd.AddCommand(newMemoryDreamShowCommand(deps))
	cmd.AddCommand(newMemoryDreamRetryCommand(deps))
	cmd.AddCommand(newMemoryDreamTriggerCommand(deps))
	cmd.AddCommand(newMemoryDreamStatusCommand(deps))
	return cmd
}

func newMemoryDreamShowCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <date-or-run-id>",
		Short: "Show one Memory v2 dreaming run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.GetMemoryDream(cmd.Context(), strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Dream", response))
		},
	}
}

func newMemoryDreamRetryCommand(deps commandDeps) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "retry <run_id>",
		Short: "Retry one failed Memory v2 dreaming run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.RetryMemoryDream(cmd.Context(), strings.TrimSpace(args[0]), MemoryDreamRetryRequest{
				Force: force,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Dream Retry", response))
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Retry even if normal gates would skip the run")
	return cmd
}

func newMemoryDreamTriggerCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags
	var force bool

	cmd := &cobra.Command{
		Use:   memoryTriggerKey,
		Short: "Trigger Memory v2 dreaming",
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
			response, err := client.TriggerMemoryDream(cmd.Context(), MemoryDreamTriggerRequest{
				Scope:       selector.Scope,
				WorkspaceID: selector.WorkspaceID,
				AgentName:   selector.AgentName,
				AgentTier:   selector.AgentTier,
				Force:       force,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryDreamTriggerBundle(response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	cmd.Flags().BoolVar(&force, "force", false, "Trigger even if normal gates would skip the run")
	return cmd
}

func newMemoryDreamStatusCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   memoryStatusKey,
		Short: "Show Memory v2 dreaming runtime status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.GetMemoryDreamStatus(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryDreamListBundle(response))
		},
	}
}

func newMemoryDailyCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   memoryDailyKey,
		Short: "Inspect Memory v2 daily operation logs",
	}
	cmd.AddCommand(newMemoryDailyListCommand(deps))
	cmd.AddCommand(
		newMemoryDailyUnsupportedSelectorCommand(
			"show <date>",
			"memory.unsupported: daily show is not registered in Slice 1 API",
			exactOneNonBlankArg(),
		),
	)
	cmd.AddCommand(
		newMemoryDailyRetentionCommand("archive", "memory.unsupported: daily archive is not registered in Slice 1 API"),
	)
	cmd.AddCommand(
		newMemoryDailyUnsupportedSelectorCommand(
			"restore <date>",
			"memory.unsupported: daily restore is not registered in Slice 1 API",
			exactOneNonBlankArg(),
		),
	)
	cmd.AddCommand(
		newMemoryDailyRetentionCommand("purge", "memory.unsupported: daily purge is not registered in Slice 1 API"),
	)
	return cmd
}

func newMemoryDailyListCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List Memory v2 daily operation logs",
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
			response, err := client.ListMemoryDailyLogs(cmd.Context(), selector)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Daily Logs", response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	return cmd
}

func newMemoryExtractorCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   memoryExtractorKey,
		Short: "Operate Memory v2 extractor runtime",
	}
	cmd.AddCommand(newMemoryExtractorStatusCommand(deps))
	cmd.AddCommand(newMemoryExtractorListPendingCommand(deps))
	cmd.AddCommand(newMemoryExtractorReplayCommand(deps))
	cmd.AddCommand(newMemoryExtractorDrainCommand(deps))
	cmd.AddCommand(newMemoryExtractorDisableCommand())
	return cmd
}

func newMemoryExtractorStatusCommand(deps commandDeps) *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   memoryStatusKey,
		Short: "Show Memory v2 extractor runtime status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.GetMemoryExtractorStatus(cmd.Context(), sessionID)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Extractor Status", response))
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Filter extractor status by session")
	return cmd
}

func newMemoryExtractorListPendingCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list-pending",
		Short: "List Memory v2 extractor pending/DLQ records",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.ListMemoryExtractorFailures(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Extractor Pending", response))
		},
	}
}

func newMemoryExtractorReplayCommand(deps commandDeps) *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "replay --session <id>",
		Short: "Replay Memory v2 extractor work",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if strings.TrimSpace(sessionID) == "" {
				return errors.New("memory.extractor.session_required: --session is required")
			}
			response, err := client.RetryMemoryExtractor(cmd.Context(), MemoryExtractorRetryRequest{
				SessionID: strings.TrimSpace(sessionID),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Extractor Replay", response))
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session whose extractor work should be replayed")
	mustMarkFlagRequired(cmd, "session")
	return cmd
}

func newMemoryExtractorDrainCommand(deps commandDeps) *cobra.Command {
	var timeoutRaw string

	cmd := &cobra.Command{
		Use:   memoryDrainKey,
		Short: "Drain Memory v2 extractor work",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			timeout := 60 * time.Second
			if strings.TrimSpace(timeoutRaw) != "" {
				parsed, err := time.ParseDuration(strings.TrimSpace(timeoutRaw))
				if err != nil {
					return fmt.Errorf("memory.extractor.timeout_invalid: %w", err)
				}
				timeout = parsed
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			response, err := client.DrainMemoryExtractor(ctx)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryObjectBundle("Memory Extractor Drain", response))
		},
	}
	cmd.Flags().StringVar(&timeoutRaw, "timeout", "60s", "Maximum drain wait duration")
	return cmd
}

func newMemoryExtractorDisableCommand() *cobra.Command {
	var sessionID string
	cmd := newUnsupportedMemoryCommand(
		"disable --session <id>",
		"memory.unsupported: extractor disable is not registered in Slice 1 API",
		cobra.NoArgs,
	)
	cmd.Flags().StringVar(&sessionID, "session", "", "Session whose extractor should be disabled")
	mustMarkFlagRequired(cmd, "session")
	return cmd
}
