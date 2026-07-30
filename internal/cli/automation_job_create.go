package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

type automationJobCreateInput struct {
	automationCreateTargetInput
	Name                string
	ScopeRaw            string
	WorkspaceRef        string
	ScheduleRaw         string
	CatchUpPolicyRaw    string
	MisfireGraceSeconds int
	RetryRaw            string
	Enabled             bool
}

func runAutomationJobsCreate(
	cmd *cobra.Command,
	deps commandDeps,
	input automationJobCreateInput,
) error {
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	scope, workspaceID, err := resolveAutomationScopeWorkspace(
		cmd,
		deps,
		client,
		input.ScopeRaw,
		input.WorkspaceRef,
	)
	if err != nil {
		return err
	}
	schedule, err := parseAutomationScheduleFlag(input.ScheduleRaw)
	if err != nil {
		return err
	}
	if err := applyAutomationScheduleReliability(
		cmd,
		&schedule,
		automationScheduleReliabilityInput{
			CatchUpPolicyRaw:    input.CatchUpPolicyRaw,
			MisfireGraceSeconds: input.MisfireGraceSeconds,
		},
	); err != nil {
		return err
	}
	retry, err := parseAutomationRetryFlag(input.RetryRaw)
	if err != nil {
		return err
	}
	target, err := buildAutomationCreateTarget(
		cmd,
		deps,
		client,
		scope,
		workspaceID,
		input.automationCreateTargetInput,
		false,
	)
	if err != nil {
		return err
	}

	request := AutomationJobCreateRequest{
		Scope:       scope,
		Name:        strings.TrimSpace(input.Name),
		TargetKind:  target.Kind,
		AgentName:   target.AgentName,
		WorkspaceID: workspaceID,
		Prompt:      target.Prompt,
		Schedule:    schedule,
		LoopTarget:  target.LoopTarget,
	}
	if cmd.Flags().Changed(automationEnabledKey) {
		request.Enabled = new(input.Enabled)
	}
	if retry != nil {
		request.Retry = retry
	}

	created, err := client.CreateAutomationJob(cmd.Context(), request)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, automationJobBundle(created))
}
