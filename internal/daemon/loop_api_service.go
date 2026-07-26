package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type loopAPIPersistence interface {
	looppkg.Store
	looppkg.RunReader
	looppkg.CatalogRunReader
	looppkg.AnnotationStore
}

type loopGoalAPIPersistence interface {
	goalpkg.TurnReader
	goalpkg.SessionProjectionReader
}

type loopSessionStatusReader interface {
	Status(context.Context, string) (*session.Info, error)
}

type loopAPIWorkspaceResolver interface {
	workspacepkg.RuntimeResolver
	Get(context.Context, string) (workspacepkg.Workspace, error)
}

type daemonLoopAPIService struct {
	aggregate         looppkg.Service
	persistence       loopAPIPersistence
	catalogRuns       looppkg.CatalogRunReader
	goalPersistence   loopGoalAPIPersistence
	resolver          *daemonLoopDefinitionResolver
	catalog           *resourceCatalog[looppkg.ResourceSpec]
	publisher         loopResourcePublisher
	toolRegistry      toolspkg.Registry
	workspaceResolver loopAPIWorkspaceResolver
	homePaths         aghconfig.HomePaths
	now               func() time.Time
	goalContext       *loopGoalContextRuntime
	sessionStatus     loopSessionStatusReader
	creationStore     store.SessionCreationStore
	publishMu         sync.Mutex
}

var _ core.LoopService = (*daemonLoopAPIService)(nil)

func (s *daemonLoopAPIService) Start(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	name string,
	inputs looppkg.Inputs,
	actor taskpkg.ActorContext,
) (*looppkg.Run, error) {
	if s == nil || s.aggregate == nil {
		return nil, errors.New("daemon: loop aggregate is unavailable")
	}
	return s.aggregate.Start(ctx, ws, name, inputs, actor)
}

func newDaemonLoopAPIService(
	state *bootState,
	homePaths aghconfig.HomePaths,
	now func() time.Time,
) (core.LoopService, error) {
	if state == nil {
		return nil, errors.New("daemon: loop api state is required")
	}
	logger := state.logger
	if logger == nil {
		logger = slog.Default()
	}
	persistence, ok := state.registry.(loopAPIPersistence)
	if !ok {
		logger.Warn("loop api service disabled", "reason", "registry_missing_loop_persistence")
		return nil, nil
	}
	if state.loopCatalog == nil {
		logger.Warn("loop api service disabled", "reason", "loop_catalog_missing")
		return nil, nil
	}
	if state.workspaceResolver == nil {
		logger.Warn("loop api service disabled", "reason", "workspace_resolver_missing")
		return nil, nil
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	resolver := &daemonLoopDefinitionResolver{
		catalog:         state.loopCatalog,
		compilerFactory: newLoopCompilerFactory(state.deps.ToolRegistry),
	}
	options := []looppkg.Option{
		looppkg.WithClock(now),
		looppkg.WithLogger(logger),
		looppkg.WithDefaultsResolver(newLoopDefaultsResolver(homePaths, state.workspaceResolver)),
		looppkg.WithGoalRunActivator(loopGoalRunActivator{state: state}),
	}
	if state.participationResolver != nil {
		options = append(options, looppkg.WithParticipationResolver(state.participationResolver))
	}
	if revoker, ok := state.sessions.(loopManagedInputLeaseRevoker); ok {
		var judges *loopGateJudgeRunner
		if state.tasks != nil {
			judges = state.tasks.loopJudges
		}
		options = append(options, looppkg.WithGoalPromptLeaseRevoker(loopGoalPromptLeaseRevoker{
			sessions: revoker,
			judges:   judges,
		}))
	}
	if state.notifier != nil {
		options = append(options, looppkg.WithHookDispatcher(state.notifier))
	}
	aggregate, err := looppkg.NewService(
		persistence,
		resolver,
		newGoalRunPolicyResolver(homePaths, state.workspaceResolver),
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create loop aggregate api service: %w", err)
	}
	return &daemonLoopAPIService{
		aggregate:         aggregate,
		persistence:       persistence,
		catalogRuns:       persistence,
		goalPersistence:   goalPersistenceFromRegistry(state.registry),
		resolver:          resolver,
		catalog:           state.loopCatalog,
		publisher:         state.loopResources,
		toolRegistry:      state.deps.ToolRegistry,
		workspaceResolver: state.workspaceResolver,
		homePaths:         homePaths,
		now:               now,
		goalContext:       &loopGoalContextRuntime{sessions: state.sessions},
		sessionStatus:     state.sessions,
		creationStore:     sessionCreationStoreFromRegistry(state.registry),
	}, nil
}

func goalPersistenceFromRegistry(registry any) loopGoalAPIPersistence {
	persistence, ok := registry.(loopGoalAPIPersistence)
	if !ok {
		return nil
	}
	return persistence
}

func sessionCreationStoreFromRegistry(registry any) store.SessionCreationStore {
	persistence, ok := registry.(store.SessionCreationStore)
	if !ok {
		return nil
	}
	return persistence
}
func (s *daemonLoopAPIService) CreateLoop(
	ctx context.Context,
	workspaceID string,
	req contract.CreateLoopRequest,
) (contract.LoopResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	root, err := s.workspaceLoopRoot(ctx, ws)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	s.publishMu.Lock()
	defer s.publishMu.Unlock()

	if strings.TrimSpace(req.ForkFromName) != "" {
		if req.Definition != nil {
			return contract.LoopResponse{}, fmt.Errorf(
				"%w: definition and fork_from_name are mutually exclusive",
				looppkg.ErrValidation,
			)
		}
		_, record, err := s.findLoopRecord(workspaceID, req.ForkFromName)
		if err != nil {
			return contract.LoopResponse{}, err
		}
		return s.forkLoop(ctx, ws, record.Spec.FilePath, root, record.Spec.Name)
	}
	if req.Definition == nil {
		return contract.LoopResponse{}, fmt.Errorf(
			"%w: definition or fork_from_name is required",
			looppkg.ErrValidation,
		)
	}
	def, err := loopDefinitionDomain(*req.Definition)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	def.Meta.Version = 1
	if err := s.compileForPublish(ctx, def); err != nil {
		return contract.LoopResponse{}, err
	}
	path, err := s.writeDefinition(ctx, root, def, false)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	return s.loopResponseFromDefinitionFile(ctx, path, ws, def.Meta.Name)
}

func (s *daemonLoopAPIService) GetLoop(
	_ context.Context,
	workspaceID string,
	name string,
) (contract.LoopResponse, error) {
	_, record, err := s.findLoopRecord(workspaceID, name)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	def, err := daemonLoopDefinitionFromSpec(record.Spec)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	payload, err := loopDefinitionPayload(record.Spec, def)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	return contract.LoopResponse{Loop: payload}, nil
}

func (s *daemonLoopAPIService) PatchLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.PatchLoopRequest,
) (contract.LoopResponse, error) {
	if req.ExpectedVersion == nil {
		return contract.LoopResponse{}, fmt.Errorf("%w: expected_version is required", looppkg.ErrValidation)
	}
	next, err := loopDefinitionDomain(req.Definition)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	if strings.TrimSpace(next.Meta.Name) != strings.TrimSpace(name) {
		return contract.LoopResponse{}, fmt.Errorf(
			"%w: definition meta.name must match route name",
			looppkg.ErrValidation,
		)
	}
	if next.Meta.Version != *req.ExpectedVersion {
		return contract.LoopResponse{}, fmt.Errorf(
			"%w: definition meta.version must match expected_version",
			looppkg.ErrValidation,
		)
	}

	s.publishMu.Lock()
	defer s.publishMu.Unlock()

	ws, current, err := s.findLoopRecord(workspaceID, name)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	if *req.ExpectedVersion != current.Spec.Version {
		return contract.LoopResponse{}, &core.LoopVersionConflictError{CurrentVersion: current.Spec.Version}
	}
	if err := ensureWritableLoopSource(current.Spec); err != nil {
		return contract.LoopResponse{}, err
	}
	next.Meta.Version = current.Spec.Version + 1
	if err := s.compileForPublish(ctx, next); err != nil {
		return contract.LoopResponse{}, err
	}
	root, err := s.workspaceLoopRoot(ctx, ws)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	path, err := s.writeDefinition(ctx, root, next, true)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	return s.loopResponseFromDefinitionFile(ctx, path, ws, name)
}

func (s *daemonLoopAPIService) ValidateLoop(
	ctx context.Context,
	_ string,
	name string,
	req contract.ValidateLoopRequest,
) (contract.LoopValidationResponse, error) {
	def, err := loopDefinitionDomain(req.Definition)
	if err != nil {
		return contract.LoopValidationResponse{}, err
	}
	if strings.TrimSpace(name) != "" && strings.TrimSpace(def.Meta.Name) != strings.TrimSpace(name) {
		return contract.LoopValidationResponse{}, fmt.Errorf(
			"%w: definition meta.name must match route name",
			looppkg.ErrValidation,
		)
	}
	if err := s.compileForPublish(ctx, def); err != nil {
		return contract.LoopValidationResponse{}, err
	}
	return contract.LoopValidationResponse{Valid: true}, nil
}

func (s *daemonLoopAPIService) DeleteLoop(ctx context.Context, workspaceID string, name string) error {
	_, record, err := s.findLoopRecord(workspaceID, name)
	if err != nil {
		return err
	}
	if err := ensureWritableLoopSource(record.Spec); err != nil {
		return err
	}
	root := filepath.Dir(strings.TrimSpace(record.Spec.Dir))
	if strings.TrimSpace(root) == "." || strings.TrimSpace(root) == "" {
		return fmt.Errorf("%w: loop definition root is unavailable", looppkg.ErrValidation)
	}
	if err := looppkg.DeleteWritableDefinition(root, name); err != nil {
		return err
	}
	return s.syncLoopResources(ctx)
}

func (s *daemonLoopAPIService) RunLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.RunLoopRequest,
	startKind dsl.StartKind,
	actor taskpkg.ActorContext,
	dry bool,
) (contract.RunLoopResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.RunLoopResponse{}, err
	}
	values, err := cloneLoopAPIMap(req.Inputs)
	if err != nil {
		return contract.RunLoopResponse{}, err
	}
	inputs := looppkg.Inputs{
		Values:               values,
		ParentLoopRunID:      looppkg.RunID(strings.TrimSpace(req.ParentLoopRunID)),
		NetworkParticipation: participation.CloneRequest(req.NetworkParticipation),
	}
	if req.ConfigOverrides != nil {
		config, err := loopConfigDomain(*req.ConfigOverrides)
		if err != nil {
			return contract.RunLoopResponse{}, err
		}
		inputs.ConfigOverrides = config
	}
	if dry {
		if _, err := looppkg.ResolveStartBinding(ctx, s.resolver, ws, name, startKind); err != nil {
			return contract.RunLoopResponse{}, err
		}
		plan, err := s.aggregate.DryRun(ctx, ws, name, inputs)
		if err != nil {
			return contract.RunLoopResponse{}, err
		}
		payload, err := loopPlanPayload(plan)
		if err != nil {
			return contract.RunLoopResponse{}, err
		}
		return contract.RunLoopResponse{DryRun: payload}, nil
	}
	run, err := looppkg.StartFromActor(ctx, s.aggregate, s.resolver, looppkg.StartBindingRequest{
		WorkspaceID: ws,
		LoopName:    name,
		Kind:        startKind,
		Inputs:      inputs,
	}, actor)
	if err != nil {
		return contract.RunLoopResponse{}, err
	}
	payload, err := loopRunPayload(*run)
	if err != nil {
		return contract.RunLoopResponse{}, err
	}
	return contract.RunLoopResponse{Run: &payload}, nil
}
