package extensionpkg

import (
	"context"

	"errors"
	"fmt"

	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"

	"github.com/compozy/agh/internal/session"
)

func (h *HostAPIHandler) requireHostAPISessionWorkspace(
	ctx context.Context,
	workspaceRef string,
	sessionID string,
) (*session.Info, error) {
	if h.sessions == nil {
		return nil, errors.New("extension: session manager is not configured")
	}
	id := strings.TrimSpace(sessionID)
	if id == "" {
		return nil, invalidParamsRPCError(errors.New("session_id is required"))
	}
	workspaceID, err := h.resolveRequiredWorkspaceID(ctx, workspaceRef)
	if err != nil {
		return nil, err
	}
	info, err := h.sessions.Status(ctx, id)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return nil, notFoundRPCError("session", id, err)
		}
		return nil, err
	}
	if info == nil || strings.TrimSpace(info.WorkspaceID) != workspaceID {
		return nil, notFoundRPCError(
			"session",
			id,
			fmt.Errorf("%w: session=%q workspace_id=%q", session.ErrSessionNotFound, id, workspaceID),
		)
	}
	return info, nil
}

func (h *HostAPIHandler) automationManager() (HostAPIAutomationManager, error) {
	if h == nil {
		return nil, errors.New("extension: host api handler is required")
	}
	if h.automation != nil {
		return h.automation, nil
	}
	if h.automationGetter != nil {
		if automation := h.automationGetter(); automation != nil {
			return automation, nil
		}
	}
	return nil, errors.New("extension: automation manager is not configured")
}

func (h *HostAPIHandler) resolveAutomationWorkspaceID(ctx context.Context, rawWorkspace string) (string, error) {
	trimmed := strings.TrimSpace(rawWorkspace)
	if trimmed == "" {
		return "", nil
	}
	if h.workspaces == nil {
		return trimmed, nil
	}
	resolved, err := h.workspaces.Resolve(ctx, trimmed)
	if err != nil {
		return "", err
	}
	return hostAPIResolvedWorkspaceRegistrationID(&resolved)
}

func (h *HostAPIHandler) jobFromCreateParams(
	ctx context.Context,
	req hostAPIAutomationJobCreateParams,
) (automationpkg.Job, error) {
	workspaceID, err := h.resolveAutomationWorkspaceID(ctx, req.WorkspaceID)
	if err != nil {
		return automationpkg.Job{}, err
	}

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
	return automationpkg.Job{
		Scope:       req.Scope,
		Name:        strings.TrimSpace(req.Name),
		TargetKind:  req.TargetKind,
		AgentName:   strings.TrimSpace(req.AgentName),
		WorkspaceID: workspaceID,
		Prompt:      strings.TrimSpace(req.Prompt),
		Schedule:    &schedule,
		Task:        cloneBundleTaskConfig(req.Task),
		LoopTarget:  cloneHostAPIAutomationLoopTarget(req.LoopTarget),
		Enabled:     enabled,
		Retry:       retry,
		FireLimit:   fireLimit,
		Source:      automationpkg.JobSourceDynamic,
	}, nil
}

func (h *HostAPIHandler) applyJobUpdateParams(
	_ context.Context,
	current automationpkg.Job,
	req apicontract.UpdateJobRequest,
) (automationpkg.Job, error) {
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
		next.Task = cloneBundleTaskConfig(req.Task)
	}
	if req.LoopTarget != nil {
		next.LoopTarget = cloneHostAPIAutomationLoopTarget(req.LoopTarget)
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

func validateHostAPIConfigJobUpdate(req apicontract.UpdateJobRequest) error {
	switch {
	case req.Enabled == nil:
		return errors.New("config-backed automation jobs only accept enabled updates")
	case hostAPIJobUpdateTouchesImmutableConfigFields(req):
		return errors.New("config-backed automation jobs only accept enabled updates")
	default:
		return nil
	}
}

func (h *HostAPIHandler) triggerFromCreateParams(
	ctx context.Context,
	req hostAPIAutomationTriggerCreateParams,
) (automationpkg.Trigger, automationpkg.WebhookSecretWrite, error) {
	workspaceID, err := h.resolveAutomationWorkspaceID(ctx, req.WorkspaceID)
	if err != nil {
		return automationpkg.Trigger{}, automationpkg.WebhookSecretWrite{}, err
	}

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

	trigger := automationpkg.Trigger{
		Scope:        req.Scope,
		Name:         strings.TrimSpace(req.Name),
		TargetKind:   req.TargetKind,
		AgentName:    strings.TrimSpace(req.AgentName),
		WorkspaceID:  workspaceID,
		Prompt:       strings.TrimSpace(req.Prompt),
		Event:        strings.TrimSpace(req.Event),
		Filter:       cloneHostAPIStringMap(req.Filter),
		LoopTarget:   cloneHostAPIAutomationLoopTarget(req.LoopTarget),
		Enabled:      enabled,
		Retry:        retry,
		FireLimit:    fireLimit,
		Source:       automationpkg.JobSourceDynamic,
		WebhookID:    strings.TrimSpace(req.WebhookID),
		EndpointSlug: strings.TrimSpace(req.EndpointSlug),
	}
	write := automationpkg.WebhookSecretWrite{}
	if strings.TrimSpace(req.WebhookSecretValue) != "" {
		value := strings.TrimSpace(req.WebhookSecretValue)
		write.Value = &value
	}
	return trigger, write, nil
}

func (h *HostAPIHandler) applyTriggerUpdateParams(
	_ context.Context,
	current automationpkg.Trigger,
	req apicontract.UpdateTriggerRequest,
) (automationpkg.Trigger, *automationpkg.WebhookSecretWrite, error) {
	next := current
	if req.Name != nil {
		next.Name = strings.TrimSpace(*req.Name)
	}
	if req.Prompt != nil {
		next.Prompt = strings.TrimSpace(*req.Prompt)
	}
	if req.Event != nil {
		next.Event = strings.TrimSpace(*req.Event)
	}
	if req.Filter != nil {
		next.Filter = cloneHostAPIStringMap(req.Filter)
	}
	if req.LoopTarget != nil {
		next.LoopTarget = cloneHostAPIAutomationLoopTarget(req.LoopTarget)
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
	if err := automationpkg.ValidateImmutableTriggerTarget(current, next); err != nil {
		return automationpkg.Trigger{}, nil, err
	}

	webhookSecret := applyTriggerWebhookUpdateParams(&next, req)
	return next, webhookSecret, nil
}

func applyTriggerWebhookUpdateParams(
	next *automationpkg.Trigger,
	req apicontract.UpdateTriggerRequest,
) *automationpkg.WebhookSecretWrite {
	event := strings.TrimSpace(next.Event)
	if req.WebhookID != nil {
		next.WebhookID = strings.TrimSpace(*req.WebhookID)
	} else if !strings.EqualFold(event, "webhook") {
		next.WebhookID = ""
	}
	if req.EndpointSlug != nil {
		next.EndpointSlug = strings.TrimSpace(*req.EndpointSlug)
	} else if !strings.EqualFold(event, "webhook") {
		next.EndpointSlug = ""
	}
	if !strings.EqualFold(event, "webhook") {
		next.WebhookSecretRef = ""
	}

	if req.WebhookSecretValue == nil {
		return nil
	}
	write := automationpkg.WebhookSecretWrite{}
	value := strings.TrimSpace(*req.WebhookSecretValue)
	write.Value = &value
	return &write
}

func validateHostAPIConfigTriggerUpdate(req apicontract.UpdateTriggerRequest) error {
	switch {
	case req.Enabled == nil:
		return errors.New("config-backed automation triggers only accept enabled updates")
	case hostAPITriggerUpdateTouchesImmutableConfigFields(req):
		return errors.New("config-backed automation triggers only accept enabled updates")
	default:
		return nil
	}
}
