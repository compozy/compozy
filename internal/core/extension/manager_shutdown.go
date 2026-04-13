package extensions

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type shutdownRequest struct {
	Reason     string `json:"reason"`
	DeadlineMS int64  `json:"deadline_ms"`
}

type shutdownResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

// Shutdown cooperatively drains every started extension and escalates through
// the shared subprocess package when a process refuses to exit.
func (m *Manager) Shutdown(ctx context.Context) error {
	return m.shutdownWithReason(ctx, shutdownReasonForContext(ctx))
}

func (m *Manager) shutdownWithReason(ctx context.Context, reason string) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.shutdownOnce.Do(func() {
		unregisterActiveManager(m)

		if m.shutdownHook != nil {
			m.setAllStates(ExtensionStateDraining)
			defer m.setAllStates(ExtensionStateStopped)
			m.shutdownErr = m.shutdownHook(ctx)
			m.shutdownErr = errors.Join(m.shutdownErr, m.closeAudit(contextWithoutCancel(ctx)))
			return
		}

		m.stopEventForwarding()
		m.setAllStates(ExtensionStateDraining)

		sessions := m.sessionSnapshot()
		var (
			wg       sync.WaitGroup
			errMu    sync.Mutex
			combined error
		)
		for _, session := range sessions {
			wg.Add(1)
			go func(session *extensionSession) {
				defer wg.Done()

				if err := m.shutdownSession(contextWithoutCancel(ctx), session, reason); err != nil {
					errMu.Lock()
					combined = errors.Join(combined, err)
					errMu.Unlock()
				}
			}(session)
		}
		wg.Wait()

		m.setAllStates(ExtensionStateStopped)
		m.backgroundGroup.Wait()
		m.shutdownErr = errors.Join(combined, m.closeAudit(contextWithoutCancel(ctx)))
		if err := ctx.Err(); err != nil {
			m.shutdownErr = errors.Join(m.shutdownErr, err)
		}
	})

	return m.shutdownErr
}

func (m *Manager) shutdownSession(ctx context.Context, session *extensionSession, reason string) error {
	if session == nil || session.runtime == nil {
		return nil
	}

	var shutdownErr error
	session.stopOnce.Do(func() {
		deadline := defaultShutdownTimeout(session.runtime)
		session.runtime.SetState(ExtensionStateDraining)
		session.degraded.Store(true)

		startedAt := time.Now()
		callCtx, cancel := context.WithTimeout(contextWithoutCancel(ctx), deadline)
		defer cancel()

		var response shutdownResponse
		callErr := session.Call(callCtx, "shutdown", shutdownRequest{
			Reason:     reason,
			DeadlineMS: deadline.Milliseconds(),
		}, &response)

		var processErr error
		if session.process != nil {
			session.process.CloseInput()
			remaining := deadline - time.Since(startedAt)
			if remaining < 0 {
				remaining = 0
			}
			if err := session.process.Shutdown(remaining); err != nil {
				processErr = fmt.Errorf("shutdown extension process %q: %w", session.runtime.normalizedName(), err)
			}
		}

		if processErr != nil {
			shutdownErr = errors.Join(shutdownErr, processErr)
		}
		if callErr != nil && !errors.Is(callErr, errExtensionSessionClosed) && processErr != nil {
			shutdownErr = errors.Join(
				shutdownErr,
				fmt.Errorf("send shutdown to extension %q: %w", session.runtime.normalizedName(), callErr),
			)
		}

		session.runtime.SetState(ExtensionStateStopped)
		m.recordLifecycleAudit(session.runtime, "shutdown", time.Since(startedAt), shutdownErr)
	})

	return shutdownErr
}

func shutdownReasonForContext(ctx context.Context) string {
	if ctx == nil {
		return shutdownReasonRunCompleted
	}
	if err := ctx.Err(); err != nil {
		return shutdownReasonRunCancelled
	}
	return shutdownReasonRunCompleted
}
