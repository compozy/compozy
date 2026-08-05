package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// Shutdown stops accepting new requests and drains active ones.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("httpapi: shutdown context is required")
	}

	for {
		generation, startDone, startCancel := s.shutdownTarget()
		if generation == nil {
			return nil
		}
		if startDone == nil {
			return s.shutdownGeneration(ctx, generation)
		}
		if startCancel != nil {
			startCancel()
		}
		if err := waitForStartDone(ctx, startDone); err != nil {
			return err
		}
	}
}

func (s *Server) shutdownGeneration(ctx context.Context, generation *serverGeneration) error {
	var shutdownErrs []error
	if generation.httpServer != nil {
		if err := generation.httpServer.Shutdown(ctx); err != nil {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("httpapi: shutdown http server: %w", err))
		}
	} else if generation.listener != nil {
		if err := generation.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("httpapi: close listener: %w", err))
		}
	}
	if generation.streamCancel != nil {
		generation.streamCancel()
	}

	for {
		ownsCleanup, cleanupDone, completedErr := s.claimShutdownCleanup(generation)
		if !ownsCleanup {
			if cleanupDone == nil {
				if completedErr != nil {
					shutdownErrs = append(shutdownErrs, completedErr)
				}
				return errors.Join(shutdownErrs...)
			}
			if err := waitForShutdownCleanup(ctx, cleanupDone); err != nil {
				shutdownErrs = append(shutdownErrs, err)
				return errors.Join(shutdownErrs...)
			}
			continue
		}

		if s.handlers != nil {
			if err := s.handlers.ShutdownWindowManagerStreams(ctx); err != nil {
				shutdownErrs = append(shutdownErrs, fmt.Errorf("httpapi: shutdown window-manager streams: %w", err))
			}
		}
		joinedServe := waitForServeDone(ctx, generation.serveDone)
		if joinedServe != nil {
			shutdownErrs = append(shutdownErrs, joinedServe)
		}
		if len(shutdownErrs) > 0 {
			if joinedServe == nil {
				if serveErr := s.serveError(generation); serveErr != nil {
					shutdownErrs = append(shutdownErrs, serveErr)
				}
			}
			s.releaseShutdownCleanup(generation)
			return errors.Join(shutdownErrs...)
		}

		return s.completeShutdown(generation)
	}
}
