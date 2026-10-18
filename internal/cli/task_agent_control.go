package cli

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newTaskRunRecoverCommand(deps commandDeps) *cobra.Command {
	var (
		reason      string
		metadataRaw string
	)
	cmd := &cobra.Command{
		Use:   "recover <run-id>",
		Short: "Recover one needs_attention task run",
		Args:  cobra.ExactArgs(1),
		Example: `  # Re-enqueue one run stuck in needs_attention
  compozy task run recover run-123 --reason "operator recovery"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runIDs, err := requiredTaskRunIDs(args)
			if err != nil {
				return err
			}
			request := RecoverTaskRunRequest{Reason: strings.TrimSpace(reason)}
			if cmd.Flags().Changed("metadata") {
				request.Metadata, err = parseAgentTaskJSONFlag("metadata", metadataRaw)
				if err != nil {
					return err
				}
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.RecoverTaskRun(cmd.Context(), runIDs[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, recoverTaskRunBundle(&record))
		},
	}
	cmd.Flags().
		StringVar(&reason, "reason", "", "Optional recovery reason recorded in the audit event")
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Optional recovery metadata JSON")
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func newTaskPauseCommand(deps commandDeps) *cobra.Command {
	var reason string
	var metadataRaw string
	cmd := &cobra.Command{
		Use:   "pause <task-id>",
		Short: "Pause new runs for one task",
		Args:  cobra.ExactArgs(1),
		Example: `  # Pause a noisy task while current claims finish
  compozy task pause task-123 --reason "provider incident"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID, err := requiredTaskID(args[0])
			if err != nil {
				return err
			}
			request := PauseTaskRequest{Reason: strings.TrimSpace(reason)}
			if request.Reason == "" {
				return errors.New("cli: --reason is required")
			}
			if cmd.Flags().Changed("metadata") {
				request.Metadata, err = parseAgentTaskJSONFlag("metadata", metadataRaw)
				if err != nil {
					return err
				}
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.PauseTask(cmd.Context(), taskID, request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBundle(&record))
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Task-pause reason")
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Optional task-pause metadata JSON")
	mustMarkFlagRequired(cmd, "reason")
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func newTaskResumeCommand(deps commandDeps) *cobra.Command {
	var metadataRaw string
	cmd := &cobra.Command{
		Use:   "resume <task-id>",
		Short: "Resume new runs for one paused task",
		Args:  cobra.ExactArgs(1),
		Example: `  # Re-enable scheduler claims for a task
  compozy task resume task-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID, err := requiredTaskID(args[0])
			if err != nil {
				return err
			}
			request := ResumeTaskRequest{}
			if cmd.Flags().Changed("metadata") {
				request.Metadata, err = parseAgentTaskJSONFlag("metadata", metadataRaw)
				if err != nil {
					return err
				}
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.ResumeTask(cmd.Context(), taskID, request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBundle(&record))
		},
	}
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Optional task-resume metadata JSON")
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func newTaskFanOutCommand(deps commandDeps) *cobra.Command {
	var input taskFanOutInput
	cmd := &cobra.Command{
		Use:     "fan-out <task-id>",
		Aliases: []string{"fanout"},
		Short:   "Enqueue designated sibling runs for one task",
		Example: `  # Enqueue two sibling runs in separate worktrees
  compozy task fan-out task_123 --worktree-per-run \
    --idempotency-key review-task-123-v1 \
    --designation "Review the API" --designation "Review the web app"

  # Output example
  compozy task fan-out task_123 --worktree-per-run --idempotency-key review-task-123-v1 --designation "Review the API"`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			designations, err := taskFanOutDesignations(input.Designations)
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.FanOutTaskRuns(cmd.Context(), args[0], FanOutTaskRunsRequest{
				Designations:   designations,
				IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
				WorktreePerRun: input.WorktreePerRun,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskFanOutRunsBundle(record))
		},
	}
	cmd.Flags().
		StringArrayVar(&input.Designations, "designation", nil, "Designation brief for one sibling run; repeatable")
	cmd.Flags().StringVar(&input.IdempotencyKey, "idempotency-key", "", "Fan-out idempotency key")
	cmd.Flags().BoolVar(&input.WorktreePerRun, "worktree-per-run", false, "Create a dedicated worktree for each run")
	mustMarkFlagRequired(cmd, "designation")
	mustMarkFlagRequired(cmd, "idempotency-key")
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func taskFanOutDesignations(
	values []string,
) ([]contract.TaskFanOutRunDesignationRequest, error) {
	trimmed := trimSpawnAtoms(values)
	if len(trimmed) == 0 {
		return nil, errors.New("cli: at least one --designation is required")
	}
	designations := make([]contract.TaskFanOutRunDesignationRequest, 0, len(trimmed))
	for _, brief := range trimmed {
		designations = append(designations, contract.TaskFanOutRunDesignationRequest{
			Brief: brief,
		})
	}
	return designations, nil
}

func newTaskChildCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "child",
		Short: "Manage child tasks",
	}
	cmd.AddCommand(newTaskChildCreateCommand(deps))
	return cmd
}
