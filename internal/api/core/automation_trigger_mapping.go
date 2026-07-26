package core

import (
	"errors"

	"maps"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
)

// AutomationTriggerFromCreateRequest converts the shared create payload into
// the canonical automation trigger model.
func AutomationTriggerFromCreateRequest(req contract.CreateTriggerRequest) automationpkg.Trigger {
	return triggerFromCreateRequest(req)
}

func triggerFromCreateRequest(req contract.CreateTriggerRequest) automationpkg.Trigger {
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

	return automationpkg.Trigger{
		Scope:        req.Scope,
		Name:         strings.TrimSpace(req.Name),
		TargetKind:   req.TargetKind,
		AgentName:    strings.TrimSpace(req.AgentName),
		WorkspaceID:  strings.TrimSpace(req.WorkspaceID),
		Prompt:       strings.TrimSpace(req.Prompt),
		Event:        strings.TrimSpace(req.Event),
		Filter:       cloneAutomationFilter(req.Filter),
		LoopTarget:   cloneAutomationLoopTarget(req.LoopTarget),
		Enabled:      enabled,
		Retry:        retry,
		FireLimit:    fireLimit,
		Source:       automationpkg.JobSourceDynamic,
		WebhookID:    strings.TrimSpace(req.WebhookID),
		EndpointSlug: strings.TrimSpace(req.EndpointSlug),
	}
}

// ApplyAutomationTriggerPatch applies the shared patch payload to an automation trigger model.
func ApplyAutomationTriggerPatch(
	current automationpkg.Trigger,
	req contract.UpdateTriggerRequest,
) (automationpkg.Trigger, error) {
	return applyTriggerPatch(current, req)
}

func applyTriggerPatch(
	current automationpkg.Trigger,
	req contract.UpdateTriggerRequest,
) (automationpkg.Trigger, error) {
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
		next.Filter = cloneAutomationFilter(req.Filter)
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
	if err := automationpkg.ValidateImmutableTriggerTarget(current, next); err != nil {
		return automationpkg.Trigger{}, err
	}

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

	return next, nil
}

func webhookSecretWriteFromCreateRequest(req contract.CreateTriggerRequest) automationpkg.WebhookSecretWrite {
	write := automationpkg.WebhookSecretWrite{}
	if req.WebhookSecretValue != "" {
		value := req.WebhookSecretValue
		write.Value = &value
	}
	return write
}

func webhookSecretWriteFromUpdateRequest(req contract.UpdateTriggerRequest) *automationpkg.WebhookSecretWrite {
	if req.WebhookSecretValue == nil {
		return nil
	}
	write := automationpkg.WebhookSecretWrite{}
	value := *req.WebhookSecretValue
	write.Value = &value
	return &write
}

// ValidateAutomationManagedTriggerUpdate enforces the managed trigger mutation policy.
func ValidateAutomationManagedTriggerUpdate(req contract.UpdateTriggerRequest) error {
	return validateManagedTriggerUpdate(req)
}

func validateManagedTriggerUpdate(req contract.UpdateTriggerRequest) error {
	switch {
	case req.Enabled == nil:
		return errors.New("managed automation triggers only accept enabled updates")
	case req.Name != nil ||
		req.Prompt != nil ||
		req.Event != nil ||
		req.Filter != nil ||
		req.LoopTarget != nil ||
		req.Retry != nil ||
		req.FireLimit != nil ||
		req.WebhookID != nil ||
		req.EndpointSlug != nil ||
		req.WebhookSecretValue != nil:
		return errors.New("managed automation triggers only accept enabled updates")
	default:
		return nil
	}
}

func cloneAutomationFilter(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	maps.Copy(cloned, source)
	return cloned
}
