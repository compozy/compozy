package cli

import (
	"context"

	"errors"
	"fmt"

	"strings"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/spf13/cobra"
)

func newTaskRejectCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reject <id>",
		Short: "Reject a pending approval task",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			rejected, err := client.RejectTask(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBundle(&rejected))
		},
	}
	return cmd
}

func newTaskExecutionCommand(
	deps commandDeps,
	use string,
	short string,
	execute func(context.Context, DaemonClient, string, TaskExecutionRequest) (TaskExecutionRecord, error),
) *cobra.Command {
	var input taskExecutionInput
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			request, err := buildTaskExecutionRequest(cmd, input)
			if err != nil {
				return err
			}
			execution, err := execute(cmd.Context(), client, args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskExecutionBundle(&execution))
		},
	}
	cmd.Flags().StringVar(&input.IdempotencyKey, "idempotency-key", "", "Optional idempotency key")
	bindNetworkParticipationFlags(cmd, &input.NetworkFlags)
	cmd.Flags().StringVar(&input.MetadataRaw, "metadata", "", "Optional run metadata JSON")
	return cmd
}

func buildTaskExecutionRequest(
	cmd *cobra.Command,
	input taskExecutionInput,
) (TaskExecutionRequest, error) {
	participationRequest, err := input.NetworkFlags.request()
	if err != nil {
		return TaskExecutionRequest{}, err
	}
	request := TaskExecutionRequest{
		IdempotencyKey:       strings.TrimSpace(input.IdempotencyKey),
		NetworkParticipation: participationRequest,
	}
	if cmd.Flags().Changed("metadata") {
		metadata, err := parseJSONFlag("metadata", input.MetadataRaw)
		if err != nil {
			return TaskExecutionRequest{}, err
		}
		request.Metadata = metadata
	}
	return request, nil
}

func buildTaskBlockRequest(
	cmd *cobra.Command,
	input taskBlockInput,
	now time.Time,
) (CreateTaskBlockRequest, error) {
	kind := taskpkg.BlockKind(strings.TrimSpace(input.KindRaw)).Normalize()
	if err := kind.Validate("task_block.kind"); err != nil {
		return CreateTaskBlockRequest{}, fmt.Errorf("cli: %w", err)
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return CreateTaskBlockRequest{}, errors.New("cli: --reason is required")
	}
	request := CreateTaskBlockRequest{
		Kind:   kind,
		Reason: reason,
		RunID:  strings.TrimSpace(input.RunID),
	}
	if cmd.Flags().Changed("details") {
		details, err := parseAgentTaskJSONFlag("details", input.DetailsRaw)
		if err != nil {
			return CreateTaskBlockRequest{}, err
		}
		request.Details = details
	}
	if strings.TrimSpace(input.ExpiresIn) != "" {
		duration, err := time.ParseDuration(strings.TrimSpace(input.ExpiresIn))
		if err != nil {
			return CreateTaskBlockRequest{}, fmt.Errorf("cli: invalid --expires-in duration: %w", err)
		}
		if duration <= 0 {
			return CreateTaskBlockRequest{}, errors.New("cli: --expires-in must be positive")
		}
		expiresAt := now.Add(duration).UTC()
		request.ExpiresAt = &expiresAt
	}
	return request, nil
}

func newTaskCancelCommand(deps commandDeps) *cobra.Command {
	var (
		reason      string
		metadataRaw string
	)

	cmd := &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel a task tree",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			request := CancelTaskRequest{Reason: strings.TrimSpace(reason)}
			if cmd.Flags().Changed("metadata") {
				request.Metadata, err = parseJSONFlag("metadata", metadataRaw)
				if err != nil {
					return err
				}
			}

			canceled, err := client.CancelTask(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBundle(&canceled))
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Optional cancellation reason")
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Optional cancellation metadata JSON")
	return cmd
}

func newTaskBlockCommand(deps commandDeps) *cobra.Command {
	input := taskBlockInput{}
	cmd := &cobra.Command{
		Use:   "block <id>",
		Short: "Block a task with a typed reason",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			request, err := buildTaskBlockRequest(cmd, input, cliNow(deps.now))
			if err != nil {
				return err
			}
			if request.RunID != "" && !input.AsAgent {
				return errors.New("cli: --run-id requires --as-agent so the active lease can be resolved")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			var block TaskBlockRecord
			if input.AsAgent {
				credentials, credErr := requireAgentCommandIdentity(
					cmd.Context(),
					deps,
					client,
					agentActionCLI("task.block"),
				)
				if credErr != nil {
					return credErr
				}
				block, err = client.BlockTaskAsAgent(cmd.Context(), args[0], request, credentials)
			} else {
				block, err = client.BlockTask(cmd.Context(), args[0], request)
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBlockBundle(block))
		},
	}
	cmd.Flags().StringVar(&input.KindRaw, taskKindKey, "", "Block kind: needs_input, capability, or transient")
	cmd.Flags().StringVar(&input.Reason, taskReasonKey, "", "Block reason")
	cmd.Flags().StringVar(&input.DetailsRaw, "details", "", "Optional block details JSON")
	cmd.Flags().StringVar(&input.ExpiresIn, "expires-in", "", "Optional transient block duration")
	cmd.Flags().StringVar(&input.RunID, "run-id", "", "Active run ID to park when blocking")
	cmd.Flags().BoolVar(&input.AsAgent, "as-agent", false, "Block using the current AGH-managed agent session identity")
	mustMarkFlagRequired(cmd, taskKindKey)
	mustMarkFlagRequired(cmd, taskReasonKey)
	return cmd
}

func newTaskUnblockCommand(deps commandDeps) *cobra.Command {
	var blockID string
	var note string
	var asAgent bool
	cmd := &cobra.Command{
		Use:   "unblock <id>",
		Short: "Clear one task block",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(blockID) == "" {
				return errors.New("cli: --block is required")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			request := ClearTaskBlockRequest{Note: strings.TrimSpace(note)}
			var block TaskBlockRecord
			if asAgent {
				credentials, credErr := requireAgentCommandIdentity(
					cmd.Context(),
					deps,
					client,
					agentActionCLI("task.unblock"),
				)
				if credErr != nil {
					return credErr
				}
				block, err = client.ClearTaskBlockAsAgent(cmd.Context(), args[0], blockID, request, credentials)
			} else {
				block, err = client.ClearTaskBlock(cmd.Context(), args[0], blockID, request)
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBlockBundle(block))
		},
	}
	cmd.Flags().StringVar(&blockID, "block", "", "Task block ID")
	cmd.Flags().StringVar(&note, "note", "", "Optional clear note")
	cmd.Flags().BoolVar(&asAgent, "as-agent", false, "Clear using the current AGH-managed agent session identity")
	mustMarkFlagRequired(cmd, "block")
	return cmd
}

func newTaskBlocksCommand(deps commandDeps) *cobra.Command {
	var includeCleared bool
	cmd := &cobra.Command{
		Use:   "blocks <id>",
		Short: "List task blocks",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			blocks, err := client.ListTaskBlocks(cmd.Context(), args[0], includeCleared)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBlockListBundle(blocks))
		},
	}
	cmd.Flags().BoolVar(&includeCleared, "all", false, "Include cleared blocks")
	return cmd
}

func newTaskRecoverCommand(deps commandDeps) *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "recover <id>",
		Short: "Recover a task from needs_attention",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			recovered, err := client.RecoverTask(
				cmd.Context(),
				args[0],
				RecoverTaskRequest{Note: strings.TrimSpace(note)},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBundle(&recovered))
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "Optional recovery note")
	return cmd
}
