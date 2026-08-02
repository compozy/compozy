package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"

	"github.com/compozy/compozy/internal/resources"

	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"
)

func newAgentSkillSourceSyncer(
	raw resources.RawStore,
	agentStore resources.Store[compozyconfig.AgentDef],
	agentCodec resources.KindCodec[compozyconfig.AgentDef],
	agentProjector resources.TypedProjector[compozyconfig.AgentDef],
	skillStore resources.Store[skillspkg.SkillResourceSpec],
	skillCodec resources.KindCodec[skillspkg.SkillResourceSpec],
	skillProjector resources.TypedProjector[skillspkg.SkillResourceSpec],
	mcpStore resources.Store[compozyconfig.MCPServer],
	mcpCodec resources.KindCodec[compozyconfig.MCPServer],
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
	changes, err := s.syncDesiredAgentSkillResources(ctx, desired)
	if err != nil {
		return err
	}
	return s.triggerAgentSkillChanges(ctx, changes)
}

type agentSkillSyncChanges struct {
	agents     bool
	souls      bool
	heartbeats bool
	skills     bool
	mcpServers bool
}

func (s *agentSkillSourceSyncer) syncDesiredAgentSkillResources(
	ctx context.Context,
	desired desiredAgentSkillResources,
) (agentSkillSyncChanges, error) {
	var changes agentSkillSyncChanges
	s.stageAgentTransientSpecs(desired.agents)
	var err error
	changes.agents, err = s.syncAgents(ctx, desired.agents)
	if err != nil {
		return agentSkillSyncChanges{}, err
	}
	if err := s.projectAgents(ctx); err != nil {
		return agentSkillSyncChanges{}, err
	}
	changes.souls, err = s.syncSouls(ctx, desired.souls)
	if err != nil {
		return agentSkillSyncChanges{}, err
	}
	changes.heartbeats, err = s.syncHeartbeats(ctx, desired.heartbeats)
	if err != nil {
		return agentSkillSyncChanges{}, err
	}
	changes.skills, err = s.syncSkills(ctx, desired.skills)
	if err != nil {
		return agentSkillSyncChanges{}, err
	}
	if changes.skills {
		if err := s.projectSkills(ctx); err != nil {
			return agentSkillSyncChanges{}, err
		}
	}
	changes.mcpServers, err = s.syncMCPServers(ctx, desired.mcpServers)
	if err != nil {
		return agentSkillSyncChanges{}, err
	}
	return changes, nil
}

func (s *agentSkillSourceSyncer) triggerAgentSkillChanges(
	ctx context.Context,
	changes agentSkillSyncChanges,
) error {
	if s.trigger == nil {
		return nil
	}
	for _, change := range []struct {
		changed bool
		kind    resources.ResourceKind
	}{
		{changed: changes.agents, kind: compozyconfig.AgentResourceKind},
		{changed: changes.souls, kind: soul.ResourceKind},
		{changed: changes.heartbeats, kind: heartbeat.ResourceKind},
		{changed: changes.skills, kind: skillspkg.SkillResourceKind},
		{changed: changes.mcpServers, kind: compozyconfig.MCPServerResourceKind},
	} {
		if change.changed {
			if err := s.trigger(ctx, change.kind, resources.ReconcileReasonWrite); err != nil {
				return err
			}
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
