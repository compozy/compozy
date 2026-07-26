package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/resources"

	skillspkg "github.com/compozy/agh/internal/skills"
)

func newAgentSkillSourceSyncer(
	raw resources.RawStore,
	agentStore resources.Store[aghconfig.AgentDef],
	agentCodec resources.KindCodec[aghconfig.AgentDef],
	agentProjector resources.TypedProjector[aghconfig.AgentDef],
	skillStore resources.Store[skillspkg.SkillResourceSpec],
	skillCodec resources.KindCodec[skillspkg.SkillResourceSpec],
	skillProjector resources.TypedProjector[skillspkg.SkillResourceSpec],
	mcpStore resources.Store[aghconfig.MCPServer],
	mcpCodec resources.KindCodec[aghconfig.MCPServer],
	actor resources.MutationActor,
	logger *slog.Logger,
	trigger func(context.Context, resources.ResourceKind, resources.ReconcileReason) error,
	providers ...agentSkillDeclarationProvider,
) agentSkillPublisher {
	if raw == nil || agentStore == nil || agentCodec == nil || skillStore == nil || skillCodec == nil ||
		mcpStore == nil || mcpCodec == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &agentSkillSourceSyncer{
		raw:            raw,
		agentStore:     agentStore,
		agentCodec:     agentCodec,
		agentProjector: agentProjector,
		skillStore:     skillStore,
		skillCodec:     skillCodec,
		skillProjector: skillProjector,
		mcpStore:       mcpStore,
		mcpCodec:       mcpCodec,
		actor:          actor,
		logger:         logger,
		trigger:        trigger,
		providers:      append([]agentSkillDeclarationProvider(nil), providers...),
	}
}

func agentSkillSyncActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "agent-skill-sync",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   "agent-skill-sync",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}

func (s *agentSkillSourceSyncer) Sync(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: agent/skill sync context is required")
	}
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	desired, err := s.desiredResources(ctx)
	if err != nil {
		return err
	}
	s.stageAgentTransientSpecs(desired.agents)
	agentChanged, err := s.syncAgents(ctx, desired.agents)
	if err != nil {
		return err
	}
	if err := s.projectAgents(ctx); err != nil {
		return err
	}
	skillChanged, err := s.syncSkills(ctx, desired.skills)
	if err != nil {
		return err
	}
	if skillChanged {
		if err := s.projectSkills(ctx); err != nil {
			return err
		}
	}
	mcpChanged, err := s.syncMCPServers(ctx, desired.mcpServers)
	if err != nil {
		return err
	}

	if agentChanged && s.trigger != nil {
		if err := s.trigger(ctx, aghconfig.AgentResourceKind, resources.ReconcileReasonWrite); err != nil {
			return err
		}
	}
	if skillChanged && s.trigger != nil {
		if err := s.trigger(ctx, skillspkg.SkillResourceKind, resources.ReconcileReasonWrite); err != nil {
			return err
		}
	}
	if mcpChanged && s.trigger != nil {
		if err := s.trigger(ctx, aghconfig.MCPServerResourceKind, resources.ReconcileReasonWrite); err != nil {
			return err
		}
	}
	return nil
}

func (s *agentSkillSourceSyncer) projectAgents(ctx context.Context) error {
	if s == nil || s.agentProjector == nil {
		return nil
	}
	records, err := s.agentStore.List(ctx, resourceReconcileActor(), resources.ResourceFilter{})
	if err != nil {
		return fmt.Errorf("daemon: list agent resources for projection: %w", err)
	}
	plan, err := s.agentProjector.Build(ctx, records)
	if err != nil {
		return fmt.Errorf("daemon: build agent resource projection: %w", err)
	}
	if err := s.agentProjector.Apply(ctx, plan); err != nil {
		return fmt.Errorf("daemon: apply agent resource projection: %w", err)
	}
	return nil
}

func (s *agentSkillSourceSyncer) SyncSkills(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: skill sync context is required")
	}
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	desired, err := s.desiredResources(ctx)
	if err != nil {
		return err
	}
	skillChanged, err := s.syncSkills(ctx, desired.skills)
	if err != nil {
		return err
	}
	if err := s.projectSkills(ctx); err != nil {
		return err
	}
	if skillChanged && s.trigger != nil {
		if err := s.trigger(ctx, skillspkg.SkillResourceKind, resources.ReconcileReasonWrite); err != nil {
			return err
		}
	}
	return nil
}

func (s *agentSkillSourceSyncer) projectSkills(ctx context.Context) error {
	if s == nil || s.skillProjector == nil {
		return nil
	}
	records, err := s.skillStore.List(ctx, resourceReconcileActor(), resources.ResourceFilter{})
	if err != nil {
		return fmt.Errorf("daemon: list skill resources for projection: %w", err)
	}
	plan, err := s.skillProjector.Build(ctx, records)
	if err != nil {
		return fmt.Errorf("daemon: build skill resource projection: %w", err)
	}
	if err := s.skillProjector.Apply(ctx, plan); err != nil {
		return fmt.Errorf("daemon: apply skill resource projection: %w", err)
	}
	return nil
}
