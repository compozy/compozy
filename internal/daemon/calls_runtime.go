package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	callDispatchInterval = time.Second
	callTurnEndTimeout   = 10 * time.Second
)

type callTurnEndService interface {
	DrainDeliveries(context.Context, string, int) error
	Return(context.Context, callspkg.ReturnInput) (callspkg.Settlement, error)
}

type callTurnEndSessionReader interface {
	IsPrompting(string) bool
	TranscriptPage(context.Context, string, transcript.PageQuery) (transcript.Page, error)
}

type callRuntime struct {
	service        *callspkg.Service
	turnEndService callTurnEndService
	sessions       callTurnEndSessionReader
	ctx            context.Context
	logger         *slog.Logger
	cancel         context.CancelFunc
	done           chan struct{}
}

func (d *Daemon) bootCalls(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil || state.tasks == nil || state.tasks.manager == nil || state.sessions == nil {
		return nil
	}
	callStore, ok := state.registry.(callspkg.Store)
	if !ok {
		return nil
	}
	callSessions, ok := state.sessions.(callSessionManager)
	if !ok {
		return nil
	}
	directory := &daemonCallDirectory{store: callStore, state: state}
	invoker := &daemonCallSessionInvoker{
		sessions: callSessions, isOperatorCallerSession: callStore.IsOperatorCallerSession,
		maxChildren: state.cfg.Calls.MaxChildren, maxDepth: state.cfg.Calls.MaxDepth,
	}
	service, err := callspkg.NewService(
		callspkg.WithStore(callStore),
		callspkg.WithDirectory(directory),
		callspkg.WithActivationClaimer(state.tasks.manager),
		callspkg.WithActivationRunCanceler(state.tasks.manager),
		callspkg.WithSessionInvoker(invoker),
		callspkg.WithPublishBridge(&daemonCallPublishBridge{network: func() coreNetworkSender {
			return state.network
		}}),
		callspkg.WithHookDispatcher(daemonCallHookDispatcher{
			state: state, logger: state.logger, now: d.now,
		}),
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
	if err := service.DrainDeliveries(ctx, "", 100); err != nil {
		logger := state.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("daemon: call deliveries remain pending after recovery", "error", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	runtime := &callRuntime{
		service: service, turnEndService: service, ctx: runCtx, logger: state.logger,
		cancel: cancel, done: make(chan struct{}),
	}
	runtime.sessions, _ = state.sessions.(callTurnEndSessionReader)
	state.calls = runtime
	state.deps.Calls = newCallSurfaceService(service, state.sessions)
	if registrar, ok := state.sessions.(turnEndNotifierRegistrar); ok {
		registrar.AddTurnEndNotifier(runtime.onTurnEnd)
	}
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
			if err := r.service.DrainDeliveries(ctx, "", 100); err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("daemon: drain call deliveries failed", "error", err)
			}
		}
	}
}

func (r *callRuntime) onTurnEnd(sessionID string) {
	if r == nil || r.turnEndService == nil || r.ctx == nil {
		return
	}
	turnCtx, cancel := context.WithTimeout(r.ctx, callTurnEndTimeout)
	defer cancel()
	if err := r.turnEndService.DrainDeliveries(turnCtx, sessionID, 100); err != nil {
		if !errors.Is(err, context.Canceled) {
			runtimeLogger(r.logger).Warn(
				"daemon: drain call deliveries at turn boundary failed",
				"session_id", sessionID,
				"error", err,
			)
		}
		return
	}
	if r.sessions == nil || r.sessions.IsPrompting(sessionID) {
		return
	}
	callID, finalText, err := r.finalCallTurn(turnCtx, sessionID)
	if err != nil {
		runtimeLogger(r.logger).Warn(
			"daemon: read final assistant text at call boundary failed",
			"session_id", sessionID,
			"error", err,
		)
		return
	}
	if callID == "" {
		return
	}
	_, err = r.turnEndService.Return(turnCtx, callspkg.ReturnInput{
		CallID:         callID,
		ChildSessionID: sessionID,
		FinalText:      finalText,
		ChildLive:      true,
		Actor: callspkg.SettlementActor{
			Kind: "agent_session",
			ID:   sessionID,
		},
	})
	if err == nil || callspkg.IsCode(err, callspkg.CodeReturnUnbound) ||
		callspkg.IsCode(err, callspkg.CodeAlreadySettled) {
		return
	}
	runtimeLogger(r.logger).Warn(
		"daemon: settle omitted call return at turn boundary failed",
		"session_id", sessionID,
		"error", err,
	)
}

func (r *callRuntime) finalCallTurn(ctx context.Context, sessionID string) (string, string, error) {
	page, err := r.sessions.TranscriptPage(ctx, sessionID, transcript.PageQuery{Limit: 50})
	if err != nil {
		return "", "", err
	}
	callInput := -1
	callID := ""
	for index := len(page.Entries) - 1; index >= 0; index-- {
		var metadata struct {
			Synthetic *struct {
				CallID string `json:"call_id"`
			} `json:"synthetic"`
		}
		if len(page.Entries[index].Message.Metadata) == 0 ||
			json.Unmarshal(page.Entries[index].Message.Metadata, &metadata) != nil || metadata.Synthetic == nil {
			continue
		}
		if candidate := strings.TrimSpace(metadata.Synthetic.CallID); candidate != "" {
			callInput = index
			callID = candidate
			break
		}
	}
	if callInput < 0 {
		return "", "", nil
	}
	finalText := ""
	for index := len(page.Entries) - 1; index > callInput; index-- {
		message := page.Entries[index].Message
		if message.Role == transcript.UIRoleAssistant {
			finalText = transcript.UIMessageText(message)
			break
		}
	}
	return callID, finalText, nil
}

func runtimeLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}

var _ callTurnEndService = (*callspkg.Service)(nil)
var _ callTurnEndSessionReader = (*session.Manager)(nil)

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
