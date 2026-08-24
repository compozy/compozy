package automation

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
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
		return resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
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
	if err := bindAutomationScope(
		&next.Scope,
		&next.WorkspaceID,
		&next.ProfileID,
		normalizedScope,
		"job",
	); err != nil {
		return Job{}, fmt.Errorf("automation: bind job resource scope: %w", err)
	}
	if err := next.Validate("job"); err != nil {
		return Job{}, fmt.Errorf("automation: validate job resource spec: %w", err)
	}
	if err := ValidateJobAgentName(next, "job"); err != nil {
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
	if err := bindAutomationScope(
		&next.Scope,
		&next.WorkspaceID,
		&next.ProfileID,
		normalizedScope,
		"trigger",
	); err != nil {
		return Trigger{}, fmt.Errorf("automation: bind trigger resource scope: %w", err)
	}
	if err := next.Validate("trigger"); err != nil {
		return Trigger{}, fmt.Errorf("automation: validate trigger resource spec: %w", err)
	}
	if err := ValidateTriggerAgentName(next, "trigger"); err != nil {
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
	next.ProfileID = strings.TrimSpace(next.ProfileID)
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
	next.ProfileID = strings.TrimSpace(next.ProfileID)
	next.AgentName = strings.TrimSpace(next.AgentName)
	next.WorkspaceID = strings.TrimSpace(next.WorkspaceID)
	next.Prompt = strings.TrimSpace(next.Prompt)
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
	profileID *string,
	resourceScope resources.ResourceScope,
	path string,
) error {
	switch resourceScope.Kind {
	case resources.ResourceScopeKindUser:
		if err := bindGlobalAutomationScope(domainScope, workspaceID, resourceScope, path); err != nil {
			return err
		}
		bindDefaultAutomationProfileID(profileID)
	case resources.ResourceScopeKindWorkspace:
		if err := bindWorkspaceAutomationScope(
			domainScope,
			workspaceID,
			resourceScope.ID,
			resourceScope,
			path,
		); err != nil {
			return err
		}
		bindDefaultAutomationProfileID(profileID)
	case resources.ResourceScopeKindProfile:
		if err := bindGlobalAutomationScope(domainScope, workspaceID, resourceScope, path); err != nil {
			return err
		}
		return bindAutomationProfileID(profileID, resourceScope.ID, path)
	case resources.ResourceScopeKindWorkspaceProfile:
		workspaceScopeID, _, ok := strings.Cut(resourceScope.ID, "@pf:")
		if !ok {
			return fmt.Errorf(
				"%w: %s resource scope %q does not contain a profile binding",
				resources.ErrInvalidScopeBinding,
				path,
				resourceScope.ID,
			)
		}
		if err := bindWorkspaceAutomationScope(
			domainScope,
			workspaceID,
			workspaceScopeID,
			resourceScope,
			path,
		); err != nil {
			return err
		}
		if strings.TrimSpace(*profileID) == "" {
			return fmt.Errorf(
				"%w: %s.profile_id is required for resource scope %q",
				resources.ErrInvalidScopeBinding,
				path,
				resourceScope.Kind,
			)
		}
		*profileID = strings.TrimSpace(*profileID)
	default:
		return fmt.Errorf(
			"%w: unsupported %s resource scope %q",
			resources.ErrInvalidScopeBinding,
			path,
			resourceScope.Kind,
		)
	}
	return nil
}

func bindGlobalAutomationScope(
	domainScope *Scope,
	workspaceID *string,
	resourceScope resources.ResourceScope,
	path string,
) error {
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
	return nil
}

func bindWorkspaceAutomationScope(
	domainScope *Scope,
	workspaceID *string,
	wantWorkspaceID string,
	resourceScope resources.ResourceScope,
	path string,
) error {
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
	wantWorkspaceID = strings.TrimSpace(wantWorkspaceID)
	if trimmedWorkspaceID != "" && trimmedWorkspaceID != wantWorkspaceID {
		return fmt.Errorf(
			"%w: %s.workspace_id %q does not match resource scope %q",
			resources.ErrInvalidScopeBinding,
			path,
			trimmedWorkspaceID,
			resourceScope.ID,
		)
	}
	*workspaceID = wantWorkspaceID
	return nil
}

func bindAutomationProfileID(profileID *string, wantProfileID string, path string) error {
	trimmedProfileID := strings.TrimSpace(*profileID)
	wantProfileID = strings.TrimSpace(wantProfileID)
	if trimmedProfileID != "" && trimmedProfileID != wantProfileID {
		return fmt.Errorf(
			"%w: %s.profile_id %q does not match resource profile %q",
			resources.ErrInvalidScopeBinding,
			path,
			trimmedProfileID,
			wantProfileID,
		)
	}
	*profileID = wantProfileID
	return nil
}

func bindDefaultAutomationProfileID(profileID *string) {
	*profileID = strings.TrimSpace(*profileID)
	if *profileID == "" {
		*profileID = store.DefaultProfileID
	}
}
