package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/store"
)

// admitDeriveFirstMessage admits the derive's first message through the ordinary prompt
// admission store under "<derive key>:first" with a deterministic message id, so a
// retried derive completes an undispatched admission exactly once. The prompt runs
// detached; its delivery and completion are reported by the child's prompt events.
func (m *Manager) admitDeriveFirstMessage(ctx context.Context, spec deriveSpec, childID string) error {
	if spec.kind == store.LineageKindSide {
		return m.sendDeriveFirstMessage(context.WithoutCancel(ctx), spec, childID, nil)
	}
	run, start := m.acquireDeriveFirstPromptRun(childID)
	if start {
		m.startTrackedPromptTask(func() { m.runDeriveFirstPrompt(ctx, spec, childID, run) })
	}
	select {
	case <-run.ready:
		return run.admissionErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

type deriveFirstPromptRun struct {
	ready        chan struct{}
	admissionErr error // published by closing ready
}

type deriveFirstPromptReadyKey struct{}

type deriveFirstPromptReady struct {
	childID, admissionKey string
	acknowledge           func()
}

func acknowledgeDeriveFirstPrompt(ctx context.Context, childID, admissionKey string) {
	ready, ok := ctx.Value(deriveFirstPromptReadyKey{}).(deriveFirstPromptReady)
	if ok && ready.childID == childID && ready.admissionKey == admissionKey {
		ready.acknowledge()
	}
}

func (m *Manager) acquireDeriveFirstPromptRun(childID string) (*deriveFirstPromptRun, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if run := m.deriveFirstPrompts[childID]; run != nil {
		return run, false
	}
	if m.deriveFirstPrompts == nil {
		m.deriveFirstPrompts = make(map[string]*deriveFirstPromptRun)
	}
	run := &deriveFirstPromptRun{ready: make(chan struct{})}
	m.deriveFirstPrompts[childID] = run
	return run, true
}

func (m *Manager) runDeriveFirstPrompt(
	ctx context.Context,
	spec deriveSpec,
	childID string,
	run *deriveFirstPromptRun,
) {
	release := sync.OnceFunc(func() {
		m.mu.Lock()
		delete(m.deriveFirstPrompts, childID)
		m.mu.Unlock()
	})
	defer release()
	execution, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()
	stop := context.AfterFunc(m.fallbackLifecycleContext(), cancel)
	defer stop()
	admitted := false
	ready := func() {
		m.markDeriveFirstPromptAdmitted(spec, childID)
		admitted = true
		close(run.ready)
	}
	err := m.sendDeriveFirstMessage(execution, spec, childID, ready)
	if !admitted {
		run.admissionErr = err
		release()
		close(run.ready)
		return
	}
	if err != nil && !errors.Is(err, context.Canceled) &&
		!m.deriveFirstMessageDispatched(execution, childID, deriveFirstAdmissionKey(spec.key)) {
		m.recordDeriveFirstPromptFailure(spec, childID, err)
	}
}

func (m *Manager) sendDeriveFirstMessage(ctx context.Context, spec deriveSpec, childID string, ready func()) error {
	if ready != nil {
		meta, err := m.readSessionMetaReadOnly(ctx, childID)
		if err != nil {
			return err
		}
		if meta.Derivation != nil && meta.Derivation.FirstPrompt.Failed {
			return ErrDeriveFirstPromptFailed
		}
		ctx = context.WithValue(ctx, deriveFirstPromptReadyKey{}, deriveFirstPromptReady{
			childID: childID, admissionKey: deriveFirstAdmissionKey(spec.key), acknowledge: sync.OnceFunc(ready),
		})
	}
	deliveryCtx, cancelDelivery := context.WithCancel(ctx)
	defer cancelDelivery()
	_, err := m.SendPrompt(ctx, childID, SendPromptOpts{
		Message:         spec.message,
		MessageID:       deriveFirstMessageID(spec.workspaceID, spec.key),
		IdempotencyKey:  deriveFirstAdmissionKey(spec.key),
		DeliveryContext: deliveryCtx,
	})
	if err != nil {
		return fmt.Errorf("session: admit first message of derived session %q: %w", childID, err)
	}
	m.markDeriveFirstPromptAdmitted(spec, childID)
	return nil
}

func (m *Manager) markDeriveFirstPromptAdmitted(spec deriveSpec, childID string) {
	if child, ok := m.Get(childID); ok && child.markDeriveFirstPromptAdmitted(deriveFirstAdmissionKey(spec.key)) {
		if err := m.persistSessionMetadataOnly(child); err != nil {
			m.sessionLogger(child).Warn("session.derive.first_prompt_persist_failed", "error", err)
		}
	}
}

func (m *Manager) recordDeriveFirstPromptFailure(spec deriveSpec, childID string, err error) {
	child, ok := m.Get(childID)
	if !ok || !child.markDeriveFirstPromptFailed(deriveFirstAdmissionKey(spec.key)) {
		return
	}
	m.sessionLogger(child).
		Warn("session.derive.first_prompt_failed", "error", diagnostics.RedactAndBound(err.Error(), 2048))
	ctx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	if err := m.persistSessionLifecycleState(ctx, child, false); err != nil {
		m.sessionLogger(child).Warn("session.derive.first_prompt_persist_failed", "error", err)
	}
}

// recordImportedContextConsumption records the first dispatch that carried a derived
// child's imported context (its admission key and message id) in the child's meta.
func (m *Manager) recordImportedContextConsumption(
	session *Session,
	req promptRequest,
	replayBlock string,
) {
	if !session.hasImportedContext() ||
		(strings.TrimSpace(replayBlock) == "" && !isSideLineage(session.Info().Lineage)) {
		return
	}
	if !session.markImportedContextConsumed(
		strings.TrimSpace(req.idempotencyKey), strings.TrimSpace(req.messageID), m.now(),
	) {
		return
	}
	if err := m.persistSessionMetadataOnly(session); err != nil {
		m.sessionLogger(session).Warn(
			"session.imported_context.consumption_persist_failed",
			"error", err,
		)
	}
}
