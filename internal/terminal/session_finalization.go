package terminal

import (
	"context"
	"fmt"
)

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
	if err := s.manager.closeJournalTerminal(ctx, s); err != nil {
		return fmt.Errorf("terminal: retry journal close for %q: %w", s.info.ID, err)
	}
	s.finalizationMu.Lock()
	s.journalClosePending = false
	s.finalizationMu.Unlock()
	s.audit.SetBlocked(false)
	return nil
}
