package bundles

import (
	"context"
	"errors"
	"fmt"

	"path/filepath"

	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
)

func materializeJob(
	activation Activation,
	bundle extensionpkg.BundleSpec,
	profile extensionpkg.BundleProfile,
	def extensionpkg.BundleJob,
) (automationpkg.Job, error) {
	job := automationpkg.Job{
		ID:          stableID("job", activation.ID, def.Name),
		Scope:       automationScopeFromActivation(activation.Scope),
		Name:        managedAutomationName(activation, bundle, profile, def.Name),
		AgentName:   strings.TrimSpace(def.AgentName),
		WorkspaceID: activation.WorkspaceID,
		Prompt:      strings.TrimSpace(def.Prompt),
		Schedule:    cloneSchedule(def.Schedule),
		Task:        cloneTaskConfig(def.Task),
		Enabled:     def.Enabled,
		Retry:       def.Retry,
		FireLimit:   def.FireLimit,
		Source:      automationpkg.JobSourcePackage,
	}
	if err := job.Validate("bundle.activation.job"); err != nil {
		return automationpkg.Job{}, err
	}
	return job, nil
}

func materializeAgent(
	activation Activation,
	bundle extensionpkg.BundleSpec,
	profile extensionpkg.BundleProfile,
	def extensionpkg.BundleAgent,
) (aghconfig.AgentDef, string, error) {
	agent := aghconfig.CloneAgentDef(def.Agent)
	agent.Name = strings.TrimSpace(agent.Name)
	agent.SourcePath = bundleAgentSyntheticSourcePath(activation.ID, agent.Name, "AGENT.md")
	if err := agent.Validate(); err != nil {
		return aghconfig.AgentDef{}, "", fmt.Errorf(
			"bundles: materialize agent %s/%s/%s/%s: %w",
			activation.ExtensionName,
			bundle.Name,
			profile.Name,
			agent.Name,
			err,
		)
	}
	return agent, stableID("agt", activation.ID, agent.Name), nil
}

func bundleAgentSyntheticSourcePath(activationID string, agentName string, filename string) string {
	return filepath.ToSlash(filepath.Join(
		".agh",
		"bundles",
		strings.TrimSpace(activationID),
		"agents",
		strings.TrimSpace(agentName),
		strings.TrimSpace(filename),
	))
}

func materializeTrigger(
	activation Activation,
	bundle extensionpkg.BundleSpec,
	profile extensionpkg.BundleProfile,
	def extensionpkg.BundleTrigger,
) (automationpkg.Trigger, error) {
	if strings.EqualFold(strings.TrimSpace(def.Event), "webhook") {
		return automationpkg.Trigger{}, fmt.Errorf(
			"%w: %s/%s/%s/%s",
			ErrWebhookUnsupported,
			activation.ExtensionName,
			bundle.Name,
			profile.Name,
			def.Name,
		)
	}

	trigger := automationpkg.Trigger{
		ID:           stableID("trg", activation.ID, def.Name),
		Scope:        automationScopeFromActivation(activation.Scope),
		Name:         managedAutomationName(activation, bundle, profile, def.Name),
		AgentName:    strings.TrimSpace(def.AgentName),
		WorkspaceID:  activation.WorkspaceID,
		Prompt:       strings.TrimSpace(def.Prompt),
		Event:        strings.TrimSpace(def.Event),
		Filter:       cloneStringMap(def.Filter),
		Enabled:      def.Enabled,
		Retry:        def.Retry,
		FireLimit:    def.FireLimit,
		Source:       automationpkg.JobSourcePackage,
		EndpointSlug: strings.TrimSpace(def.EndpointSlug),
	}
	if err := trigger.Validate("bundle.activation.trigger"); err != nil {
		return automationpkg.Trigger{}, err
	}
	return trigger, nil
}

func managedAutomationName(
	activation Activation,
	bundle extensionpkg.BundleSpec,
	profile extensionpkg.BundleProfile,
	name string,
) string {
	parts := []string{
		strings.TrimSpace(activation.ExtensionName),
		strings.TrimSpace(bundle.Name),
		strings.TrimSpace(profile.Name),
		strings.TrimSpace(name),
	}
	return strings.Join(parts, "/")
}

func automationScopeFromActivation(scope Scope) automationpkg.Scope {
	if scope.Normalize() == ScopeWorkspace {
		return automationpkg.AutomationScopeWorkspace
	}
	return automationpkg.AutomationScopeGlobal
}

func bridgeScopeFromActivation(scope Scope) bridgepkg.Scope {
	if scope.Normalize() == ScopeWorkspace {
		return bridgepkg.ScopeWorkspace
	}
	return bridgepkg.ScopeGlobal
}

func (s *Service) joinRollbackFailure(
	ctx context.Context,
	reconcileErr error,
	rollbackErr error,
	action string,
	activationID string,
) error {
	if rollbackErr == nil {
		return reconcileErr
	}
	s.logger.ErrorContext(
		ctx,
		"bundles.activation.rollback_failed",
		"activation_id", strings.TrimSpace(activationID),
		"action", strings.TrimSpace(action),
		"error", rollbackErr,
	)
	return errors.Join(
		reconcileErr,
		fmt.Errorf(
			"bundles: %s for activation %q: %w",
			strings.TrimSpace(action),
			strings.TrimSpace(activationID),
			rollbackErr,
		),
	)
}

func isPathLikeWorkspaceRef(ref string) bool {
	trimmed := strings.TrimSpace(ref)
	return filepath.IsAbs(trimmed) ||
		strings.HasPrefix(trimmed, ".") ||
		strings.HasPrefix(trimmed, "~") ||
		strings.ContainsAny(trimmed, `/\`)
}
