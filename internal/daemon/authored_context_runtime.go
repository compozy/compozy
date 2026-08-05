package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/acp"
	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"

	"github.com/compozy/compozy/internal/session"
)

const (
	taskRecoverySessionMissing = "missing"
)

const (
	authoredContextRuntimeValidKey = "valid"
)

type authoredContextDeps struct {
	SoulAuthoring      core.SoulAuthoringService
	SoulHistoryPurger  core.SoulHistoryPurger
	SoulRefresher      core.SoulRefresher
	HeartbeatAuthoring core.HeartbeatAuthoringService
	HeartbeatPurger    core.HeartbeatHistoryPurger
	HeartbeatStatus    core.HeartbeatStatusService
	HeartbeatWake      core.HeartbeatWakeService
	SessionHealth      core.SessionHealthReader
	WakeEvents         core.HeartbeatWakeEventReader
	wakePrompter       *apiHeartbeatWakePrompter
}

type apiHeartbeatWakePrompter struct {
	ctx      context.Context
	sessions SessionManager
	logger   *slog.Logger
	workers  *ownedWorkerGroup
}

type heartbeatWakeHealthReader struct {
	reader heartbeat.SessionHealthReader
}

func authoredContextRuntimeDeps(ctx context.Context, state *bootState, sessions SessionManager) authoredContextDeps {
	var deps authoredContextDeps
	if state == nil {
		return deps
	}
	soulService := soulAuthoringServiceDependency(state.registry, state.logger)
	if soulService != nil {
		deps.SoulAuthoring = soulService
		deps.SoulHistoryPurger = soulService
	}
	deps.SoulRefresher = soulRefresherDependency(sessions)
	heartbeatService := heartbeatAuthoringServiceDependency(state.registry, state.logger)
	if heartbeatService != nil {
		deps.HeartbeatAuthoring = heartbeatService
		deps.HeartbeatPurger = heartbeatService
	}
	deps.SessionHealth = sessionHealthReaderDependency(sessions)
	deps.HeartbeatStatus = heartbeatStatusServiceDependency(
		state.registry,
		sessions,
		agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
			soul:      state.soulCatalog,
			heartbeat: state.heartbeatCatalog,
		}),
		state.logger,
	)
	deps.HeartbeatWake, deps.wakePrompter = heartbeatWakeServiceDependency(
		ctx,
		state.registry,
		sessions,
		state.cfg.Agents.Heartbeat,
		state.logger,
	)
	deps.WakeEvents = heartbeatWakeEventReaderDependency(state.registry)
	deps.SoulAuthoring = hookSoulAuthoringService(deps.SoulAuthoring, state.notifier)
	deps.HeartbeatAuthoring = hookHeartbeatAuthoringService(deps.HeartbeatAuthoring, state.notifier)
	deps.HeartbeatStatus = hookHeartbeatStatusService(deps.HeartbeatStatus, state.notifier)
	deps.HeartbeatWake = hookHeartbeatWakeService(deps.HeartbeatWake, state.notifier)
	return deps
}

func soulRefresherDependency(sessions SessionManager) core.SoulRefresher {
	refresher, ok := sessions.(core.SoulRefresher)
	if !ok {
		return nil
	}
	return refresher
}

func heartbeatStatusServiceDependency(
	store any,
	sessions SessionManager,
	policyResolver heartbeat.PolicyResolver,
	logger *slog.Logger,
) core.HeartbeatStatusService {
	statusStore, ok := store.(heartbeat.StatusStore)
	if !ok {
		return nil
	}
	options := make([]heartbeat.StatusOption, 0, 2)
	if reader, ok := sessions.(heartbeat.SessionHealthReader); ok {
		options = append(options, heartbeat.WithHeartbeatStatusSessionHealthReader(reader))
	}
	if policyResolver != nil {
		options = append(options, heartbeat.WithHeartbeatStatusPolicyResolver(policyResolver))
	}
	service, err := heartbeat.NewManagedHeartbeatStatusService(statusStore, options...)
	if err != nil {
		logAuthoredContextDependencyError(logger, "daemon: create heartbeat status service", err)
		return nil
	}
	return service
}

func heartbeatWakeServiceDependency(
	ctx context.Context,
	store any,
	sessions SessionManager,
	config compozyconfig.HeartbeatConfig,
	logger *slog.Logger,
) (core.HeartbeatWakeService, *apiHeartbeatWakePrompter) {
	if ctx == nil {
		logAuthoredContextDependencyError(
			logger,
			"daemon: create heartbeat wake service",
			errors.New("context is required"),
		)
		return nil, nil
	}
	wakeStore, ok := store.(heartbeat.WakeStore)
	if !ok {
		return nil, nil
	}
	healthReader, ok := sessions.(heartbeat.SessionHealthReader)
	if !ok {
		return nil, nil
	}
	lifecycleCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	prompter := &apiHeartbeatWakePrompter{
		ctx:      lifecycleCtx,
		sessions: sessions,
		logger:   logger,
		workers:  newOwnedWorkerGroup(cancel),
	}
	service, err := heartbeat.NewManagedWakeService(
		wakeStore,
		heartbeatWakeHealthReader{reader: healthReader},
		prompter,
		config,
	)
	if err != nil {
		prompter.workers.Stop()
		logAuthoredContextDependencyError(logger, "daemon: create heartbeat wake service", err)
		return nil, nil
	}
	return service, prompter
}

func sessionHealthReaderDependency(sessions SessionManager) core.SessionHealthReader {
	reader, ok := sessions.(core.SessionHealthReader)
	if !ok {
		return nil
	}
	return reader
}

func (r heartbeatWakeHealthReader) GetSessionHealth(
	ctx context.Context,
	sessionID string,
) (heartbeat.SessionHealth, error) {
	if r.reader == nil {
		return heartbeat.SessionHealth{}, heartbeat.ErrSessionHealthNotFound
	}
	health, err := r.reader.GetSessionHealth(ctx, sessionID)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return heartbeat.SessionHealth{}, fmt.Errorf("%w: %s", heartbeat.ErrSessionHealthNotFound, sessionID)
		}
		return heartbeat.SessionHealth{}, err
	}
	return health, nil
}

func heartbeatWakeEventReaderDependency(store any) core.HeartbeatWakeEventReader {
	reader, ok := store.(core.HeartbeatWakeEventReader)
	if !ok {
		return nil
	}
	return reader
}

func (p *apiHeartbeatWakePrompter) PromptHeartbeatWake(
	ctx context.Context,
	req heartbeat.SyntheticWakePromptRequest,
) (heartbeat.SyntheticWakePromptResult, error) {
	if p == nil || p.sessions == nil {
		return heartbeat.SyntheticWakePromptResult{}, errors.New("daemon: api heartbeat prompter requires sessions")
	}
	synthetic, ok := p.sessions.(schedulerSyntheticPrompter)
	if !ok {
		return heartbeat.SyntheticWakePromptResult{}, errors.New(
			"daemon: api heartbeat prompter requires synthetic prompt support",
		)
	}
	complete, admitted := p.workers.Begin()
	if !admitted {
		return heartbeat.SyntheticWakePromptResult{}, errors.New("daemon: API heartbeat prompter is stopping")
	}
	events, err := synthetic.PromptSynthetic(ctx, req.SessionID, session.SyntheticPromptOpts{
		Message: req.Message,
		TurnID:  req.TurnID,
		Metadata: acp.PromptSyntheticMeta{
			TaskID:               req.SyntheticCorrelation.TaskID,
			TaskRunID:            req.SyntheticCorrelation.TaskRunID,
			WorkflowID:           req.SyntheticCorrelation.WorkflowID,
			ClaimTokenHash:       req.SyntheticCorrelation.ClaimTokenHash,
			CoordinatorSessionID: req.SyntheticCorrelation.CoordinatorSessionID,
			Reason:               heartbeat.SyntheticReasonHeartbeatWake,
			Summary:              req.Summary,
			WakeEventID:          req.WakeEventID,
			PolicySnapshotID:     req.PolicySnapshotID,
			PolicyDigest:         req.PolicyDigest,
			ConfigDigest:         req.ConfigDigest,
		},
		SkipIfBusy: true,
	})
	if err != nil {
		complete()
		if errors.Is(err, session.ErrPromptInProgress) {
			return heartbeat.SyntheticWakePromptResult{}, heartbeat.ErrSyntheticPromptBusy
		}
		return heartbeat.SyntheticWakePromptResult{}, err
	}
	p.drainEvents(req.SessionID, req.WakeEventID, events, complete)
	return heartbeat.SyntheticWakePromptResult{SyntheticPromptID: req.TurnID}, nil
}

func (p *apiHeartbeatWakePrompter) drainEvents(
	sessionID string,
	wakeEventID string,
	events <-chan acp.AgentEvent,
	complete func(),
) {
	if events == nil {
		complete()
		return
	}
	drainCtx := p.ctx
	go func() {
		defer complete()
		for {
			select {
			case <-drainCtx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if event.Type == acp.EventTypeError && p != nil && p.logger != nil {
					p.logger.Warn(
						"api.heartbeat_wake.agent_error",
						"session_id", sessionID,
						"wake_event_id", wakeEventID,
					)
				}
			}
		}
	}()
}

func (p *apiHeartbeatWakePrompter) shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: API heartbeat prompter shutdown context is required")
	}
	select {
	case <-p.workers.Stop():
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: shutdown API heartbeat prompter: %w", ctx.Err())
	}
}

func logAuthoredContextDependencyError(logger *slog.Logger, message string, err error) {
	if logger == nil || err == nil {
		return
	}
	logger.Warn(message, "error", err)
}

func hookSoulAuthoringService(
	next core.SoulAuthoringService,
	hooks *hooksNotifier,
) core.SoulAuthoringService {
	if next == nil {
		return nil
	}
	return hookedSoulAuthoringService{next: next, hooks: hooks}
}
