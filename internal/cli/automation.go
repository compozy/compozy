package cli

import (
	"errors"

	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/spf13/cobra"
)

const (
	hooksValueKey = "value"
)

const (
	automationAgentValue     = "Agent"
	automationAttemptValue   = "Attempt"
	automationBodyValue      = "Body"
	automationCreatedValue   = "Created"
	automationEnabledValue   = "Enabled"
	automationEndedValue     = "Ended"
	automationErrorValue     = "Error"
	automationEventValue     = "Event"
	automationPathValue      = "Path"
	automationScopeValue     = "Scope"
	automationSessionValue   = "Session"
	automationSourceValue    = "Source"
	automationStartedValue   = "Started"
	automationStatusValue    = "Status"
	automationTargetValue    = "Target"
	automationUpdatedValue   = "Updated"
	automationWorkspaceValue = "Workspace"
	automationAgentNameKey   = "agent_name"
	automationAttemptKey     = "attempt"
	automationCreateKey      = "create"
	automationDeleteIDValue  = "delete <id>"
	automationEndedAtKey     = "ended_at"
	automationErrorKey       = "error"
	automationEventKey       = "event"
	automationGetIDValue     = "get <id>"
	automationHistoryIDValue = "history <id>"
	automationPathKey        = "path"
	automationPromptKey      = "prompt"
	automationRetryKey       = "retry"
	automationScheduleKey    = "schedule"
	automationScheduleValue  = "Schedule"
	automationScopeKey       = "scope"
	automationSourceKey      = "source"
	automationStartedAtKey   = "started_at"
	automationStatusKey      = "status"
	automationTargetKey      = "target"
	automationUpdateIDValue  = "update <id>"
	automationUpdatedAtKey   = "updated_at"
	automationWorkspaceIDKey = "workspace_id"
)

type automationTriggerCommandInput struct {
	Name               string
	ScopeRaw           string
	EventRaw           string
	WorkspaceRef       string
	AgentName          string
	Prompt             string
	RetryRaw           string
	FilterFlags        []string
	Enabled            bool
	WebhookID          string
	EndpointSlug       string
	WebhookSecretValue string
}

type automationJobUpdateInput struct {
	JobID               string
	Name                string
	Prompt              string
	ScheduleRaw         string
	CatchUpPolicyRaw    string
	MisfireGraceSeconds int
	RetryRaw            string
	Enabled             bool
}

func newAutomationJobsCreateCommand(deps commandDeps) *cobra.Command {
	input := automationJobCreateInput{}

	cmd := &cobra.Command{
		Use:   automationCreateKey,
		Short: "Create an automation job",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAutomationJobsCreate(cmd, deps, input)
		},
	}
	cmd.Flags().StringVar(&input.Name, automationNameKey, "", "Job name")
	cmd.Flags().StringVar(&input.ScopeRaw, automationScopeKey, "", "Job scope: global or workspace")
	cmd.Flags().
		StringVar(
			&input.ScheduleRaw,
			automationScheduleKey,
			"",
			"Schedule spec: <cron-expr>, every:<duration>, or at:<timestamp>",
		)
	bindAutomationScheduleReliabilityFlags(cmd, &input.CatchUpPolicyRaw, &input.MisfireGraceSeconds)
	cmd.Flags().StringVar(&input.AgentName, "agent", "", "Agent definition name")
	cmd.Flags().
		StringVar(&input.WorkspaceRef, "workspace", "", "Workspace path, name, or ID (required when --scope=workspace)")
	cmd.Flags().StringVar(&input.Prompt, automationPromptKey, "", "Prompt body to dispatch")
	cmd.Flags().
		StringVar(&input.RetryRaw, automationRetryKey, "", `Retry policy: "none", "backoff", or "backoff:<max_retries>:<base_delay>"`)
	cmd.Flags().BoolVar(&input.Enabled, automationEnabledKey, false, "Create the job enabled or disabled")
	mustMarkFlagRequired(cmd, automationNameKey)
	mustMarkFlagRequired(cmd, automationScopeKey)
	mustMarkFlagRequired(cmd, automationScheduleKey)
	mustMarkFlagRequired(cmd, "agent")
	mustMarkFlagRequired(cmd, automationPromptKey)
	return cmd
}

func newAutomationJobsGetCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   automationGetIDValue,
		Short: "Show one automation job",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			job, err := client.GetAutomationJob(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationJobBundle(job))
		},
	}
}

func newAutomationJobsUpdateCommand(deps commandDeps) *cobra.Command {
	var (
		name                string
		prompt              string
		scheduleRaw         string
		catchUpPolicyRaw    string
		misfireGraceSeconds int
		retryRaw            string
		enabled             bool
	)

	cmd := &cobra.Command{
		Use:   automationUpdateIDValue,
		Short: "Update an automation job",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			request, err := buildAutomationJobUpdateRequest(cmd, client, automationJobUpdateInput{
				JobID:               args[0],
				Name:                name,
				Prompt:              prompt,
				ScheduleRaw:         scheduleRaw,
				CatchUpPolicyRaw:    catchUpPolicyRaw,
				MisfireGraceSeconds: misfireGraceSeconds,
				RetryRaw:            retryRaw,
				Enabled:             enabled,
			})
			if err != nil {
				return err
			}
			if !request.HasChanges() {
				return errors.New("cli: automation job update requires at least one change flag")
			}

			updated, err := client.UpdateAutomationJob(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationJobBundle(updated))
		},
	}
	cmd.Flags().StringVar(&name, automationNameKey, "", "Update the job name")
	cmd.Flags().StringVar(&prompt, automationPromptKey, "", "Update the prompt body")
	cmd.Flags().StringVar(
		&scheduleRaw,
		automationScheduleKey,
		"",
		"Update the schedule spec; recurring reliability is preserved unless its flags are set",
	)
	bindAutomationScheduleReliabilityFlags(cmd, &catchUpPolicyRaw, &misfireGraceSeconds)
	cmd.Flags().
		StringVar(&retryRaw, automationRetryKey, "", `Update retry policy: "none", "backoff", or "backoff:<max_retries>:<base_delay>"`)
	cmd.Flags().BoolVar(&enabled, automationEnabledKey, false, "Update the enabled state")
	return cmd
}

func buildAutomationJobUpdateRequest(
	cmd *cobra.Command,
	client DaemonClient,
	input automationJobUpdateInput,
) (AutomationJobUpdateRequest, error) {
	request := AutomationJobUpdateRequest{}
	scheduleChanged := cmd.Flags().Changed(automationScheduleKey)
	reliabilityChanged := automationScheduleReliabilityChanged(cmd)
	if cmd.Flags().Changed(automationNameKey) {
		request.Name = new(strings.TrimSpace(input.Name))
	}
	if cmd.Flags().Changed(automationPromptKey) {
		request.Prompt = new(strings.TrimSpace(input.Prompt))
	}
	if scheduleChanged {
		schedule, err := parseAutomationScheduleFlag(input.ScheduleRaw)
		if err != nil {
			return AutomationJobUpdateRequest{}, err
		}
		request.Schedule = &schedule
	}

	var currentSchedule *automationpkg.ScheduleSpec
	if reliabilityChanged || scheduleChanged && request.Schedule.Mode != automationpkg.ScheduleModeAt {
		current, err := client.GetAutomationJob(cmd.Context(), input.JobID)
		if err != nil {
			return AutomationJobUpdateRequest{}, err
		}
		currentSchedule = current.Schedule
	}
	if scheduleChanged && request.Schedule.Mode != automationpkg.ScheduleModeAt && currentSchedule != nil {
		request.Schedule.CatchUpPolicy = currentSchedule.CatchUpPolicy
		request.Schedule.MisfireGraceSeconds = currentSchedule.MisfireGraceSeconds
	}
	if reliabilityChanged {
		if request.Schedule == nil {
			if currentSchedule == nil {
				return AutomationJobUpdateRequest{}, errors.New(
					"cli: automation job has no schedule to update reliability",
				)
			}
			schedule := *currentSchedule
			request.Schedule = &schedule
		}
		if err := applyAutomationScheduleReliability(
			cmd,
			request.Schedule,
			automationScheduleReliabilityInput{
				CatchUpPolicyRaw:    input.CatchUpPolicyRaw,
				MisfireGraceSeconds: input.MisfireGraceSeconds,
			},
		); err != nil {
			return AutomationJobUpdateRequest{}, err
		}
	}
	if cmd.Flags().Changed(automationRetryKey) {
		retry, err := parseAutomationRetryFlag(input.RetryRaw)
		if err != nil {
			return AutomationJobUpdateRequest{}, err
		}
		if retry == nil {
			return AutomationJobUpdateRequest{}, errors.New(
				`cli: --retry requires "none", "backoff", or "backoff:<max_retries>:<base_delay>"`,
			)
		}
		request.Retry = retry
	}
	if cmd.Flags().Changed(automationEnabledKey) {
		request.Enabled = new(input.Enabled)
	}
	return request, nil
}

func newAutomationJobsDeleteCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   automationDeleteIDValue,
		Short: "Delete an automation job",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			current, err := client.GetAutomationJob(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := client.DeleteAutomationJob(cmd.Context(), args[0]); err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationJobBundle(current))
		},
	}
}

func newAutomationJobsTriggerCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "trigger <id>",
		Short: "Force an immediate automation job run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			run, err := client.TriggerAutomationJob(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationRunBundle(run))
		},
	}
}

func newAutomationJobsHistoryCommand(deps commandDeps) *cobra.Command {
	var (
		statusRaw string
		sinceRaw  string
		untilRaw  string
		last      int
	)

	cmd := &cobra.Command{
		Use:   automationHistoryIDValue,
		Short: "Show run history for one automation job",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			query, err := parseAutomationRunListQuery(statusRaw, sinceRaw, untilRaw, last, deps.now)
			if err != nil {
				return err
			}

			runs, err := client.AutomationJobRuns(cmd.Context(), args[0], query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, automationRunListBundle(runs))
		},
	}
	cmd.Flags().StringVar(&statusRaw, automationStatusKey, "", "Filter by run status")
	cmd.Flags().
		StringVar(&sinceRaw, "since", "", "Show runs since an RFC3339 timestamp or relative duration")
	cmd.Flags().
		StringVar(&untilRaw, "until", "", "Show runs until an RFC3339 timestamp or relative duration")
	cmd.Flags().IntVar(&last, "last", 0, "Show only the most recent N runs")
	return cmd
}
