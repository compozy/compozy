package cli

import (
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	diagnosticitems "github.com/compozy/agh/internal/diagnostics"

	"github.com/spf13/cobra"
)

const (
	taskTypeValue = "Type"
)

const (
	taskAttemptValue              = "Attempt"
	taskBridgeValue               = "Bridge"
	taskClaimedByValue            = "Claimed By"
	taskParticipationChannelValue = "Participation Channel"
	taskCreatedValue              = "Created"
	taskCreatedByValue            = "Created By"
	taskDescriptionValue          = "Description"
	taskEndedValue                = "Ended"
	taskErrorValue                = "Error"
	taskIdentifierValue           = "Identifier"
	taskKindValue                 = "Kind"
	taskModeValue                 = "Mode"
	taskOriginValue               = "Origin"
	taskOutcomeValue              = "Outcome"
	taskOwnerValue                = "Owner"
	taskParentValue               = "Parent"
	taskQueuedValue               = "Queued"
	taskReasonValue               = "Reason"
	taskResultValue               = "Result"
	taskReviewValue               = "Review"
	taskRunValue                  = "Run"
	taskSandboxValue              = "Sandbox"
	taskScopeValue                = "Scope"
	taskSessionValue              = "Session"
	taskStartedValue              = "Started"
	taskStatusValue               = "Status"
	taskSubscriptionValue         = "Subscription"
	taskTaskValue                 = "Task"
	taskTaskIDValue               = "Task ID"
	taskTimeValue                 = "Time"
	taskTitleValue                = "Title"
	taskUpdatedValue              = "Updated"
	taskWorkspaceValue            = "Workspace"
	taskAttemptKey                = "attempt"
	taskAutoEnqueueOnReadyFlag    = "auto-enqueue-on-ready"
	taskClaimedByKey              = "claimed_by"
	taskCreateKey                 = "create"
	taskCreatedAtKey              = "created_at"
	createdByKey                  = "created_by"
	taskDeleteIDValue             = "delete <id>"
	taskDeletedKey                = "deleted"
	taskDescriptionKey            = "description"
	taskEndedAtKey                = "ended_at"
	taskErrorKey                  = "error"
	taskFailureKindKey            = "failure_kind"
	taskGetIDValue                = "get <id>"
	taskGroupIDKey                = "group_id"
	taskIdentifierKey             = "identifier"
	taskKindKey                   = "kind"
	taskListKey                   = "list"
	taskParticipationChannelKey   = "participation_channel"
	taskNextKey                   = "next"
	taskOriginKey                 = "origin"
	taskOutcomeKey                = "outcome"
	taskPeerIDKey                 = "peer_id"
	taskProfileKey                = "profile"
	taskQueuedAtKey               = "queued_at"
	taskReasonKey                 = "reason"
	taskReviewKey                 = "review"
	taskRunIDKey                  = "run_id"
	taskScopeKey                  = "scope"
	taskSessionIDKey              = "session_id"
	taskStartedAtKey              = "started_at"
	taskStatusKey                 = "status"
	taskTaskKey                   = "task"
	taskTaskIDKey                 = "task_id"
	taskTimestampKey              = "timestamp"
	taskTitleKey                  = "title"
	taskUpdateIDValue             = "update <id>"
	taskUpdatedAtKey              = "updated_at"
	taskWorkspaceIDKey            = "workspace_id"
)

type taskCreateInput struct {
	ID                 string
	Identifier         string
	ScopeRaw           string
	WorkspaceRef       string
	NetworkFlags       networkParticipationFlags
	Title              string
	Description        string
	PriorityRaw        string
	OwnerKindRaw       string
	OwnerRef           string
	MetadataRaw        string
	AutoEnqueueOnReady bool
	NoWakeCreator      bool
}

type taskBlockInput struct {
	KindRaw    string
	Reason     string
	DetailsRaw string
	ExpiresIn  string
	RunID      string
	AsAgent    bool
}

type taskExecutionInput struct {
	IdempotencyKey string
	NetworkFlags   networkParticipationFlags
	MetadataRaw    string
}

type taskReviewSubmitInput struct {
	RunID             string
	OutcomeRaw        string
	Confidence        float64
	Reason            string
	DeliveryID        string
	MissingWork       []string
	MissingWorkRaw    string
	NextRoundGuidance string
	ReviewText        string
}

type taskNotificationSubscribeInput struct {
	SubscriptionID   string
	BridgeInstanceID string
	ScopeRaw         string
	WorkspaceID      string
	PeerID           string
	ThreadID         string
	GroupID          string
	DeliveryModeRaw  string
}

type taskPromoteInput struct {
	WorkspaceRef    string
	Channel         string
	ThreadID        string
	OriginMessageID string
	Title           string
	Description     string
	Priority        string
	MetadataRaw     string
}

type taskFanOutInput struct {
	NetworkFlags   networkParticipationFlags
	Designations   []string
	IdempotencyKey string
}

type taskInspectTarget string

const (
	taskInspectTargetTask taskInspectTarget = "task"
	taskInspectTargetRun  taskInspectTarget = "run"
)

func taskInspectTargetForID(id string) taskInspectTarget {
	trimmed := strings.TrimSpace(id)
	switch {
	case strings.HasPrefix(trimmed, "task_"), strings.HasPrefix(trimmed, "task-"):
		return taskInspectTargetTask
	case strings.HasPrefix(trimmed, "run_"), strings.HasPrefix(trimmed, "run-"):
		return taskInspectTargetRun
	default:
		return ""
	}
}

func cliNow(now func() time.Time) time.Time {
	if now == nil {
		return time.Now().UTC()
	}
	return now().UTC()
}

func taskInspectUnknownIDRecord(id string, asOf time.Time) TaskInspectRecord {
	return TaskInspectRecord{
		Target:     providerModelAvailabilityUnknown,
		NextAction: providerModelAvailabilityUnknown,
		AsOf:       asOf,
		Diagnostics: []contract.DiagnosticItem{
			diagnosticitems.NewItem(
				"task.inspect.id_format_unknown",
				diagnosticcontract.CodeIDFormatUnknown,
				diagnosticcontract.CategoryTask,
				"Task inspect id format is unknown",
				"Task inspect accepts ids with task_ / task- or run_ / run- prefixes.",
				diagnosticcontract.SeverityError,
				diagnosticcontract.FreshnessLive,
				diagnosticitems.WithEvidence(map[string]any{"id": id}),
			),
		},
	}
}

func newTaskCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   taskTaskKey,
		Short: "Manage tasks and task runs",
		Example: `  # Create durable task intent without starting execution
  agh task create --scope workspace --workspace checkout-api --title "Audit auth flow"

  # Explicitly enqueue execution for an existing task
  agh task start task-123 --network live \
    --network-channel-strategy named --network-channel coord-run-123

  # Let the current agent session claim work
  agh task next --wait`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newTaskListCommand(deps))
	cmd.AddCommand(newTaskCreateCommand(deps))
	cmd.AddCommand(newTaskGetCommand(deps))
	cmd.AddCommand(newTaskInspectCommand(deps))
	cmd.AddCommand(newTaskUpdateCommand(deps))
	cmd.AddCommand(newTaskDeleteCommand(deps))
	cmd.AddCommand(newTaskProfileCommand(deps))
	cmd.AddCommand(newTaskReviewCommand(deps))
	cmd.AddCommand(newTaskNotificationCommand(deps))
	cmd.AddCommand(newTaskPublishCommand(deps))
	cmd.AddCommand(newTaskStartCommand(deps))
	cmd.AddCommand(newTaskApproveCommand(deps))
	cmd.AddCommand(newTaskRejectCommand(deps))
	cmd.AddCommand(newTaskCancelCommand(deps))
	cmd.AddCommand(newTaskBlockCommand(deps))
	cmd.AddCommand(newTaskUnblockCommand(deps))
	cmd.AddCommand(newTaskBlocksCommand(deps))
	cmd.AddCommand(newTaskNextCommand(deps))
	cmd.AddCommand(newTaskHeartbeatCommand(deps))
	cmd.AddCommand(newTaskCompleteCommand(deps))
	cmd.AddCommand(newTaskFailCommand(deps))
	cmd.AddCommand(newTaskReleaseCommand(deps))
	cmd.AddCommand(newTaskRetryCommand(deps))
	cmd.AddCommand(newTaskRecoverCommand(deps))
	cmd.AddCommand(newTaskPauseCommand(deps))
	cmd.AddCommand(newTaskResumeCommand(deps))
	cmd.AddCommand(newTaskPromoteCommand(deps))
	cmd.AddCommand(newTaskFanOutCommand(deps))
	cmd.AddCommand(newTaskChildCommand(deps))
	cmd.AddCommand(newTaskDependencyCommand(deps))
	cmd.AddCommand(newTaskRunCommand(deps))
	return cmd
}

func createTaskRecord(
	cmd *cobra.Command,
	deps commandDeps,
	client DaemonClient,
	request CreateTaskRequest,
	asAgent bool,
) (TaskRecord, error) {
	if !asAgent {
		return client.CreateTask(cmd.Context(), request)
	}
	credentials, err := requireAgentCommandIdentity(
		cmd.Context(),
		deps,
		client,
		agentActionCLI("task.create"),
	)
	if err != nil {
		return TaskRecord{}, err
	}
	return client.CreateTaskAsAgent(cmd.Context(), request, credentials)
}
