package terminal

import (
	"context"
	"fmt"
)

func (s *session) persistCommandBeforeExit(ctx context.Context, info Info) {
	if s.beforeExitPublish == nil {
		return
	}
	if err := s.beforeExitPublish(ctx, info); err != nil {
		s.audit.SetBlocked(true)
		s.manager.logger.Error(
			"terminal: persist command before publishing exit",
			"terminal_id", info.ID,
			"error", err,
		)
	}
}

func (s *session) awaitRemoval(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("terminal: wait for terminal %q finalization: %w", s.info.ID, context.Cause(ctx))
	case <-s.done:
	}

	s.finalizationMu.Lock()
	pending := s.journalClosePending
	s.finalizationMu.Unlock()
	if !pending {
		return nil
	}
	retryCtx, cancel := context.WithTimeout(ctx, defaultJournalShutdownTimeout)
	defer cancel()
	if err := s.manager.closeJournalTerminal(retryCtx, s); err != nil {
		return fmt.Errorf("terminal: retry journal close for %q: %w", s.info.ID, err)
	}
	s.finalizationMu.Lock()
	s.journalClosePending = false
	s.finalizationMu.Unlock()
	s.audit.SetBlocked(false)
	return nil
}
