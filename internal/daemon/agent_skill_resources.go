package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"sync"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"

	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/soul"
)

const (
	agentManagedIDPrefix = "daemon.sync.agent."
	skillManagedIDPrefix = "daemon.sync.skill."
)

type agentSkillPublisher interface {
	Sync(context.Context) error
	SyncSkills(context.Context) error
}

type agentSkillPublisherFunc func(context.Context) error

func (f agentSkillPublisherFunc) Sync(ctx context.Context) error {
	if f == nil {
		return nil
	}
	return f(ctx)
}

func (f agentSkillPublisherFunc) SyncSkills(ctx context.Context) error {
	return f.Sync(ctx)
}

type agentSkillDeclarationProvider func(context.Context) (agentSkillDesiredResources, error)

type agentPublicationInput struct {
	sourceKey string
	scope     resources.ResourceScope
	spec      aghconfig.AgentDef
}

type skillPublicationInput struct {
	sourceKey string
	scope     resources.ResourceScope
	spec      skillspkg.SkillResourceSpec
}

type agentSkillDesiredResources struct {
	agents     []agentPublicationInput
	skills     []skillPublicationInput
	mcpServers []mcpServerPublicationInput
}

type agentSkillSourceSyncer struct {
	syncMu         sync.Mutex
	raw            resources.RawStore
	agentStore     resources.Store[aghconfig.AgentDef]
	agentCodec     resources.KindCodec[aghconfig.AgentDef]
	agentProjector resources.TypedProjector[aghconfig.AgentDef]
	skillStore     resources.Store[skillspkg.SkillResourceSpec]
	skillCodec     resources.KindCodec[skillspkg.SkillResourceSpec]
	skillProjector resources.TypedProjector[skillspkg.SkillResourceSpec]
	mcpStore       resources.Store[aghconfig.MCPServer]
	mcpCodec       resources.KindCodec[aghconfig.MCPServer]
	actor          resources.MutationActor
	logger         *slog.Logger
	trigger        func(context.Context, resources.ResourceKind, resources.ReconcileReason) error
	providers      []agentSkillDeclarationProvider
}

type skillResourceProjectionPlan struct {
	revision int64
	records  []resources.Record[skillspkg.SkillResourceSpec]
}

func (p *skillResourceProjectionPlan) Kind() resources.ResourceKind {
	if p == nil {
		return ""
	}
	return skillspkg.SkillResourceKind
}

func (p *skillResourceProjectionPlan) Revision() int64 {
	if p == nil {
		return 0
	}
	return p.revision
}

func (p *skillResourceProjectionPlan) OperationCount() int {
	if p == nil {
		return 0
	}
	return len(p.records)
}

type skillResourceProjector struct {
	registry *skillspkg.Registry
}

func newAgentProjector(catalog *resourceCatalog[aghconfig.AgentDef]) resources.TypedProjector[aghconfig.AgentDef] {
	if catalog == nil {
		return nil
	}
	return &resourceCatalogProjector[aghconfig.AgentDef]{
		kind:      aghconfig.AgentResourceKind,
		catalog:   catalog,
		cloneSpec: cloneAgentDef,
	}
}

func newSoulProjector(catalog *resourceCatalog[soul.ResourceSpec]) resources.TypedProjector[soul.ResourceSpec] {
	if catalog == nil {
		return nil
	}
	return &resourceCatalogProjector[soul.ResourceSpec]{
		kind:      soul.ResourceKind,
		catalog:   catalog,
		cloneSpec: cloneSoulResourceSpec,
	}
}

func newHeartbeatProjector(
	catalog *resourceCatalog[heartbeat.ResourceSpec],
) resources.TypedProjector[heartbeat.ResourceSpec] {
	if catalog == nil {
		return nil
	}
	return &resourceCatalogProjector[heartbeat.ResourceSpec]{
		kind:      heartbeat.ResourceKind,
		catalog:   catalog,
		cloneSpec: cloneHeartbeatResourceSpec,
	}
}

func newSkillProjector(registry *skillspkg.Registry) resources.TypedProjector[skillspkg.SkillResourceSpec] {
	if registry == nil {
		return nil
	}
	return &skillResourceProjector{registry: registry}
}

func appendSkillProjectorRegistration(
	registrations []resources.ProjectorRegistration,
	deps *resourceReconcileDriverDeps,
) ([]resources.ProjectorRegistration, error) {
	if deps.SkillsRegistry == nil {
		return registrations, nil
	}
	return appendTypedProjectorRegistration(
		registrations,
		deps.CodecRegistry,
		skillspkg.SkillResourceKind,
		newSkillProjector(deps.SkillsRegistry),
	)
}

func (p *skillResourceProjector) Kind() resources.ResourceKind {
	return skillspkg.SkillResourceKind
}

func (p *skillResourceProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *skillResourceProjector) Build(
	_ context.Context,
	records []resources.Record[skillspkg.SkillResourceSpec],
) (resources.ProjectionPlan, error) {
	if p == nil || p.registry == nil {
		return nil, errors.New("daemon: skill resource projector is required")
	}
	var revision int64
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
	}
	return &skillResourceProjectionPlan{
		revision: revision,
		records:  cloneResourceRecords(records, cloneSkillResourceSpec),
	}, nil
}

func (p *skillResourceProjector) Apply(ctx context.Context, plan resources.ProjectionPlan) error {
	if p == nil || p.registry == nil {
		return errors.New("daemon: skill resource projector is required")
	}
	if ctx == nil {
		return errors.New("daemon: skill resource projector apply context is required")
	}
	typed, ok := plan.(*skillResourceProjectionPlan)
	if !ok {
		return fmt.Errorf("daemon: skill resource projector plan has type %T", plan)
	}
	return p.registry.ApplyResourceRecords(typed.revision, typed.records)
}
