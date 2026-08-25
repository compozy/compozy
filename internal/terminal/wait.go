package terminal

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	idleSampleInterval = 100 * time.Millisecond
	idleSamplesNeeded  = 3
	idleMaximumWait    = 700 * time.Millisecond
)

func (s *session) Wait(ctx context.Context, condition WaitCondition) (*WaitResult, error) {
	if ctx == nil {
		return nil, errors.New("terminal: wait context is required")
	}
	until := strings.TrimSpace(condition.Until)
	if until == "" {
		until = "exit"
	}
	var matcher *regexp.Regexp
	if until == "match" {
		if strings.TrimSpace(condition.Pattern) == "" {
			return nil, &Error{Code: "terminal_wait_pattern_required", Message: "terminal wait match requires a pattern", Err: ErrUnsupported}
		}
		compiled, err := regexp.Compile(condition.Pattern)
		if err != nil {
			return nil, &Error{Code: "terminal_wait_pattern_invalid", Message: err.Error(), Err: ErrUnsupported}
		}
		matcher = compiled
	}
	if until != "exit" && until != "match" && until != "idle" {
		return nil, &Error{Code: "terminal_wait_condition_invalid", Message: "terminal wait condition must be exit, match, or idle", Err: ErrUnsupported}
	}
	waitCtx := ctx
	var cancel context.CancelFunc
	if condition.TimeoutMs > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, time.Duration(condition.TimeoutMs)*time.Millisecond)
		defer cancel()
	}
	if until == "idle" {
		return s.waitIdle(waitCtx)
	}
	for {
		s.mu.RLock()
		exit := cloneExit(s.exit)
		readerEnded := s.readerEnded
		revisionReady := s.revisionReady
		s.mu.RUnlock()
		if exit != nil {
			if until == "exit" {
				select {
				case <-waitCtx.Done():
					return nil, waitCtx.Err()
				case <-s.done:
				}
				s.mu.RLock()
				exit = cloneExit(s.exit)
				s.mu.RUnlock()
			}
			return s.waitExitResult(exit), nil
		}
		if until == "match" {
			output, _ := s.ring.Snapshot()
			if matcher.Match(output) {
				return &WaitResult{Reason: "match", Screen: s.currentScreen(waitCtx), Untrusted: true}, nil
			}
		}
		if readerEnded && until != "exit" {
			return &WaitResult{Reason: "stalled", Screen: s.currentScreen(waitCtx), Untrusted: true}, nil
		}
		select {
		case <-waitCtx.Done():
			if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
				return &WaitResult{Reason: "timeout", Screen: s.currentScreen(context.Background()), Untrusted: true}, nil
			}
			return nil, waitCtx.Err()
		case <-revisionReady:
		}
	}
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
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return &WaitResult{Reason: "timeout", Screen: s.currentScreen(context.Background()), Untrusted: true}, nil
			}
			return nil, ctx.Err()
		case <-deadline.C:
			return &WaitResult{Reason: "still_running", Screen: s.currentScreen(context.Background()), Untrusted: true}, nil
		case <-ticker.C:
			s.mu.RLock()
			exit := cloneExit(s.exit)
			revision := s.revision
			readerEnded := s.readerEnded
			s.mu.RUnlock()
			if exit != nil {
				return s.waitExitResult(exit), nil
			}
			if readerEnded {
				return &WaitResult{Reason: "stalled", Screen: s.currentScreen(context.Background()), Untrusted: true}, nil
			}
			if lastRevision == revision {
				stable++
			} else {
				stable = 0
				lastRevision = revision
			}
			if stable >= idleSamplesNeeded {
				return &WaitResult{Reason: "idle", Screen: s.currentScreen(context.Background()), Untrusted: true}, nil
			}
		}
	}
}

func (s *session) currentScreen(ctx context.Context) string {
	read, err := s.Screen(ctx, ReadOptions{View: "screen"})
	if err != nil {
		read, err = s.Screen(ctx, ReadOptions{View: "tail", MaxBytes: 64 << 10})
		if err != nil {
			return ""
		}
	}
	return read.Content
}

func (s *session) waitExitResult(exit *Exit) *WaitResult {
	return &WaitResult{
		Reason: "exit", ExitCode: exit.Code,
		Screen: s.currentScreen(context.Background()), Untrusted: true,
	}
}
