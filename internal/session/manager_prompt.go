package session

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
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
	return event.Failure.Kind == store.FailureProcess
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

	return m.submitPromptRequest(ctx, req)
}

// CancelPrompt cancels prompt setup/execution for a known session.
func (m *Manager) CancelPrompt(ctx context.Context, id string) error {
	if m == nil {
		return errors.New("session: manager is required")
	}
	if ctx == nil {
		return errors.New("session: cancel prompt context is required")
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return errors.New("session: session id is required")
	}

	session, ok := m.Get(target)
	if !ok {
		if _, err := m.readMetaWithContext(ctx, target); err != nil {
			return err
		}
		return nil
	}
	if !session.IsPrompting() {
		return nil
	}
	turnID := session.CurrentTurnID()
	session.cancelCurrentPrompt()

	proc := session.processHandle()
	if proc == nil {
		m.emitTranscriptMarker(
			ctx,
			session,
			turnID,
			transcript.MarkerPromptCancel,
			"Prompt canceled by operator.",
			map[string]any{transcriptMarkerEvidenceSourceKey: "cancel_prompt"},
		)
		return nil
	}

	cancelErr := m.driver.Cancel(ctx, proc)
	if cancelErr != nil {
		if isProcessDone(proc) {
			return nil
		}
		return fmt.Errorf("session: cancel prompt for %q: %w", target, cancelErr)
	}
	if scoped, ok := m.driver.(ScopedInterrupter); ok && strings.TrimSpace(turnID) != "" {
		if _, err := scoped.Interrupt(ctx, target, turnID); err != nil &&
			!errors.Is(err, ErrScopedInterruptNotFound) {
			return fmt.Errorf("session: interrupt scoped tools for %q: %w", target, err)
		}
	}
	m.emitTranscriptMarker(
		ctx,
		session,
		turnID,
		transcript.MarkerPromptCancel,
		"Prompt canceled by operator.",
		map[string]any{transcriptMarkerEvidenceSourceKey: "cancel_prompt"},
	)
	return nil
}

// ApprovePermission resolves one pending interactive permission request for an active session.
func (m *Manager) ApprovePermission(ctx context.Context, id string, req acp.ApproveRequest) error {
	if ctx == nil {
		return errors.New("session: approval context is required")
	}
	if err := req.Validate(); err != nil {
		return err
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return errors.New("session: session id is required")
	}

	session, ok := m.Get(target)
	if !ok {
		meta, err := m.readMetaWithContext(ctx, target)
		if err != nil {
			return err
		}
		return fmt.Errorf("%w: %s (%s)", ErrSessionNotActive, target, meta.State)
	}

	if err := session.ApprovePermission(ctx, req); err != nil {
		switch {
		case errors.Is(err, ErrSessionNotActive):
			return err
		case errors.Is(err, acp.ErrPendingPermissionNotFound):
			return fmt.Errorf("%w: %s", ErrPendingPermissionNotFound, target)
		case errors.Is(err, acp.ErrPendingPermissionConflict):
			return fmt.Errorf("%w: %s", ErrPendingPermissionConflict, target)
		case errors.Is(err, acp.ErrPermissionDecisionUnsupported):
			return fmt.Errorf("%w: %s", ErrInvalidPermissionDecision, target)
		default:
			return err
		}
	}
	return nil
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
