package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	settingspkg "github.com/compozy/compozy/internal/settings"
	skillspkg "github.com/compozy/compozy/internal/skills"
)

const skillSourceRollbackTimeout = 5 * time.Second

type skillResourceRepublisher interface {
	SyncSkills(context.Context) error
}

type stagedSkillResourceRepublisher interface {
	SyncSkillsStaged(context.Context) (func(context.Context) error, error)
}

func (a daemonSettingsRuntimeApplier) applySkillSourceConfigChange(
	ctx context.Context,
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
) []settingspkg.ApplyFailure {
	if a.state.skillsRegistry == nil || !skillSourceApplyRequired(ctx, previous, next) {
		return nil
	}
	if a.state.workspaceResolver != nil {
		a.state.workspaceResolver.InvalidateAll()
	}
	generation := skillspkg.ConfigGenerationFromContext(ctx)
	if generation <= 0 {
		generation = a.state.skillsRegistry.ConfigGeneration() + 1
		ctx = skillspkg.WithConfigGeneration(ctx, generation)
	}
	needsPublication, err := a.state.skillsRegistry.ApplyConfigGeneration(
		ctx,
		generation,
		a.daemon.skillsRegistryConfig(next),
	)
	if err != nil {
		return skillSourceApplyFailures(a.state.skillsRegistry.SourceApplyFailureError(ctx, generation, err))
	}
	if !needsPublication {
		return nil
	}
	rollback, err := syncSkillResourcesStaged(ctx, a.state.agentSkillResources)
	if err != nil {
		return a.abortAndRestoreSkillSources(
			ctx,
			generation,
			a.state.skillsRegistry.SourceApplyFailureError(ctx, generation, err),
			rollback,
		)
	}
	if err := a.state.skillsRegistry.CommitConfigGeneration(ctx, generation); err != nil {
		return a.abortAndRestoreSkillSources(
			ctx,
			generation,
			a.state.skillsRegistry.SourceApplyFailureError(ctx, generation, err),
			rollback,
		)
	}
	return nil
}

func skillSourceApplyRequired(
	ctx context.Context,
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
) bool {
	if skillRegistryConfigChanged(previous, next) {
		return true
	}
	scope := skillspkg.SourceEventCorrelationFromContext(ctx).Scope
	return scope == string(settingspkg.ScopeProfile) || scope == string(settingspkg.ScopeWorkspace)
}

func (a daemonSettingsRuntimeApplier) abortAndRestoreSkillSources(
	ctx context.Context,
	generation int64,
	cause error,
	rollback func(context.Context) error,
) []settingspkg.ApplyFailure {
	a.state.skillsRegistry.AbortConfigGeneration(generation)
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), skillSourceRollbackTimeout)
	defer cancel()
	if rollback != nil {
		if cleanupErr := rollback(cleanupCtx); cleanupErr != nil {
			cause = errors.Join(cause, fmt.Errorf("restore previous skill resources: %w", cleanupErr))
		}
	}
	return skillSourceApplyFailures(cause)
}

func syncSkillResourcesStaged(
	ctx context.Context,
	publisher agentSkillPublisher,
) (func(context.Context) error, error) {
	if publisher == nil {
		return nil, errors.New("daemon: skill resource publisher is unavailable")
	}
	if staged, ok := publisher.(stagedSkillResourceRepublisher); ok {
		return staged.SyncSkillsStaged(ctx)
	}
	if err := syncSkillResources(ctx, publisher); err != nil {
		return nil, err
	}
	return nil, nil
}

func syncSkillResources(ctx context.Context, publisher agentSkillPublisher) error {
	if publisher == nil {
		return errors.New("daemon: skill resource publisher is unavailable")
	}
	if scoped, ok := publisher.(skillResourceRepublisher); ok {
		return scoped.SyncSkills(ctx)
	}
	return publisher.Sync(ctx)
}

func skillRegistryConfigChanged(previous *compozyconfig.Config, next *compozyconfig.Config) bool {
	if previous == nil || next == nil {
		return previous != next
	}
	return !slices.Equal(previous.Skills.Sources, next.Skills.Sources) ||
		!slices.Equal(previous.Skills.CustomSources, next.Skills.CustomSources) ||
		!slices.Equal(previous.Skills.DisabledSkills, next.Skills.DisabledSkills)
}

func skillSourceApplyFailures(err error) []settingspkg.ApplyFailure {
	return []settingspkg.ApplyFailure{configApplyFailure(
		"skill_sources",
		diagnosticcontract.CategoryConfig,
		"Skill source sync failed",
		err,
	)}
}
