package session

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/retry"
	"github.com/compozy/compozy/internal/store"
)

const (
	attentionRetryInitialDelay = 100 * time.Millisecond
	attentionRetryMaxDelay     = 5 * time.Second
)

type attentionReconciliationLifecycle struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	closing bool
	wg      sync.WaitGroup
}

// PublishPendingInteractionCommits reconciles committed acknowledgements in the manager lifetime.
// after runs only after every affected session has refreshed and published its attention edge.
// It returns false when shutdown has closed worker admission.
func (m *Manager) PublishPendingInteractionCommits(
	commits []store.SessionAttentionCommit,
	after func(),
) bool {
	pending := make([]store.SessionAttentionCommit, 0, len(commits))
	seen := make(map[string]bool)
	for _, commit := range commits {
		if commit.Interaction == nil || seen[commit.Interaction.SessionID] {
			continue
		}
		seen[commit.Interaction.SessionID] = true
		commit.Interaction = new(*commit.Interaction)
		pending = append(pending, commit)
	}
	lifecycle := &m.attentionReconciliation
	lifecycle.mu.Lock()
	if lifecycle.closing {
		lifecycle.mu.Unlock()
		return false
	}
	lifecycle.wg.Add(1)
	lifecycle.mu.Unlock()
	go func() {
		defer lifecycle.wg.Done()
		m.reconcileAcknowledgedInteractions(lifecycle.ctx, pending, after)
	}()
	return true
}

// reconcileAcknowledgedInteractions retries failed refreshes without repeating successful edges.
func (m *Manager) reconcileAcknowledgedInteractions(
	ctx context.Context,
	pending []store.SessionAttentionCommit,
	after func(),
) {
	for delay := attentionRetryInitialDelay; ctx.Err() == nil; delay = min(2*delay, attentionRetryMaxDelay) {
		failed := pending[:0]
		for _, commit := range pending {
			if ctx.Err() != nil {
				return
			}
			if err := m.publishPendingInteractionCommit(
				ctx,
				commit,
			); err != nil &&
				!errors.Is(err, store.ErrSessionNotFound) {
				m.logger.WarnContext(ctx, "session: refresh acknowledged interaction failed; retrying",
					"session_id", commit.Interaction.SessionID, "error", err)
				failed = append(failed, commit)
			}
		}
		pending = failed
		if len(pending) == 0 {
			if ctx.Err() == nil && after != nil {
				after()
			}
			return
		}
		if err := retry.Wait(ctx, delay); err != nil {
			return
		}
	}
}

// stopAttentionReconciliation closes admission, cancels reads and joins admitted workers.
func (m *Manager) stopAttentionReconciliation() {
	lifecycle := &m.attentionReconciliation
	lifecycle.mu.Lock()
	lifecycle.closing = true
	if lifecycle.cancel != nil {
		lifecycle.cancel()
	}
	lifecycle.mu.Unlock()
	lifecycle.wg.Wait()
}

// publishPendingInteractionCommit bounds each refresh attempt independently of callers and earlier reads.
func (m *Manager) publishPendingInteractionCommit(ctx context.Context, commit store.SessionAttentionCommit) error {
	id := commit.Interaction.SessionID
	var refreshed store.SessionAttentionCommit
	var err error
	for range 2 {
		attemptCtx, cancel := context.WithTimeout(ctx, defaultLifecycleTimeout)
		unlock, lockErr := m.lockAttentionCommit(attemptCtx, id)
		if lockErr != nil {
			cancel()
			return lockErr
		}
		refreshed, err = m.commitAttentionLocked(id, func() (store.SessionAttentionCommit, error) {
			current, err := m.attentionStore.GetSessionAttention(attemptCtx, id)
			if err != nil {
				return commit, err
			}
			if err := m.refreshPendingInteractions(attemptCtx, id); err != nil {
				return commit, err
			}
			commit.After = current
			return commit, nil
		})
		if err == nil {
			err = m.emitAttentionCommit(attemptCtx, id, refreshed)
			unlock()
			cancel()
			if err == nil {
				return nil
			}
			continue
		}
		unlock()
		cancel()
	}
	return err
}
