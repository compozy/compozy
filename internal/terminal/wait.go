package terminal

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	idleSampleInterval  = 100 * time.Millisecond
	idleSamplesNeeded   = 3
	idleMaximumWait     = 700 * time.Millisecond
	waitSnapshotTimeout = time.Second
)

func (s *session) Wait(ctx context.Context, condition WaitCondition) (*WaitResult, error) {
	if ctx == nil {
		return nil, errors.New("terminal: wait context is required")
	}
	until, matcher, err := prepareWaitCondition(condition)
	if err != nil {
		return nil, err
	}
	waitCtx := ctx
	var cancel context.CancelFunc
	if condition.TimeoutMs > 0 {
		waitCtx, cancel = context.WithTimeoutCause(
			ctx,
			time.Duration(condition.TimeoutMs)*time.Millisecond,
			&Error{Code: "terminal_wait_timeout", Message: "terminal wait timed out", Err: ErrWaitTimeout},
		)
		defer cancel()
	}
	if until == terminalWaitIdle {
		return s.waitIdle(waitCtx)
	}
	for {
		s.mu.RLock()
		exit := cloneExit(s.exit)
		readerEnded := s.readerEnded
		revisionReady := s.revisionReady
		s.mu.RUnlock()
		if exit != nil {
			if until == terminalWaitExit {
				select {
				case <-waitCtx.Done():
					cause := context.Cause(waitCtx)
					if errors.Is(cause, ErrWaitTimeout) {
						return s.waitTimeoutResult(waitCtx, cause)
					}
					return nil, cause
				case <-s.done:
				}
				s.mu.RLock()
				exit = cloneExit(s.exit)
				s.mu.RUnlock()
			}
			return s.waitExitResult(waitCtx, exit)
		}
		if until == terminalWaitMatch {
			output, _ := s.ring.Snapshot()
			if matcher.Match(output) {
				return s.waitStateResult(waitCtx, terminalWaitMatch)
			}
		}
		if readerEnded && until != terminalWaitExit {
			return s.waitStateResult(waitCtx, "stalled")
		}
		select {
		case <-waitCtx.Done():
			cause := context.Cause(waitCtx)
			if errors.Is(cause, ErrWaitTimeout) {
				return s.waitTimeoutResult(waitCtx, cause)
			}
			return nil, cause
		case <-revisionReady:
		}
	}
}

func prepareWaitCondition(condition WaitCondition) (string, *regexp.Regexp, error) {
	until := strings.TrimSpace(condition.Until)
	if until == "" {
		until = terminalWaitExit
	}
	if until != terminalWaitExit && until != terminalWaitMatch && until != terminalWaitIdle {
		return "", nil, &Error{
			Code: "terminal_wait_condition_invalid", Message: "terminal wait condition must be exit, match, or idle",
			Err: ErrUnsupported,
		}
	}
	if until != terminalWaitMatch {
		return until, nil, nil
	}
	if strings.TrimSpace(condition.Pattern) == "" {
		return "", nil, &Error{
			Code:    "terminal_wait_pattern_required",
			Message: "terminal wait match requires a pattern",
			Err:     ErrUnsupported,
		}
	}
	matcher, err := regexp.Compile(condition.Pattern)
	if err != nil {
		return "", nil, &Error{
			Code: "terminal_wait_pattern_invalid", Message: "terminal wait pattern is invalid",
			Err: errors.Join(ErrUnsupported, err),
		}
	}
	return until, matcher, nil
}

func (s *session) waitIdle(ctx context.Context) (*WaitResult, error) {
	deadline := time.NewTimer(idleMaximumWait)
	defer deadline.Stop()
	ticker := time.NewTicker(idleSampleInterval)
	defer ticker.Stop()
	stable := 0
	s.mu.RLock()
	lastRevision := s.revision
	s.mu.RUnlock()
	for {
		select {
		case <-ctx.Done():
			cause := context.Cause(ctx)
			if errors.Is(cause, ErrWaitTimeout) {
				return s.waitTimeoutResult(ctx, cause)
			}
			return nil, cause
		case <-deadline.C:
			return s.waitStateResult(ctx, "still_running")
		case <-ticker.C:
			s.mu.RLock()
			exit := cloneExit(s.exit)
			revision := s.revision
			readerEnded := s.readerEnded
			s.mu.RUnlock()
			if exit != nil {
				return s.waitExitResult(ctx, exit)
			}
			if readerEnded {
				return s.waitStateResult(ctx, "stalled")
			}
			if lastRevision == revision {
				stable++
			} else {
				stable = 0
				lastRevision = revision
			}
			if stable >= idleSamplesNeeded {
				return s.waitStateResult(ctx, terminalWaitIdle)
			}
		}
	}
}

func (s *session) waitTimeoutResult(ctx context.Context, cause error) (*WaitResult, error) {
	snapshotCtx, cancel := boundedCleanupContext(ctx, waitSnapshotTimeout)
	defer cancel()
	result, snapshotErr := s.waitStateResult(snapshotCtx, "timeout")
	return result, errors.Join(cause, snapshotErr)
}

func (s *session) currentScreen(ctx context.Context) (string, error) {
	read, screenErr := s.Screen(ctx, ReadOptions{View: terminalViewScreen})
	if screenErr == nil {
		return read.Content, nil
	}
	read, tailErr := s.Screen(ctx, ReadOptions{View: terminalViewTail, MaxBytes: 64 << 10})
	if tailErr != nil {
		return "", errors.Join(
			fmt.Errorf("terminal: snapshot wait screen: %w", screenErr),
			fmt.Errorf("terminal: snapshot wait tail: %w", tailErr),
		)
	}
	return read.Content, nil
}

func (s *session) waitStateResult(ctx context.Context, reason string) (*WaitResult, error) {
	screen, err := s.currentScreen(ctx)
	return &WaitResult{Reason: reason, Screen: screen, Untrusted: true}, err
}

func (s *session) waitExitResult(ctx context.Context, exit *Exit) (*WaitResult, error) {
	result, err := s.waitStateResult(ctx, terminalWaitExit)
	result.ExitCode = exit.Code
	return result, err
}
