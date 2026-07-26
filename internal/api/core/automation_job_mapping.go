package core

import (
	"errors"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
)

// AutomationJobFromCreateRequest converts the shared create payload into the
// canonical automation job model.
func AutomationJobFromCreateRequest(req contract.CreateJobRequest) (automationpkg.Job, error) {
	return jobFromCreateRequest(req)
}

func jobFromCreateRequest(req contract.CreateJobRequest) (automationpkg.Job, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	retry := automationpkg.DefaultRetryConfig()
	if req.Retry != nil {
		retry = *req.Retry
	}

	fireLimit := automationpkg.DefaultFireLimitConfig()
	if req.FireLimit != nil {
		fireLimit = *req.FireLimit
	}

	schedule := req.Schedule
	taskConfig, err := cloneAutomationJobTaskConfig(req.Task)
	if err != nil {
		return automationpkg.Job{}, err
	}
	return automationpkg.Job{
		Scope:       req.Scope,
		Name:        strings.TrimSpace(req.Name),
		TargetKind:  req.TargetKind,
		AgentName:   strings.TrimSpace(req.AgentName),
		WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		Prompt:      strings.TrimSpace(req.Prompt),
		Schedule:    &schedule,
		Task:        taskConfig,
		LoopTarget:  cloneAutomationLoopTarget(req.LoopTarget),
		Enabled:     enabled,
		Retry:       retry,
		FireLimit:   fireLimit,
		Source:      automationpkg.JobSourceDynamic,
	}, nil
}

// ApplyAutomationJobPatch applies the shared patch payload to an automation job model.
func ApplyAutomationJobPatch(
	current automationpkg.Job,
	req contract.UpdateJobRequest,
) (automationpkg.Job, error) {
	return applyJobPatch(current, req)
}

func applyJobPatch(current automationpkg.Job, req contract.UpdateJobRequest) (automationpkg.Job, error) {
	next := current
	if req.Name != nil {
		next.Name = strings.TrimSpace(*req.Name)
	}
	if req.Prompt != nil {
		next.Prompt = strings.TrimSpace(*req.Prompt)
	}
	if req.Schedule != nil {
		schedule := *req.Schedule
		next.Schedule = &schedule
	}
	if req.Task != nil {
		taskConfig, err := cloneAutomationJobTaskConfig(req.Task)
		if err != nil {
			return automationpkg.Job{}, err
		}
		next.Task = taskConfig
	}
	if req.LoopTarget != nil {
		next.LoopTarget = cloneAutomationLoopTarget(req.LoopTarget)
	}
	if req.Enabled != nil {
		next.Enabled = *req.Enabled
	}
	if req.Retry != nil {
		next.Retry = *req.Retry
	}
	if req.FireLimit != nil {
		next.FireLimit = *req.FireLimit
	}
	if err := automationpkg.ValidateImmutableJobTarget(current, next); err != nil {
		return automationpkg.Job{}, err
	}
	return next, nil
}

// ValidateAutomationManagedJobUpdate enforces the managed job mutation policy.
func ValidateAutomationManagedJobUpdate(req contract.UpdateJobRequest) error {
	return validateManagedJobUpdate(req)
}

func validateManagedJobUpdate(req contract.UpdateJobRequest) error {
	switch {
	case req.Enabled == nil:
		return errors.New("managed automation jobs only accept enabled updates")
	case req.Name != nil ||
		req.Prompt != nil ||
		req.Schedule != nil ||
		req.Task != nil ||
		req.LoopTarget != nil ||
		req.Retry != nil ||
		req.FireLimit != nil:
		return errors.New("managed automation jobs only accept enabled updates")
	default:
		return nil
	}
}
