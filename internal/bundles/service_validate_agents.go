package bundles

import (
	"context"
	"errors"
	"fmt"
	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/resources"
)

func (s *Service) validateActivationAgentBindings(
	ctx context.Context,
	activation Activation,
	materialized materializedActivationResources,
) error {
	available, err := s.visibleAgentNames(ctx, activation)
	if err != nil {
		return err
	}
	for _, agent := range materialized.agents {
		name := strings.TrimSpace(agent.Spec.Name)
		if name == "" {
			continue
		}
		if err := aghconfig.ValidateAuthoredAgentName(name); err != nil {
			return fmt.Errorf(
				"bundles: validate activation agent %q at %q: %w",
				name,
				agent.Spec.SourcePath,
				err,
			)
		}
		if _, exists := available[name]; exists {
			return fmt.Errorf("%w: %s", ErrAgentConflict, name)
		}
		available[name] = struct{}{}
	}
	if err := validateAutomationAgentReferences(
		activation,
		materialized.jobs,
		materialized.triggers,
		available,
	); err != nil {
		return err
	}
	return nil
}

func (s *Service) visibleAgentNames(ctx context.Context, activation Activation) (map[string]struct{}, error) {
	names := make(map[string]struct{})
	records, err := s.store.ListAgentResources(ctx)
	if err != nil {
		return nil, err
	}
	currentOwner := ownerForActivation(activation.ID)
	for _, record := range records {
		if !agentRecordVisibleToActivation(record, activation) {
			continue
		}
		if record.Owner.Normalize() == currentOwner.Normalize() {
			continue
		}
		name := strings.TrimSpace(record.Spec.Name)
		if name != "" {
			names[name] = struct{}{}
		}
	}
	if activation.Scope.Normalize() != ScopeWorkspace {
		return names, nil
	}
	if s.workspaceResolver == nil {
		return nil, errors.New("bundles: workspace resolver is required to validate workspace-scoped bundle agents")
	}
	resolved, err := s.workspaceResolver.Resolve(ctx, activation.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf(
			"bundles: resolve workspace %q for bundle agent validation: %w",
			activation.WorkspaceID,
			err,
		)
	}
	for _, agent := range resolved.Agents {
		name := strings.TrimSpace(agent.Name)
		if name != "" {
			names[name] = struct{}{}
		}
	}
	return names, nil
}

func agentRecordVisibleToActivation(record resources.Record[aghconfig.AgentDef], activation Activation) bool {
	scope := record.Scope.Normalize()
	switch activation.Scope.Normalize() {
	case ScopeWorkspace:
		return scope.Kind == resources.ResourceScopeKindGlobal ||
			(scope.Kind == resources.ResourceScopeKindWorkspace &&
				strings.TrimSpace(scope.ID) == strings.TrimSpace(activation.WorkspaceID))
	default:
		return scope.Kind == resources.ResourceScopeKindGlobal
	}
}

func validateAutomationAgentReferences(
	activation Activation,
	jobs []automationpkg.Job,
	triggers []automationpkg.Trigger,
	available map[string]struct{},
) error {
	for _, job := range jobs {
		agentName := strings.TrimSpace(job.AgentName)
		if agentName == "" {
			continue
		}
		if _, ok := available[agentName]; !ok {
			return fmt.Errorf(
				"%w: activation %q job %q references %q",
				ErrAgentReferenceNotFound,
				activation.ID,
				job.Name,
				agentName,
			)
		}
	}
	for _, trigger := range triggers {
		agentName := strings.TrimSpace(trigger.AgentName)
		if agentName == "" {
			continue
		}
		if _, ok := available[agentName]; !ok {
			return fmt.Errorf(
				"%w: activation %q trigger %q references %q",
				ErrAgentReferenceNotFound,
				activation.ID,
				trigger.Name,
				agentName,
			)
		}
	}
	return nil
}

func validateDesiredAgentScopeConflicts(agents []ownedAgentResource) error {
	seen := make(map[string]desiredAgentScopes)
	for _, agent := range agents {
		name := strings.TrimSpace(agent.Spec.Name)
		if name == "" {
			continue
		}
		scope := agent.Scope.Normalize()
		scopes := seen[name]
		if desiredAgentScopeConflicts(scopes, scope) {
			return fmt.Errorf("%w: %s", ErrAgentConflict, name)
		}
		seen[name] = rememberDesiredAgentScope(scopes, scope)
	}
	return nil
}

type desiredAgentScopes struct {
	count      int
	global     bool
	workspaces map[string]struct{}
}

func desiredAgentScopeConflicts(seen desiredAgentScopes, scope resources.ResourceScope) bool {
	switch scope.Kind {
	case resources.ResourceScopeKindGlobal:
		return seen.count > 0
	case resources.ResourceScopeKindWorkspace:
		if seen.global {
			return true
		}
		_, exists := seen.workspaces[strings.TrimSpace(scope.ID)]
		return exists
	default:
		return seen.global
	}
}

func rememberDesiredAgentScope(
	seen desiredAgentScopes,
	scope resources.ResourceScope,
) desiredAgentScopes {
	seen.count++
	switch scope.Kind {
	case resources.ResourceScopeKindGlobal:
		seen.global = true
	case resources.ResourceScopeKindWorkspace:
		if seen.workspaces == nil {
			seen.workspaces = make(map[string]struct{}, 1)
		}
		seen.workspaces[strings.TrimSpace(scope.ID)] = struct{}{}
	}
	return seen
}
