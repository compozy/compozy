package journal

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

const (
	idleCommandDelay      = 300 * time.Millisecond
	maximumIdleInputBytes = 64 << 10
)

type idleCandidate struct {
	command   string
	actor     terminalpkg.Actor
	startedAt time.Time
}

type inputReservation struct {
	lane  *terminalLane
	count int
	once  sync.Once
}

func (r *inputReservation) Commit(actor terminalpkg.Actor, input terminalpkg.JournalInput) {
	if r == nil {
		return
	}
	r.once.Do(func() {
		candidates := 0
		if input.Redacted {
			candidates = r.lane.observeRedactedInput(input.Characters, actor)
		} else {
			candidates = r.lane.observeInput(input.Content, actor)
		}
		if candidates < r.count {
			r.lane.release(r.count - candidates)
		}
		r.lane.finishInputReservation()
	})
}

func (r *inputReservation) Release() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		r.lane.release(r.count)
		r.lane.finishInputReservation()
	})
}

// ObserveInput records accepted human input as an approximate fallback candidate.
func (s *Service) ObserveInput(info terminalpkg.Info, actor terminalpkg.Actor, input []byte) {
	journalInput := terminalpkg.JournalInput{Content: input}
	reservation, admitted := s.ReserveInput(info, journalInput)
	if !admitted {
		return
	}
	reservation.Commit(actor, journalInput)
}

// ReserveInput reserves bounded journal capacity before input reaches the PTY.
func (s *Service) ReserveInput(
	info terminalpkg.Info,
	input terminalpkg.JournalInput,
) (terminalpkg.JournalInputReservation, bool) {
	lane := s.lane(info)
	if lane == nil {
		return nil, false
	}
	count := inputSubmissionCount(input.Content)
	if input.Redacted {
		count = max(count, 1)
	}
	reservation, admitted := lane.reserveInput(count)
	if !admitted {
		return nil, false
	}
	return reservation, true
}

// ObserveOutput postpones approximate completion while the terminal is still producing bytes.
func (s *Service) ObserveOutput(info terminalpkg.Info, output []byte) {
	lane := s.lane(info)
	if lane != nil {
		lane.observeOutput(output)
	}
}

func (l *terminalLane) observeInput(input []byte, actor terminalpkg.Actor) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return 0
	}
	candidates := 0
	for _, value := range input {
		switch value {
		case '\r', '\n':
			if l.scheduleIdleCandidateLocked(actor) {
				candidates++
			}
		case '\b', 0x7f:
			_, size := utf8.DecodeLastRune(l.input)
			if size > 0 {
				l.input = l.input[:len(l.input)-size]
			}
		case '\t':
			l.appendInputByteLocked(' ')
		default:
			if value >= 0x20 {
				l.appendInputByteLocked(value)
			}
		}
	}
	return candidates
}

func (l *terminalLane) appendInputByteLocked(value byte) {
	if len(l.input) >= maximumIdleInputBytes {
		return
	}
	l.input = append(l.input, value)
}

func (l *terminalLane) scheduleIdleCandidateLocked(actor terminalpkg.Actor) bool {
	command := strings.TrimSpace(string(l.input))
	l.input = l.input[:0]
	if command == "" {
		return false
	}
	if len(l.idle) == 0 && l.assembly == nil {
		l.outputTail = nil
	}
	l.idle = append(l.idle, idleCandidate{
		command: scrubCommand(command), actor: actor, startedAt: l.service.now(),
	})
	l.resetIdleTimerLocked()
	return true
}

func (l *terminalLane) resetIdleTimerLocked() {
	if l.idleTimer != nil {
		l.idleTimer.Stop()
	}
	l.idleGeneration++
	generation := l.idleGeneration
	l.idleTimer = time.AfterFunc(idleCommandDelay, func() { l.finishIdleCandidates(generation) })
}

func (l *terminalLane) observeOutput(output []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	if l.assembly != nil || len(l.idle) > 0 {
		l.appendOutputTailLocked(output)
	}
	if len(l.idle) == 0 {
		return
	}
	l.resetIdleTimerLocked()
}

func (l *terminalLane) observeRedactedInput(characters int, actor terminalpkg.Actor) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return 0
	}
	marker := terminalpkg.RedactedInputMarker(characters)
	if l.assembly == nil && len(l.idle) == 0 {
		l.outputTail = nil
		l.idle = append(l.idle, idleCandidate{
			command: terminalpkg.RenderOutputSegment(marker), actor: actor, startedAt: l.service.now(),
		})
		l.resetIdleTimerLocked()
		l.appendOutputSegmentLocked(marker)
		return 1
	}
	l.appendOutputSegmentLocked(marker)
	return 0
}

func (l *terminalLane) cancelIdleCandidate() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.idle) == 0 {
		return
	}
	l.idle = l.idle[1:]
	if len(l.idle) == 0 && l.idleTimer != nil {
		l.idleTimer.Stop()
	}
}

func (l *terminalLane) finishIdleCandidates(generation uint64) {
	l.mu.Lock()
	if l.closed || len(l.idle) == 0 || l.idleGeneration != generation {
		l.mu.Unlock()
		return
	}
	candidates := append([]idleCandidate(nil), l.idle...)
	l.idle = nil
	l.mu.Unlock()
	for _, candidate := range candidates {
		l.finishIdleCandidate(candidate)
	}
}

func (l *terminalLane) finishIdleCandidate(candidate idleCandidate) {
	ctx, cancel := context.WithTimeout(l.service.laneCtx, laneCleanupTimeout)
	defer cancel()
	id, err := l.service.reserveObservedCommandID(ctx, l.info.WS)
	if err != nil {
		l.service.logger.Error("terminal journal: reserve idle command id", "terminal_id", l.info.ID, "error", err)
		l.setAuditBlocked()
		return
	}
	finishedAt := l.service.now()
	duration := finishedAt.Sub(candidate.startedAt).Milliseconds()
	row := terminalpkg.CommandRow{
		ID: id, TerminalID: terminalIDPointer(l.info.ID), ProfileID: l.info.ProfileID,
		Actor: candidate.actor, Command: candidate.command, Cwd: l.info.Cwd,
		StartedAt: candidate.startedAt, DurationMs: &duration, ExitCause: "unknown",
		DetectedBy: "idle", Approval: approvalForActor(candidate.actor),
		OutputTail: l.takeOutputTail(),
	}
	l.emitEvent(terminalpkg.Event{
		Kind: terminalpkg.EventKindCommandStarted, WorkspaceID: l.info.WS, ProfileID: l.info.ProfileID,
		TerminalID: l.info.ID, Actor: candidate.actor, At: candidate.startedAt,
		Detail: &terminalpkg.EventDetail{
			CommandID: id, Command: candidate.command, Cwd: l.info.Cwd, DetectedBy: "idle",
		},
	})
	l.finishCommand(row, finishedAt)
}

func inputSubmissionCount(input []byte) int {
	count := 0
	previousCR := false
	for _, value := range input {
		if value == '\n' && previousCR {
			previousCR = false
			continue
		}
		if value == '\r' || value == '\n' {
			count++
		}
		previousCR = value == '\r'
	}
	return count
}
