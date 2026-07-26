package cli

import (
	"errors"

	"strings"

	"github.com/spf13/cobra"
)

func newTaskChildCreateCommand(deps commandDeps) *cobra.Command {
	var (
		id           string
		identifier   string
		scopeRaw     string
		workspaceRef string
		networkFlags networkParticipationFlags
		title        string
		description  string
		priorityRaw  string
		ownerKindRaw string
		ownerRef     string
		metadataRaw  string
		autoEnqueue  bool
	)

	cmd := &cobra.Command{
		Use:   "create <parent-id>",
		Short: "Create a child task beneath a parent",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			baseRequest, err := buildTaskCreateRequest(cmd, taskCreateInput{
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
			})
			if err != nil {
				return err
			}
			request := CreateTaskChildRequest(baseRequest)

			created, err := client.CreateChildTask(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskBundle(&created))
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Explicit child task ID")
	cmd.Flags().
		StringVar(&identifier, taskIdentifierKey, "", "Human-friendly child task identifier")
	cmd.Flags().StringVar(&scopeRaw, taskScopeKey, "", "Child task scope: global or workspace")
	cmd.Flags().
		StringVar(&workspaceRef, "workspace", "", "Workspace path, name, or ID (required when --scope=workspace)")
	bindNetworkParticipationFlags(cmd, &networkFlags)
	cmd.Flags().StringVar(&title, taskTitleKey, "", "Child task title")
	cmd.Flags().StringVar(&description, taskDescriptionKey, "", "Child task description")
	cmd.Flags().
		StringVar(&priorityRaw, "priority", "", "Child task priority: low, medium, high, or urgent")
	cmd.Flags().StringVar(&ownerKindRaw, "owner-kind", "", "Optional child owner kind")
	cmd.Flags().StringVar(&ownerRef, "owner-ref", "", "Optional child owner reference")
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Optional child metadata JSON")
	cmd.Flags().
		BoolVar(&autoEnqueue, taskAutoEnqueueOnReadyFlag, false, "Auto-enqueue the child run once dependencies complete")
	mustMarkFlagRequired(cmd, taskScopeKey)
	mustMarkFlagRequired(cmd, taskTitleKey)
	return cmd
}

func newTaskDependencyCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dependency",
		Short: "Manage task dependencies",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newTaskDependencyAddCommand(deps))
	cmd.AddCommand(newTaskDependencyRemoveCommand(deps))
	return cmd
}

func newTaskDependencyAddCommand(deps commandDeps) *cobra.Command {
	var (
		dependsOnID string
		kindRaw     string
	)

	cmd := &cobra.Command{
		Use:   "add <task-id>",
		Short: "Add a dependency edge to a task",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			request := AddTaskDependencyRequest{DependsOnTaskID: strings.TrimSpace(dependsOnID)}
			if request.DependsOnTaskID == "" {
				return errors.New("cli: --depends-on is required")
			}
			if strings.TrimSpace(kindRaw) != "" {
				kind, err := parseOptionalTaskDependencyKind(kindRaw)
				if err != nil {
					return err
				}
				request.Kind = kind
			}

			updated, err := client.AddTaskDependency(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskDetailBundle(&updated))
		},
	}
	cmd.Flags().StringVar(&dependsOnID, "depends-on", "", "Dependency task ID")
	cmd.Flags().StringVar(&kindRaw, taskKindKey, "", "Dependency kind")
	mustMarkFlagRequired(cmd, "depends-on")
	return cmd
}

func newTaskDependencyRemoveCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <task-id> <depends-on-id>",
		Short: "Remove a dependency edge from a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			updated, err := client.RemoveTaskDependency(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskDetailBundle(&updated))
		},
	}
}

func newTaskRunCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Manage task runs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newTaskRunListCommand(deps))
	cmd.AddCommand(newTaskRunShowCommand(deps))
	cmd.AddCommand(newTaskRunEnqueueCommand(deps))
	cmd.AddCommand(newTaskRunStartCommand(deps))
	cmd.AddCommand(newTaskRunAttachSessionCommand(deps))
	cmd.AddCommand(newTaskRunCompleteCommand(deps))
	cmd.AddCommand(newTaskRunFailCommand(deps))
	cmd.AddCommand(newTaskRunCancelCommand(deps))
	cmd.AddCommand(newTaskRunRecoverCommand(deps))
	return cmd
}

func newTaskRunListCommand(deps commandDeps) *cobra.Command {
	var (
		statusRaw               string
		sessionID               string
		participationChannelRaw string
		last                    int
	)

	cmd := &cobra.Command{
		Use:   "list <task-id>",
		Short: "List runs for a task",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			query, err := parseTaskRunListFilters(
				statusRaw,
				sessionID,
				participationChannelRaw,
				last,
			)
			if err != nil {
				return err
			}

			runs, err := client.ListTaskRuns(cmd.Context(), args[0], query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunListBundle(runs))
		},
	}
	cmd.Flags().StringVar(&statusRaw, taskStatusKey, "", "Filter by run status")
	cmd.Flags().StringVar(&sessionID, "session", "", "Filter by attached session ID")
	cmd.Flags().StringVar(
		&participationChannelRaw,
		"participation-channel",
		"",
		"Filter by resolved Network participation channel",
	)
	cmd.Flags().IntVar(&last, "last", 0, "Show only the most recent N runs")
	return cmd
}

func newTaskRunShowCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show a task run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			detail, err := client.GetTaskRun(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunDetailBundle(&detail))
		},
	}
}

func newTaskRunEnqueueCommand(deps commandDeps) *cobra.Command {
	var (
		idempotencyKey string
		networkFlags   networkParticipationFlags
		metadataRaw    string
	)

	cmd := &cobra.Command{
		Use:   "enqueue <task-id>",
		Short: "Enqueue a task run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			participationRequest, err := networkFlags.request()
			if err != nil {
				return err
			}

			request := EnqueueTaskRunRequest{
				IdempotencyKey:       strings.TrimSpace(idempotencyKey),
				NetworkParticipation: participationRequest,
			}
			if cmd.Flags().Changed("metadata") {
				request.Metadata, err = parseJSONFlag("metadata", metadataRaw)
				if err != nil {
					return err
				}
			}

			run, err := client.EnqueueTaskRun(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunBundle(run))
		},
	}
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	bindNetworkParticipationFlags(cmd, &networkFlags)
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Optional run metadata JSON")
	return cmd
}

func newTaskRunStartCommand(deps commandDeps) *cobra.Command {
	var idempotencyKey string

	cmd := &cobra.Command{
		Use:   "start <run-id>",
		Short: "Start a claimed task run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			run, err := client.StartTaskRun(cmd.Context(), args[0], StartTaskRunRequest{
				IdempotencyKey: strings.TrimSpace(idempotencyKey),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunBundle(run))
		},
	}
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	return cmd
}

func newTaskRunAttachSessionCommand(deps commandDeps) *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "attach-session <run-id>",
		Short: "Attach an existing session to a claimed or starting run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if strings.TrimSpace(sessionID) == "" {
				return errors.New("cli: --session is required")
			}

			run, err := client.AttachTaskRunSession(
				cmd.Context(),
				args[0],
				AttachTaskRunSessionRequest{
					SessionID: strings.TrimSpace(sessionID),
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunBundle(run))
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Existing session ID to attach")
	mustMarkFlagRequired(cmd, "session")
	return cmd
}

func newTaskRunCompleteCommand(deps commandDeps) *cobra.Command {
	var resultRaw string

	cmd := &cobra.Command{
		Use:   "complete <run-id>",
		Short: "Complete a running task run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			request := CompleteTaskRunRequest{}
			if cmd.Flags().Changed("result") {
				request.Result, err = parseJSONFlag("result", resultRaw)
				if err != nil {
					return err
				}
			}

			run, err := client.CompleteTaskRun(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunBundle(run))
		},
	}
	cmd.Flags().StringVar(&resultRaw, "result", "", "Optional result JSON")
	return cmd
}

func newTaskRunFailCommand(deps commandDeps) *cobra.Command {
	var (
		errorMessage string
		metadataRaw  string
	)

	cmd := &cobra.Command{
		Use:   "fail <run-id>",
		Short: "Fail a task run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			request := FailTaskRunRequest{Error: strings.TrimSpace(errorMessage)}
			if request.Error == "" {
				return errors.New("cli: --error is required")
			}
			if cmd.Flags().Changed("metadata") {
				request.Metadata, err = parseJSONFlag("metadata", metadataRaw)
				if err != nil {
					return err
				}
			}

			run, err := client.FailTaskRun(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunBundle(run))
		},
	}
	cmd.Flags().StringVar(&errorMessage, taskErrorKey, "", "Failure message")
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Optional failure metadata JSON")
	mustMarkFlagRequired(cmd, taskErrorKey)
	return cmd
}

func newTaskRunCancelCommand(deps commandDeps) *cobra.Command {
	var (
		reason      string
		metadataRaw string
	)

	cmd := &cobra.Command{
		Use:   "cancel <run-id>",
		Short: "Cancel a task run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			request := CancelTaskRunRequest{Reason: strings.TrimSpace(reason)}
			if cmd.Flags().Changed("metadata") {
				request.Metadata, err = parseJSONFlag("metadata", metadataRaw)
				if err != nil {
					return err
				}
			}

			run, err := client.CancelTaskRun(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunBundle(run))
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Optional cancellation reason")
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Optional cancellation metadata JSON")
	return cmd
}

func parseTaskRunListFilters(
	statusRaw string,
	sessionID string,
	participationChannelRaw string,
	last int,
) (TaskRunListQuery, error) {
	status, err := parseOptionalTaskRunStatus(statusRaw)
	if err != nil {
		return TaskRunListQuery{}, err
	}
	if err := validateTaskLast(last); err != nil {
		return TaskRunListQuery{}, err
	}
	if err := validateTaskParticipationChannelFlag(participationChannelRaw); err != nil {
		return TaskRunListQuery{}, err
	}
	return TaskRunListQuery{
		Status:               status,
		SessionID:            strings.TrimSpace(sessionID),
		ParticipationChannel: strings.TrimSpace(participationChannelRaw),
		Limit:                last,
	}, nil
}
