package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

type promptSubmissionPath string

const (
	promptSubmissionPathUserFacing promptSubmissionPath = "user_facing"
	promptSubmissionPathSynthetic  promptSubmissionPath = "synthetic"
	jsonReasonFieldReason                               = "reason"
	maxMCPAuthReasonJSONDepth                           = 16
)

type promptPumpLoopState struct {
	source              <-chan acp.AgentEvent
	runtime             <-chan acp.AgentEvent
	activity            *promptActivitySupervisor
	turnEnded           bool
	sourceProbeRequired bool
	fileMutations       fileMutationVerifier
	fileMutationMarked  bool
}

func (s *promptPumpLoopState) active() bool {
	return s != nil && (s.source != nil || s.runtime != nil)
}

func (s *promptPumpLoopState) sourceClosedShouldReturn() bool {
	if s == nil {
		return true
	}
	s.source = nil
	s.sourceProbeRequired = false
	s.stopRuntime()
	return s.runtime == nil || s.turnEnded
}

func (s *promptPumpLoopState) runtimeClosedShouldReturn() bool {
	if s == nil {
		return true
	}
	s.runtime = nil
	s.sourceProbeRequired = false
	return s.turnEnded || s.source == nil
}

func (s *promptPumpLoopState) turnEndedShouldReturn() bool {
	if s == nil {
		return true
	}
	s.turnEnded = true
	s.sourceProbeRequired = false
	s.stopRuntime()
	return s.runtime == nil
}

func (s *promptPumpLoopState) stopRuntime() {
	if s == nil || s.activity == nil || s.runtime == nil {
		return
	}
	s.activity.stop()
}

func isPromptTerminalEvent(eventType string) bool {
	return eventType == acp.EventTypeDone || eventType == acp.EventTypeError
}

func isFatalPromptFailureEvent(event acp.AgentEvent) bool {
	if event.Type != acp.EventTypeError || event.Failure == nil {
		return false
	}
	switch event.Failure.Kind {
	case store.FailureProcess, store.FailureTransport:
		return true
	default:
		return false
	}
}

// Prompt sends one prompt turn to an active session and mirrors the runtime stream into storage and observers.
func (m *Manager) Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error) {
	return m.PromptWithOpts(ctx, id, PromptOpts{
		Message:    msg,
		TurnSource: TurnSourceUser,
	})
}

// PromptWithOpts sends one prompt turn with daemon-local provenance metadata.
func (m *Manager) PromptWithOpts(ctx context.Context, id string, opts PromptOpts) (<-chan acp.AgentEvent, error) {
	req, err := m.parsePromptRequest(ctx, id, opts)
	if err != nil {
		return nil, err
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}
	if err := m.admitManagedInstancePrompt(ctx, req.target); err != nil {
		return nil, err
	}
	m.attachDaemonLocalBudgetAdmission(ctx, &req)
	// Daemon-local callers (automations, wakes, judges, extensions) are never human ingress, but their
	// mentions still resolve against the target session like every other admission path.
	if req.delivery != PromptDeliveryMaintenance {
		m.prepareMentions(ctx, &req)
	}

	return m.submitPromptRequest(ctx, req)
}

// attachDaemonLocalBudgetAdmission is shared by regular daemon-local prompts
// and native maintenance compaction, both of which can invoke the provider.
func (m *Manager) attachDaemonLocalBudgetAdmission(ctx context.Context, req *promptRequest) {
	// Daemon-local turns have no persisted external prompt admission. Check the
	// spend fence at their existing last-before-driver commit callback, after
	// runtime preparation but before any provider effect. A recovered already-
	// started turn never goes through this new-turn gate.
	runID, _ := ctx.Value(instanceManagedRunKey{}).(string)
	req.commitDispatch = func(commitCtx context.Context) error {
		var trip *store.InstancePauseTrip
		err := m.withDailyBudgetAdmission(commitCtx, req.target, func(lockedCtx context.Context) error {
			if runID != "" {
				if err := m.admitManagedInstancePrompt(WithManagedRun(lockedCtx, runID), req.target); err != nil {
					return err
				}
			}
			guard, ok := m.promptAdmissionStore.(interface {
				AdmitDailyBudgetTurn(context.Context, string) (bool, *store.InstancePauseTrip, error)
			})
			if !ok {
				return nil
			}
			allowed, paused, err := guard.AdmitDailyBudgetTurn(lockedCtx, req.target)
			trip = paused
			if err != nil {
				return err
			}
			if !allowed {
				return &store.DailyBudgetTurnBlockedError{PausedInstance: trip}
			}
			return nil
		})
		if trip != nil && m.instancePaused != nil {
			if effectErr := m.instancePaused(m.fallbackLifecycleContext(), *trip); effectErr != nil {
				m.logger.ErrorContext(commitCtx, "session: apply daemon-local daily budget pause effects",
					"instance_id", trip.InstanceID, "error", effectErr)
			}
		}
		return err
	}
}

// ensureDailyBudgetObserverReady forwards a failed live usage write into the
// session's provider-admission boundary. The optional notifier capability is
// inactive for legacy managers without instance-budget support.
func (m *Manager) ensureDailyBudgetObserverReady(ctx context.Context, sessionID string) error {
	if m == nil || m.notifier == nil {
		return nil
	}
	if observer, ok := m.notifier.(interface {
		EnsureDailyBudgetReady(context.Context, string) error
	}); ok {
		return observer.EnsureDailyBudgetReady(ctx, sessionID)
	}
	return nil
}

// withDailyBudgetAdmission holds the observer's receipt/admission fence through
// the *durable* claim or dispatch commit. The earlier preflight remains useful
// for fast failures, but is never the final correctness boundary.
func (m *Manager) withDailyBudgetAdmission(
	ctx context.Context, sessionID string, commit func(context.Context) error,
) error {
	if m != nil && m.notifier != nil {
		if observer, ok := m.notifier.(interface {
			WithDailyBudgetAdmission(context.Context, string, func(context.Context) error) error
		}); ok {
			return observer.WithDailyBudgetAdmission(ctx, sessionID, commit)
		}
	}
	if err := m.ensureDailyBudgetObserverReady(ctx, sessionID); err != nil {
		return fmt.Errorf("%w: %w", store.ErrDailyBudgetRecoveryPending, err)
	}
	return commit(ctx)
}

// RequestPermission asks an active session's permission path for a tool-call decision.
func (m *Manager) RequestPermission(
	ctx context.Context,
	id string,
	req acp.RequestPermissionRequest,
) (acp.RequestPermissionResponse, error) {
	if ctx == nil {
		return acp.RequestPermissionResponse{}, errors.New("session: permission context is required")
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return acp.RequestPermissionResponse{}, errors.New("session: session id is required")
	}

	session, ok := m.Get(target)
	if !ok {
		meta, err := m.readMetaWithContext(ctx, target)
		if err != nil {
			return acp.RequestPermissionResponse{}, err
		}
		return acp.RequestPermissionResponse{}, fmt.Errorf("%w: %s (%s)", ErrSessionNotActive, target, meta.State)
	}

	response, err := session.RequestPermission(ctx, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrSessionNotActive):
			return acp.RequestPermissionResponse{}, err
		default:
			return acp.RequestPermissionResponse{}, fmt.Errorf("session: request permission for %q: %w", target, err)
		}
	}
	return response, nil
}
