package support

import (
	"context"
	"errors"
)

// Shutdown closes admission, cancels accepted work, and waits for it to settle.
func (s *Service) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("support: shutdown context is required")
	}

	s.shutdown.Do(func() {
		s.lifecycleMu.Lock()
		s.closing = true
		ownerCancel := s.ownerCancel
		s.lifecycleMu.Unlock()

		ownerCancel()
		go func() {
			s.work.Wait()
			close(s.shutdownDone)
		}()
	})

	select {
	case <-s.shutdownDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	operationCtx, cancel := detachedContext(ctx)
	stopOwnerCancellation := context.AfterFunc(s.ownerCtx, cancel)
	return operationCtx, func() {
		stopOwnerCancellation()
		cancel()
	}
}
