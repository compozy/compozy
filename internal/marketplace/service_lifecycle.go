package marketplace

import (
	"context"
	"errors"
)

// Close cancels and joins every refresh flight owned by this service generation.
func (s *CatalogService) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("marketplace catalog: close context is required")
	}
	s.closeOnce.Do(func() {
		s.flightMu.Lock()
		s.closed = true
		if s.lifecycleStop != nil {
			s.lifecycleStop()
		}
		s.flightMu.Unlock()
		go func() {
			s.flightWG.Wait()
			close(s.closeDone)
		}()
	})
	select {
	case <-s.closeDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *CatalogService) lifecycleError() error {
	if s == nil {
		return nil
	}
	s.flightMu.Lock()
	defer s.flightMu.Unlock()
	if s.closed {
		return ErrServiceClosed
	}
	return nil
}
