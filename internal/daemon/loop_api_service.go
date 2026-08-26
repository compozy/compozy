package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	goalpkg "github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type loopAPIPersistence interface {
	looppkg.Store
	looppkg.RunReader
	looppkg.CatalogRunReader
	looppkg.AnnotationStore
	looppkg.DefinitionStateStore
	looppkg.GenerationLineageReader
	looppkg.RouteCauseReader
	gate.VerdictReader
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

type loopProfileNameResolver interface {
	ProfileName(context.Context, string) (string, error)
}

type daemonLoopAPIService struct {
	aggregate         looppkg.Service
	persistence       loopAPIPersistence
	catalogRuns       looppkg.CatalogRunReader
	goalPersistence   loopGoalAPIPersistence
	resolver          looppkg.DefinitionResolver
	catalog           *resourceCatalog[looppkg.ResourceSpec]
	publisher         loopResourcePublisher
	toolRegistry      toolspkg.Registry
	workspaceResolver loopAPIWorkspaceResolver
	profiles          loopProfileNameResolver
	homePaths         compozyconfig.HomePaths
	now               func() time.Time
	goalContext       *loopGoalContextRuntime
	sessionStatus     loopSessionStatusReader
	creationStore     store.SessionCreationStore
	runtimeCatalog    looppkg.WorkspaceRuntimeCatalog
	responderPolicy   looppkg.ResponderPolicy
	runReads          looppkg.RunReadService
	logger            *slog.Logger
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
	homePaths compozyconfig.HomePaths,
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
		profiles:        state.profiles,
	}
	runtimeCatalog := loopRuntimeCatalogFactory{
		homePaths: homePaths, workspaceResolver: state.workspaceResolver,
	}
	responderPolicy := daemonLoopResponderPolicy{runs: persistence, sessions: state.sessions}
	readStore, ok := state.registry.(looppkg.RunReadStore)
	if !ok {
		return nil, errors.New("daemon: loop run read persistence is unavailable")
	}
	aggregate, err := looppkg.NewService(
		persistence,
		resolver,
		newGoalRunPolicyResolver(homePaths, state.workspaceResolver),
		loopAPIServiceOptions(state, homePaths, now, logger, runtimeCatalog, responderPolicy)...,
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
		profiles:          state.profiles,
		homePaths:         homePaths,
		now:               now,
		goalContext:       &loopGoalContextRuntime{sessions: state.sessions},
		sessionStatus:     state.sessions,
		creationStore:     sessionCreationStoreFromRegistry(state.registry),
		runtimeCatalog:    runtimeCatalog,
		responderPolicy:   responderPolicy,
		runReads:          looppkg.NewRunReadService(readStore, now),
		logger:            logger,
	}, nil
}

func loopAPIServiceOptions(
	state *bootState,
	homePaths compozyconfig.HomePaths,
	now func() time.Time,
	logger *slog.Logger,
	runtimeCatalog looppkg.WorkspaceRuntimeCatalog,
	responderPolicy looppkg.ResponderPolicy,
) []looppkg.Option {
	options := []looppkg.Option{
		looppkg.WithClock(now),
		looppkg.WithLogger(logger),
		looppkg.WithDefaultsResolver(newLoopDefaultsResolver(homePaths, state.workspaceResolver)),
		looppkg.WithInputDefaultsResolver(newLoopInputDefaultsResolver(homePaths, state.workspaceResolver)),
		looppkg.WithGoalRunActivator(loopGoalRunActivator{state: state}),
		looppkg.WithWorkerRunActivator(loopWorkerRunActivator{state: state}),
		looppkg.WithCoordinatorRunActivator(loopCoordinatorRunActivator{state: state}),
		looppkg.WithRuntimeCatalog(runtimeCatalog),
		looppkg.WithInputEntityCatalog(daemonLoopInputEntityCatalog{state: state}),
		looppkg.WithCancellationSessionController(loopCancellationSessionController{sessions: state.sessions}),
		looppkg.WithResponderPolicy(responderPolicy),
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
	return options
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
	profileID string,
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
		_, record, err := s.findLoopRecord(ctx, workspaceID, profileID, req.ForkFromName)
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
	ctx context.Context,
	workspaceID string,
	profileID string,
	name string,
) (contract.LoopResponse, error) {
	_, record, err := s.findLoopRecord(ctx, workspaceID, profileID, name)
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
	if s.aggregate == nil {
		return contract.LoopResponse{Loop: payload}, nil
	}
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	snapshot, err := s.aggregate.GetConfigSnapshot(ctx, ws, profileID, name)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	var storedLifecycle *looppkg.LifecycleConfig
	if snapshot.Stored != nil {
		storedLifecycle = snapshot.Stored.Lifecycle
	}
	resolvedLifecycle, err := looppkg.ResolveNodeLifecycleConfig(
		dsl.Node{}, storedLifecycle, snapshot.Effective.Lifecycle,
	)
	if err != nil {
		return contract.LoopResponse{}, fmt.Errorf("daemon: resolve effective Loop lifecycle: %w", err)
	}
	lifecyclePayload := loopResolvedLifecyclePayload(resolvedLifecycle)
	effectivePayload, err := loopEffectiveConfigPayload(snapshot.Effective)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	payload.EffectiveConfig = &effectivePayload
	payload.EffectiveLifecycle = &lifecyclePayload
	return contract.LoopResponse{Loop: payload}, nil
}

func (s *daemonLoopAPIService) PatchLoop(
	ctx context.Context,
	workspaceID string,
	profileID string,
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

	ws, current, err := s.findLoopRecord(ctx, workspaceID, profileID, name)
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
	workspaceID string,
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
	var catalog looppkg.RuntimeCatalog
	if s.runtimeCatalog != nil {
		ws, normalizeErr := normalizeLoopWorkspaceID(workspaceID)
		if normalizeErr != nil {
			return contract.LoopValidationResponse{}, normalizeErr
		}
		catalog, err = s.runtimeCatalog.ForWorkspace(ctx, ws)
		if err != nil {
			return contract.LoopValidationResponse{}, fmt.Errorf("resolve workspace runtime catalog: %w", err)
		}
	}
	if err := looppkg.ValidateDefinitionRuntime(ctx, catalog, def); err != nil {
		return contract.LoopValidationResponse{}, err
	}
	if err := s.compileForPublish(ctx, def); err != nil {
		return contract.LoopValidationResponse{}, err
	}
	return contract.LoopValidationResponse{Valid: true}, nil
}

func (s *daemonLoopAPIService) RunLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	input core.LoopRunInput,
) (contract.RunLoopResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.RunLoopResponse{}, err
	}
	values, err := cloneLoopAPIMap(input.Request.Inputs)
	if err != nil {
		return contract.RunLoopResponse{}, err
	}
	inputs := looppkg.Inputs{
		ProfileID:            strings.TrimSpace(input.ProfileID),
		Values:               values,
		ParentLoopRunID:      looppkg.RunID(strings.TrimSpace(input.Request.ParentLoopRunID)),
		NetworkParticipation: participation.CloneRequest(input.Request.NetworkParticipation),
	}
	if input.Request.ConfigOverrides != nil {
		config, err := loopConfigDomain(*input.Request.ConfigOverrides)
		if err != nil {
			return contract.RunLoopResponse{}, err
		}
		inputs.ConfigOverrides = config
	}
	if input.Dry {
		if _, err := looppkg.ResolveStartBinding(
			ctx,
			s.resolver,
			ws,
			inputs.ProfileID,
			name,
			input.StartKind,
		); err != nil {
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
	webEndpoint, err := s.resolveLoopRunWebEndpoint(ctx, ws)
	if err != nil {
		return contract.RunLoopResponse{}, err
	}
	run, err := looppkg.StartFromActor(ctx, s.aggregate, s.resolver, looppkg.StartBindingRequest{
		WorkspaceID: ws,
		LoopName:    name,
		Kind:        input.StartKind,
		Inputs:      inputs,
	}, input.Actor)
	if err != nil {
		return contract.RunLoopResponse{}, err
	}
	payload, err := loopRunPayload(*run)
	if err != nil {
		return contract.RunLoopResponse{}, err
	}
	return contract.RunLoopResponse{Run: &payload, WebURL: webEndpoint.runURL(payload.ID)}, nil
}
