package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/core"
	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

const callDispatchInterval = time.Second

type callRuntime struct {
	service *callspkg.Service
	cancel  context.CancelFunc
	done    chan struct{}
}

func (d *Daemon) bootCalls(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil || state.tasks == nil || state.tasks.manager == nil || state.sessions == nil {
		return nil
	}
	callStore, ok := state.registry.(callspkg.Store)
	if !ok {
		return errors.New("daemon: registry does not implement the calls store")
	}
	directory := &daemonCallDirectory{store: callStore, state: state}
	invoker := &daemonCallSessionInvoker{
		sessions: state.sessions, maxChildren: state.cfg.Calls.MaxChildren, maxDepth: state.cfg.Calls.MaxDepth,
	}
	service, err := callspkg.NewService(
		callspkg.WithStore(callStore),
		callspkg.WithDirectory(directory),
		callspkg.WithActivationClaimer(state.tasks.manager),
		callspkg.WithActivationRunCanceler(state.tasks.manager),
		callspkg.WithSessionInvoker(invoker),
		callspkg.WithConfig(state.cfg.Calls),
		callspkg.WithClock(d.now),
		callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		return fmt.Errorf("daemon: create calls service: %w", err)
	}
	if err := service.RecoverCallRuntime(ctx); err != nil {
		return fmt.Errorf("daemon: recover calls runtime: %w", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	runtime := &callRuntime{service: service, cancel: cancel, done: make(chan struct{})}
	state.calls = runtime
	go runtime.run(runCtx, state.logger)
	cleanup.add(runtime.shutdown)
	return nil
}

func (r *callRuntime) run(ctx context.Context, logger *slog.Logger) {
	defer close(r.done)
	if logger == nil {
		logger = slog.Default()
	}
	ticker := time.NewTicker(callDispatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if _, err := r.service.DispatchQueued(ctx, 100); err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("daemon: dispatch queued calls failed", "error", err)
			}
			if _, err := r.service.SweepDeadlines(ctx, now); err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("daemon: sweep call deadlines failed", "error", err)
			}
		}
	}
}

func (r *callRuntime) shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.cancel()
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: stop calls runtime: %w", ctx.Err())
	}
}

type daemonCallDirectory struct {
	store callspkg.Store
	state *bootState
}

func (d *daemonCallDirectory) ResolveCallTarget(
	ctx context.Context,
	input callspkg.CreateInput,
) (callspkg.TargetContext, []callspkg.AgentRosterEntry, error) {
	target, err := d.store.ResolveCallTargetContext(ctx, input)
	if err != nil {
		return callspkg.TargetContext{}, nil, err
	}
	catalog := agentCatalogDependency(d.state.agentCatalog)
	if catalog == nil {
		return target, nil, nil
	}
	var entries []core.AgentCatalogEntry
	if input.Scope == callspkg.ScopeWorkspace && d.state.workspaceResolver != nil {
		resolved, resolveErr := d.state.workspaceResolver.Resolve(ctx, input.WorkspaceID)
		if resolveErr != nil {
			return callspkg.TargetContext{}, nil, fmt.Errorf("daemon: resolve call workspace: %w", resolveErr)
		}
		entries, err = catalog.ListAgentsForWorkspace(ctx, &resolved)
	} else {
		entries, err = catalog.ListAgents(ctx)
	}
	if err != nil {
		return callspkg.TargetContext{}, nil, fmt.Errorf("daemon: list call agent roster: %w", err)
	}
	roster := make([]callspkg.AgentRosterEntry, 0, len(entries))
	requested := strings.TrimSpace(input.Target.Agent)
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Def.Name)
		roster = append(roster, callspkg.AgentRosterEntry{Name: name})
		if name == requested {
			target.AgentName = name
		}
	}
	return target, roster, nil
}

type daemonCallSessionInvoker struct {
	sessions    callSessionManager
	maxChildren int
	maxDepth    int
}

type callSessionManager interface {
	Status(context.Context, string) (*session.Info, error)
	Resume(context.Context, string) (*session.Session, error)
	SendPrompt(context.Context, string, session.SendPromptOpts) (session.SendPromptResult, error)
	StopWithCause(context.Context, string, session.StopCause, string) error
}

type callSessionSpawner interface {
	Spawn(context.Context, session.SpawnOpts) (*session.Session, error)
}

func (i *daemonCallSessionInvoker) SpawnChild(
	ctx context.Context,
	spec callspkg.ChildSpec,
) (callspkg.SessionRef, error) {
	desiredID := "ses_call_" + strings.TrimPrefix(spec.CallID, "call_")
	existing, err := i.sessions.Status(ctx, desiredID)
	if err == nil {
		if existing == nil || existing.AgentName != spec.AgentName || existing.Lineage == nil ||
			strings.TrimSpace(existing.Lineage.ParentSessionID) != strings.TrimSpace(spec.ParentSessionID) {
			return callspkg.SessionRef{}, errors.New("daemon: existing call child identity does not match activation")
		}
		if existing.State == session.StateStopped {
			if _, err := i.sessions.Resume(ctx, desiredID); err != nil {
				return callspkg.SessionRef{}, fmt.Errorf("daemon: resume call child %q: %w", desiredID, err)
			}
		}
		if _, err := i.send(ctx, desiredID, spec.Prompt, spec.CallID); err != nil {
			return callspkg.SessionRef{}, err
		}
		return callspkg.SessionRef{ID: desiredID}, nil
	}
	if !errors.Is(err, session.ErrSessionNotFound) {
		return callspkg.SessionRef{}, fmt.Errorf("daemon: inspect call child %q: %w", desiredID, err)
	}
	spawner, ok := i.sessions.(callSessionSpawner)
	if !ok {
		return callspkg.SessionRef{}, errors.New("daemon: session manager does not implement child spawn")
	}
	budget := &store.SessionSpawnBudget{MaxChildren: i.maxChildren, MaxDepth: i.maxDepth}
	child, err := spawner.Spawn(ctx, session.SpawnOpts{
		ParentSessionID: spec.ParentSessionID, DesiredSessionID: desiredID, AgentName: spec.AgentName,
		Provider: spec.Runtime.Provider, Model: spec.Runtime.Model,
		ReasoningEffort: spec.Runtime.ReasoningEffort, Speed: spec.Runtime.Speed,
		TTL: spec.IdleTTL, AutoStopOnParent: true, NotifyCreator: false, NotifyCreatorSet: true,
		PermissionPolicy: spec.Permissions.Policy(),
		GovernanceBudget: budget,
	})
	if err != nil {
		if errors.Is(err, session.ErrSpawnPermissionDenied) {
			return callspkg.SessionRef{}, &callspkg.Error{
				Code: callspkg.CodeWideningRejected, Message: "spawn hook widened caller permissions", Cause: err,
			}
		}
		return callspkg.SessionRef{}, err
	}
	if _, err := i.send(ctx, child.ID, spec.Prompt, spec.CallID); err != nil {
		cleanupErr := i.sessions.StopWithCause(ctx, child.ID, session.CauseFailed, "call activation prompt failed")
		return callspkg.SessionRef{}, errors.Join(err, cleanupErr)
	}
	return callspkg.SessionRef{ID: child.ID}, nil
}

func (i *daemonCallSessionInvoker) Revive(ctx context.Context, sessionID, prompt, callID string) error {
	if _, err := i.sessions.Resume(ctx, sessionID); err != nil {
		return err
	}
	_, err := i.send(ctx, sessionID, prompt, callID)
	if err == nil {
		return nil
	}
	cleanupErr := i.sessions.StopWithCause(ctx, sessionID, session.CauseFailed, "call revival prompt failed")
	return errors.Join(err, cleanupErr)
}

func (i *daemonCallSessionInvoker) DeliverAtBoundary(
	ctx context.Context,
	delivery callspkg.Delivery,
) (callspkg.DeliveryOutcome, error) {
	result, err := i.send(ctx, delivery.RecipientSessionID, delivery.Body, delivery.CallID+":"+delivery.Kind)
	if err != nil {
		return callspkg.DeliveryOutcome{}, err
	}
	return callspkg.DeliveryOutcome{State: result.Status, Reason: result.Delivery}, nil
}

func (i *daemonCallSessionInvoker) send(
	ctx context.Context,
	sessionID, message, identity string,
) (session.SendPromptResult, error) {
	identity = strings.TrimSpace(identity)
	return i.sessions.SendPrompt(ctx, sessionID, session.SendPromptOpts{
		Message: message, MessageID: "msg_" + identity, IdempotencyKey: "call:" + identity,
		Mode: session.BusyInputModeQueue,
	})
}

func (i *daemonCallSessionInvoker) StopManaged(ctx context.Context, sessionID, reason string) error {
	return i.sessions.StopWithCause(ctx, sessionID, session.CauseUserRequested, reason)
}

var _ callspkg.Directory = (*daemonCallDirectory)(nil)
var _ callspkg.SessionInvoker = (*daemonCallSessionInvoker)(nil)
