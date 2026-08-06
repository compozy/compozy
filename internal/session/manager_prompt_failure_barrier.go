package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) prepareFatalPromptFailureEvent(
	ctx context.Context,
	session *Session,
	event acp.AgentEvent,
	fatal *promptPumpFatal,
) bool {
	if m == nil || session == nil || event.Failure == nil || fatal == nil {
		return false
	}
	if fatal.finalizationOwned {
		return true
	}
	proc := session.processHandle()
	if proc == nil {
		return false
	}

	summary := firstTrimmedNonEmpty(
		failureSummary(event.Failure, event.Error),
		strings.TrimSpace(event.Error),
		"agent runtime became unavailable during prompt",
	)
	stopCtx, cancel := detachedPromptStopContext(ctx, m)
	defer cancel()
	if err := m.waitForConversationFinalization(stopCtx, session.ID); err != nil {
		m.logFatalPromptBarrierFailure(session, event.Failure.Kind, err)
		return false
	}
	prepared, preparedProc, alreadyStopped, _, _, err := m.prepareStopWithCauseMode(
		stopCtx,
		session.ID,
		CauseProcessExited,
		summary,
		stopPreparationFatalPromptFailure,
	)
	if err != nil || alreadyStopped || prepared != session || preparedProc == nil {
		if err == nil {
			err = errors.New("session: fatal prompt stop preparation did not retain an active process")
		}
		m.logFatalPromptBarrierFailure(session, event.Failure.Kind, err)
		return false
	}
	proc = preparedProc
	owned, _ := m.claimFinalization(session)
	if !owned {
		err = errors.New("session: fatal prompt process barrier could not own finalization")
		m.logFatalPromptBarrierFailure(session, event.Failure.Kind, err)
		return false
	}

	waitErr, err := m.stopProcessForFatalPromptFailure(stopCtx, session, proc, event.Failure, summary)
	if err != nil {
		m.finishFinalization(session.ID, err)
		m.logFatalPromptBarrierFailure(session, event.Failure.Kind, err)
		return false
	}
	fatal.ownFinalization(waitErr)
	return true
}

func (m *Manager) logFatalPromptBarrierFailure(
	session *Session,
	kind store.FailureKind,
	err error,
) {
	m.sessionLogger(session).Warn(
		"session: fatal prompt process barrier failed",
		"failure_kind", kind,
		"error", err,
	)
}

func (m *Manager) stopProcessForFatalPromptFailure(
	ctx context.Context,
	session *Session,
	proc *AgentProcess,
	failure *store.SessionFailure,
	summary string,
) (error, error) {
	if proc == nil {
		return nil, errors.New("session: fatal prompt process is unavailable")
	}
	doneBeforeStop := isProcessDone(proc)
	if !doneBeforeStop {
		if err := m.driver.Stop(ctx, proc); err != nil && !isProcessDone(proc) {
			return nil, fmt.Errorf("session: stop process after fatal prompt failure: %w", err)
		}
	}
	if !isProcessDone(proc) {
		select {
		case <-proc.Done():
		case <-ctx.Done():
			return nil, fmt.Errorf("session: wait for fatal prompt process exit: %w", ctx.Err())
		}
	}

	processWaitErr := proc.Wait()
	typedFailure := acp.WrapFailure(failure.Kind, summary, errors.New(summary))
	session.setStopCause(CauseProcessExited)
	return errors.Join(typedFailure, processWaitErr), nil
}

func (m *Manager) finalizeSessionAfterFatalPromptFailure(
	ctx context.Context,
	session *Session,
	fatal *promptPumpFatal,
) {
	if m == nil || session == nil || fatal == nil || !fatal.finalizationOwned {
		return
	}
	session.setFailure(store.CloneSessionFailure(fatal.failure))
	stopCtx, cancel := detachedPromptStopContext(ctx, m)
	defer cancel()
	if err := m.finalizeStoppedOwned(stopCtx, session, fatal.waitErr); err != nil {
		m.sessionLogger(session).Warn(
			"session: finalize after fatal prompt failure failed",
			"failure_kind", fatal.failure.Kind,
			"error", err,
		)
	}
}
