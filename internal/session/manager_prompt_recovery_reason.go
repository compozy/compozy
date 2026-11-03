package session

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// authRecoveryContinueMessage resumes a turn whose provider sign-in dropped after it had started
// work; the reloaded provider session already holds that work, so resending the request would repeat it.
const authRecoveryContinueMessage = "Your previous turn stopped because the provider sign-in was " +
	"briefly unavailable. It is available again: continue the interrupted work. Verify the current " +
	"state before repeating actions: a tool call without a recorded result may already have completed. " +
	"Do not assume that an interrupted action had no effect."

// promptRecoveryReasonFor maps a turn failure to the automatic recovery that can wait it out.
func promptRecoveryReasonFor(event acp.AgentEvent) (store.SessionRuntimeRecoveryReason, bool) {
	if event.Type != acp.EventTypeError || event.Failure == nil {
		return "", false
	}
	switch event.Failure.Kind {
	case store.FailureProcess, store.FailureTransport:
		return store.SessionRuntimeRecoveryRuntimeExit, true
	case store.FailurePrompt:
		if event.ProviderError != nil && event.ProviderError.Code == acp.ProviderErrorAuthRequired {
			return store.SessionRuntimeRecoveryProviderAuth, true
		}
	}
	return "", false
}

func (m *Manager) promptRecoverySchedule(reason store.SessionRuntimeRecoveryReason) []time.Duration {
	if reason == store.SessionRuntimeRecoveryProviderAuth {
		return m.authRecoveryDelays
	}
	return m.promptRecoveryDelays
}

// promptRecoveryAvailable reports whether this failure class may start or continue recovering.
func promptRecoveryAvailable(session *Session, reason store.SessionRuntimeRecoveryReason) bool {
	return reason != store.SessionRuntimeRecoveryProviderAuth || session.authRecoveryAvailable()
}

// recoveredPromptPayload chooses what the replacement runtime receives: the original request
// while the provider has not acted on the turn, or a continuation once a lost sign-in
// interrupted work the reloaded provider session already holds.
func recoveredPromptPayload(
	recovery *promptRecoveryState,
	reason store.SessionRuntimeRecoveryReason,
) (acp.PromptRequest, string) {
	request := clonePromptRecoveryRequest(recovery.request)
	if reason != store.SessionRuntimeRecoveryProviderAuth || !recovery.workObserved || request.BypassesContext() {
		return request, recovery.originalMessage
	}
	request.Attachments = nil
	request.MentionLinks = nil
	return request, authRecoveryContinueMessage
}

// settleStoppedPromptRecovery leaves a non-fatal failure on the runtime the session kept; a
// runtime-exit failure is finalized by the fatal prompt path instead.
func (m *Manager) settleStoppedPromptRecovery(
	ctx context.Context,
	session *Session,
	reason store.SessionRuntimeRecoveryReason,
) error {
	if reason != store.SessionRuntimeRecoveryProviderAuth || !session.abandonAutomaticRecovery(m.now()) {
		return nil
	}
	return m.persistSessionLifecycleState(ctx, session, false)
}

func (m *Manager) maxPromptRecoveryWait() time.Duration {
	var total time.Duration
	for _, delays := range [][]time.Duration{m.promptRecoveryDelays, m.authRecoveryDelays} {
		for _, delay := range delays {
			total += delay + defaultLifecycleTimeout
		}
	}
	return total
}
