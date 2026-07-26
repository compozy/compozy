package automation

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/resources"
)

const (
	// JobResourceKind is the canonical desired-state kind for scheduled automation jobs.
	JobResourceKind resources.ResourceKind = "automation.job"
	// TriggerResourceKind is the canonical desired-state kind for event-driven automation triggers.
	TriggerResourceKind resources.ResourceKind = "automation.trigger"

	automationResourceMaxBytes = 256 << 10
)

// NewJobResourceCodec builds the typed codec for automation.job records.
func NewJobResourceCodec() (resources.KindCodec[Job], error) {
	return resources.NewJSONCodec(JobResourceKind, automationResourceMaxBytes, validateJobResourceSpec)
}

// NewTriggerResourceCodec builds the typed codec for automation.trigger records.
func NewTriggerResourceCodec() (resources.KindCodec[Trigger], error) {
	return resources.NewJSONCodec(TriggerResourceKind, automationResourceMaxBytes, validateTriggerResourceSpec)
}

// ResourceScopeForAutomation converts automation scope fields into the shared resource scope.
func ResourceScopeForAutomation(scope Scope, workspaceID string) resources.ResourceScope {
	switch scope {
	case AutomationScopeWorkspace:
		return resources.ResourceScope{
			Kind: resources.ResourceScopeKindWorkspace,
			ID:   strings.TrimSpace(workspaceID),
		}
	default:
		return resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
	}
}

func validateJobResourceSpec(_ context.Context, scope resources.ResourceScope, spec Job) (Job, error) {
	normalizedScope := scope.Normalize()
	if err := normalizedScope.Validate("scope"); err != nil {
		return Job{}, fmt.Errorf("automation: validate job resource scope: %w", err)
	}

	next := normalizeJobResourceSpec(spec)
	if next.Task != nil {
		request, err := NormalizeDirectTaskParticipation(next.Task.NetworkParticipation)
		if err != nil {
			return Job{}, fmt.Errorf("automation: normalize job task participation: %w", err)
		}
		next.Task.NetworkParticipation = request
	}
	if err := normalizeLoopTargetParticipation(next.LoopTarget); err != nil {
		return Job{}, fmt.Errorf("automation: normalize job loop participation: %w", err)
	}
	if err := bindAutomationScope(&next.Scope, &next.WorkspaceID, normalizedScope, "job"); err != nil {
		return Job{}, fmt.Errorf("automation: bind job resource scope: %w", err)
	}
	if err := next.Validate("job"); err != nil {
		return Job{}, fmt.Errorf("automation: validate job resource spec: %w", err)
	}
	return next, nil
}

func validateTriggerResourceSpec(_ context.Context, scope resources.ResourceScope, spec Trigger) (Trigger, error) {
	normalizedScope := scope.Normalize()
	if err := normalizedScope.Validate("scope"); err != nil {
		return Trigger{}, fmt.Errorf("automation: validate trigger resource scope: %w", err)
	}

	next := normalizeTriggerResourceSpec(spec)
	if err := normalizeLoopTargetParticipation(next.LoopTarget); err != nil {
		return Trigger{}, fmt.Errorf("automation: normalize trigger loop participation: %w", err)
	}
	if err := bindAutomationScope(&next.Scope, &next.WorkspaceID, normalizedScope, "trigger"); err != nil {
		return Trigger{}, fmt.Errorf("automation: bind trigger resource scope: %w", err)
	}
	if err := next.Validate("trigger"); err != nil {
		return Trigger{}, fmt.Errorf("automation: validate trigger resource spec: %w", err)
	}
	return next, nil
}

func normalizeLoopTargetParticipation(target *LoopTarget) error {
	if target == nil || target.NetworkParticipation == nil {
		return nil
	}
	normalized, err := participation.NormalizeIntent(*target.NetworkParticipation)
	if err != nil {
		return err
	}
	if normalized == (participation.Request{}) {
		target.NetworkParticipation = nil
		return nil
	}
	target.NetworkParticipation = &normalized
	return nil
}

func normalizeJobResourceSpec(spec Job) Job {
	next := cloneJob(spec)
	next.ID = strings.TrimSpace(next.ID)
	next.Name = strings.TrimSpace(next.Name)
	next.AgentName = strings.TrimSpace(next.AgentName)
	next.WorkspaceID = strings.TrimSpace(next.WorkspaceID)
	next.Prompt = strings.TrimSpace(next.Prompt)
	if next.Source == "" {
		next.Source = JobSourceDynamic
	}
	if next.Retry.Strategy == "" {
		next.Retry = DefaultRetryConfig()
	}
	if next.FireLimit.Max == 0 || strings.TrimSpace(next.FireLimit.Window) == "" {
		next.FireLimit = DefaultFireLimitConfig()
	}
	next.CreatedAt = next.CreatedAt.UTC()
	next.UpdatedAt = next.UpdatedAt.UTC()
	return next
}

func normalizeTriggerResourceSpec(spec Trigger) Trigger {
	next := cloneTrigger(spec)
	next.ID = strings.TrimSpace(next.ID)
	next.Name = strings.TrimSpace(next.Name)
	next.AgentName = strings.TrimSpace(next.AgentName)
	next.WorkspaceID = strings.TrimSpace(next.WorkspaceID)
	next.Prompt = strings.TrimSpace(next.Prompt)
	next.Event = strings.TrimSpace(next.Event)
	next.WebhookID = strings.TrimSpace(next.WebhookID)
	next.EndpointSlug = strings.TrimSpace(next.EndpointSlug)
	next.WebhookSecretRef = strings.TrimSpace(next.WebhookSecretRef)
	if next.Source == "" {
		next.Source = JobSourceDynamic
	}
	if next.Retry.Strategy == "" {
		next.Retry = DefaultRetryConfig()
	}
	if next.FireLimit.Max == 0 || strings.TrimSpace(next.FireLimit.Window) == "" {
		next.FireLimit = DefaultFireLimitConfig()
	}
	next.CreatedAt = next.CreatedAt.UTC()
	next.UpdatedAt = next.UpdatedAt.UTC()
	return next
}

func bindAutomationScope(
	domainScope *Scope,
	workspaceID *string,
	resourceScope resources.ResourceScope,
	path string,
) error {
	switch resourceScope.Kind {
	case resources.ResourceScopeKindGlobal:
		if *domainScope == "" {
			*domainScope = AutomationScopeGlobal
		}
		if *domainScope != AutomationScopeGlobal {
			return fmt.Errorf(
				"%w: %s.scope %q does not match resource scope %q",
				resources.ErrInvalidScopeBinding,
				path,
				*domainScope,
				resourceScope.Kind,
			)
		}
		if strings.TrimSpace(*workspaceID) != "" {
			return fmt.Errorf(
				"%w: %s.workspace_id must be empty for global resource scope",
				resources.ErrInvalidScopeBinding,
				path,
			)
		}
		*workspaceID = ""
	case resources.ResourceScopeKindWorkspace:
		if *domainScope == "" {
			*domainScope = AutomationScopeWorkspace
		}
		if *domainScope != AutomationScopeWorkspace {
			return fmt.Errorf(
				"%w: %s.scope %q does not match resource scope %q",
				resources.ErrInvalidScopeBinding,
				path,
				*domainScope,
				resourceScope.Kind,
			)
		}
		trimmedWorkspaceID := strings.TrimSpace(*workspaceID)
		switch {
		case trimmedWorkspaceID == "":
			*workspaceID = resourceScope.ID
		case trimmedWorkspaceID != resourceScope.ID:
			return fmt.Errorf(
				"%w: %s.workspace_id %q does not match resource scope %q",
				resources.ErrInvalidScopeBinding,
				path,
				trimmedWorkspaceID,
				resourceScope.ID,
			)
		default:
			*workspaceID = trimmedWorkspaceID
		}
	}
	return nil
}
