package cli

import (
	"context"
	"encoding/json"

	"fmt"

	"strings"

	"github.com/spf13/cobra"
)

func newTaskCreateCommand(deps commandDeps) *cobra.Command {
	var (
		id            string
		identifier    string
		scopeRaw      string
		workspaceRef  string
		networkFlags  networkParticipationFlags
		title         string
		description   string
		ownerKindRaw  string
		ownerRef      string
		metadataRaw   string
		priorityRaw   string
		asAgent       bool
		autoEnqueue   bool
		noWakeCreator bool
	)

	cmd := &cobra.Command{
		Use:   taskCreateKey,
		Short: "Create a task",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			request, err := buildTaskCreateRequest(cmd, taskCreateInput{
				ID:                 id,
				Identifier:         identifier,
				ScopeRaw:           scopeRaw,
				WorkspaceRef:       workspaceRef,
				NetworkFlags:       networkFlags,
				Title:              title,
				Description:        description,
				PriorityRaw:        priorityRaw,
				OwnerKindRaw:       ownerKindRaw,
				OwnerRef:           ownerRef,
				MetadataRaw:        metadataRaw,
				AutoEnqueueOnReady: autoEnqueue,
				NoWakeCreator:      noWakeCreator,
			})
			if err != nil {
				return err
			}

			created, err := createTaskRecord(cmd, deps, client, request, asAgent)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBundle(&created))
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Explicit task ID")
	cmd.Flags().StringVar(&identifier, taskIdentifierKey, "", "Human-friendly task identifier")
	cmd.Flags().StringVar(&scopeRaw, taskScopeKey, "", "Task scope: global or workspace")
	cmd.Flags().
		StringVar(&workspaceRef, "workspace", "", "Workspace path, name, or ID (required when --scope=workspace)")
	bindNetworkParticipationFlags(cmd, &networkFlags)
	cmd.Flags().StringVar(&title, taskTitleKey, "", "Task title")
	cmd.Flags().StringVar(&description, taskDescriptionKey, "", "Task description")
	cmd.Flags().
		StringVar(&priorityRaw, "priority", "", "Task priority: low, medium, high, or urgent")
	cmd.Flags().StringVar(&ownerKindRaw, "owner-kind", "", "Optional owner kind")
	cmd.Flags().StringVar(&ownerRef, "owner-ref", "", "Optional owner reference")
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Optional metadata JSON")
	cmd.Flags().
		BoolVar(&asAgent, "as-agent", false, "Create using the current AGH-managed agent session identity")
	cmd.Flags().
		BoolVar(&autoEnqueue, taskAutoEnqueueOnReadyFlag, false, "Auto-enqueue the run once blocking dependencies complete")
	cmd.Flags().
		BoolVar(&noWakeCreator, "no-wake-creator", false, "Disable creator wake notifications for this task")
	mustMarkFlagRequired(cmd, taskScopeKey)
	mustMarkFlagRequired(cmd, taskTitleKey)
	return cmd
}

func newTaskGetCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   taskGetIDValue,
		Short: "Show one task with related detail",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			taskDetail, err := client.GetTask(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskDetailBundle(&taskDetail))
		},
	}
}

func newTaskInspectCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <id>",
		Short: "Inspect a task or run with diagnostics",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			id := strings.TrimSpace(args[0])
			var inspect TaskInspectRecord
			switch taskInspectTargetForID(id) {
			case taskInspectTargetTask:
				inspect, err = client.InspectTask(cmd.Context(), id)
			case taskInspectTargetRun:
				inspect, err = client.InspectRun(cmd.Context(), id)
			default:
				inspect = taskInspectUnknownIDRecord(id, cliNow(deps.now))
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskInspectBundle(&inspect))
		},
	}
}

func newTaskPublishCommand(deps commandDeps) *cobra.Command {
	return newTaskExecutionCommand(
		deps,
		"publish <id>",
		"Publish a draft task and enqueue its first run",
		func(ctx context.Context, client DaemonClient, id string, request TaskExecutionRequest) (TaskExecutionRecord, error) {
			return client.PublishTask(ctx, id, request)
		},
	)
}

func newTaskStartCommand(deps commandDeps) *cobra.Command {
	return newTaskExecutionCommand(
		deps,
		"start <id>",
		"Enqueue a run for an executable task",
		func(ctx context.Context, client DaemonClient, id string, request TaskExecutionRequest) (TaskExecutionRecord, error) {
			return client.StartTask(ctx, id, request)
		},
	)
}

func newTaskApproveCommand(deps commandDeps) *cobra.Command {
	return newTaskExecutionCommand(
		deps,
		"approve <id>",
		"Approve a task and enqueue its first run",
		func(ctx context.Context, client DaemonClient, id string, request TaskExecutionRequest) (TaskExecutionRecord, error) {
			return client.ApproveTask(ctx, id, request)
		},
	)
}

func newTaskDeleteCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   taskDeleteIDValue,
		Short: "Delete a task",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if err := client.DeleteTask(cmd.Context(), args[0]); err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskDeleteBundle(args[0]))
		},
	}
	return cmd
}

func newTaskProfileCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   taskProfileKey,
		Short: "Manage task execution profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newTaskProfileInspectCommand(deps))
	cmd.AddCommand(newTaskProfileUpdateCommand(deps))
	cmd.AddCommand(newTaskProfileDeleteCommand(deps))
	return cmd
}

func newTaskProfileInspectCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <id>",
		Short: "Show one task execution profile",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			profile, err := client.GetTaskExecutionProfile(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskExecutionProfileBundle(&profile))
		},
	}
}

func newTaskProfileUpdateCommand(deps commandDeps) *cobra.Command {
	var profileRaw string
	cmd := &cobra.Command{
		Use:   taskUpdateIDValue,
		Short: "Replace one task execution profile",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			request, err := buildTaskExecutionProfileRequest(args[0], profileRaw)
			if err != nil {
				return err
			}
			profile, err := client.SetTaskExecutionProfile(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskExecutionProfileBundle(&profile))
		},
	}
	cmd.Flags().StringVar(&profileRaw, taskProfileKey, "", "Task execution profile JSON")
	mustMarkFlagRequired(cmd, taskProfileKey)
	return cmd
}

func newTaskProfileDeleteCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   taskDeleteIDValue,
		Short: "Delete one task execution profile",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if err := client.DeleteTaskExecutionProfile(cmd.Context(), args[0]); err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskExecutionProfileDeleteBundle(args[0]))
		},
	}
}

func buildTaskExecutionProfileRequest(
	taskID string,
	raw string,
) (*TaskExecutionProfileRequest, error) {
	payload, err := parseJSONFlag(taskProfileKey, raw)
	if err != nil {
		return nil, err
	}
	var request TaskExecutionProfileRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, fmt.Errorf("cli: parse --profile JSON: %w", err)
	}
	trimmedID := strings.TrimSpace(taskID)
	if strings.TrimSpace(request.TaskID) != "" && strings.TrimSpace(request.TaskID) != trimmedID {
		return nil, fmt.Errorf(
			"cli: profile.task_id must match task id %q",
			trimmedID,
		)
	}
	request.TaskID = trimmedID
	return &request, nil
}
