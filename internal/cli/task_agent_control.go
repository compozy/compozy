package cli

import (
	"errors"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

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
  agh task run recover run-123 --reason "operator recovery"`,
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
  agh task pause task-123 --reason "provider incident"`,
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
	return cmd
}

func newTaskResumeCommand(deps commandDeps) *cobra.Command {
	var metadataRaw string

	cmd := &cobra.Command{
		Use:   "resume <task-id>",
		Short: "Resume new runs for one paused task",
		Args:  cobra.ExactArgs(1),
		Example: `  # Re-enable scheduler claims for a task
  agh task resume task-123`,
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
	return cmd
}

func newTaskPromoteCommand(deps commandDeps) *cobra.Command {
	var input taskPromoteInput
	cmd := &cobra.Command{
		Use:   networkPromoteCommandUse,
		Short: "Promote one network thread message into a task",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			metadata, err := parseOptionalNetworkMetadata(cmd, input.MetadataRaw)
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveCLIWorkspaceRouteRef(
				cmd.Context(),
				deps,
				client,
				strings.TrimSpace(input.WorkspaceRef),
			)
			if err != nil {
				return err
			}
			promoted, err := client.PromoteNetworkThreadTask(
				cmd.Context(),
				workspace,
				strings.TrimSpace(input.Channel),
				strings.TrimSpace(input.ThreadID),
				PromoteNetworkThreadTaskRequest{
					OriginMessageID: strings.TrimSpace(input.OriginMessageID),
					Title:           strings.TrimSpace(input.Title),
					Description:     strings.TrimSpace(input.Description),
					Priority:        strings.TrimSpace(input.Priority),
					Metadata:        metadata,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkThreadPromotionBundle(&promoted))
		},
	}
	cmd.Flags().StringVar(&input.WorkspaceRef, "workspace", "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(&input.Channel, networkChannelKey, "", "Source network channel")
	cmd.Flags().StringVar(&input.ThreadID, networkSurfaceThread, "", "Source public thread id")
	cmd.Flags().StringVar(&input.OriginMessageID, "origin-message", "", "Origin message id to promote")
	cmd.Flags().StringVar(&input.Title, taskTitleKey, "", "Optional task title")
	cmd.Flags().StringVar(&input.Description, taskDescriptionKey, "", "Optional task description")
	cmd.Flags().StringVar(&input.Priority, "priority", "", "Optional task priority")
	cmd.Flags().StringVar(&input.MetadataRaw, "metadata", "", "Optional JSON metadata for the promoted task")
	mustMarkFlagRequired(cmd, "workspace")
	mustMarkFlagRequired(cmd, networkChannelKey)
	mustMarkFlagRequired(cmd, networkSurfaceThread)
	mustMarkFlagRequired(cmd, "origin-message")
	return cmd
}

func newTaskFanOutCommand(deps commandDeps) *cobra.Command {
	var input taskFanOutInput
	cmd := &cobra.Command{
		Use:     "fan-out <task-id>",
		Aliases: []string{"fanout"},
		Short:   "Enqueue designated sibling runs for one task",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			designations, err := taskFanOutDesignations(input.Designations)
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			participationRequest, err := input.NetworkFlags.request()
			if err != nil {
				return err
			}
			record, err := client.FanOutTaskRuns(cmd.Context(), args[0], FanOutTaskRunsRequest{
				NetworkParticipation: participationRequest,
				Designations:         designations,
				IdempotencyKey:       strings.TrimSpace(input.IdempotencyKey),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskFanOutRunsBundle(record))
		},
	}
	bindNetworkParticipationFlags(cmd, &input.NetworkFlags)
	cmd.Flags().
		StringArrayVar(&input.Designations, "designation", nil, "Designation brief for one sibling run; repeatable")
	cmd.Flags().StringVar(&input.IdempotencyKey, "idempotency-key", "", "Optional fan-out idempotency key")
	mustMarkFlagRequired(cmd, "designation")
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newTaskChildCreateCommand(deps))
	return cmd
}
