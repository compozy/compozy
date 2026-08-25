package journal

import (
	"strings"
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

// ObserveInput records accepted human input as an approximate fallback candidate.
func (s *Service) ObserveInput(info terminalpkg.Info, actor terminalpkg.Actor, input []byte) {
	reservation, admitted := s.ReserveInput(info, input)
	if !admitted {
		return
	}
	s.CommitInput(info, actor, input, reservation)
}

// ReserveInput reserves bounded journal capacity before input reaches the PTY.
func (s *Service) ReserveInput(info terminalpkg.Info, input []byte) (int, bool) {
	lane := s.lane(info)
	if lane == nil || len(input) == 0 {
		return 0, true
	}
	count := inputSubmissionCount(input)
	return count, lane.reserve(count)
}

// CommitInput turns an accepted reservation into idle-fallback candidates.
func (s *Service) CommitInput(
	info terminalpkg.Info,
	actor terminalpkg.Actor,
	input []byte,
	reservation int,
) {
	lane := s.lane(info)
	if lane == nil {
		return
	}
	candidates := lane.observeInput(input, actor)
	if candidates < reservation {
		lane.release(reservation - candidates)
	}
}

// ReleaseInput returns capacity when PTY delivery fails after reservation.
func (s *Service) ReleaseInput(info terminalpkg.Info, reservation int) {
	if lane := s.lane(info); lane != nil {
		lane.release(reservation)
	}
}

// ObserveOutput postpones approximate completion while the terminal is still producing bytes.
func (s *Service) ObserveOutput(info terminalpkg.Info) {
	lane := s.lane(info)
	if lane != nil {
		lane.observeOutput()
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

func (l *terminalLane) observeOutput() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || len(l.idle) == 0 {
		return
	}
	l.resetIdleTimerLocked()
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
	id, err := randomCommandID()
	if err != nil {
		l.service.logger.Error("terminal journal: generate idle command id", "terminal_id", l.info.ID, "error", err)
		l.setAuditBlocked(true)
		return
	}
	finishedAt := l.service.now()
	duration := finishedAt.Sub(candidate.startedAt).Milliseconds()
	row := terminalpkg.CommandRow{
		ID: id, TerminalID: terminalIDPointer(l.info.ID), ProfileID: l.info.ProfileID,
		Actor: candidate.actor, Command: candidate.command, Cwd: l.info.Cwd,
		StartedAt: candidate.startedAt, DurationMs: &duration, ExitCause: "unknown",
		DetectedBy: "idle", Approval: approvalForActor(candidate.actor),
	}
	l.emitEvent(terminalpkg.TerminalEvent{
		Kind: terminalpkg.EventKindCommandStarted, WorkspaceID: l.info.WS, ProfileID: l.info.ProfileID,
		TerminalID: l.info.ID, Actor: candidate.actor, At: candidate.startedAt,
		Detail: terminalpkg.EventDetail{
			CommandID: id, Command: candidate.command, Cwd: l.info.Cwd, DetectedBy: "idle",
		},
	})
	l.enqueue(row)
	l.emitEvent(terminalpkg.TerminalEvent{
		Kind: terminalpkg.EventKindCommandFinished, WorkspaceID: l.info.WS, ProfileID: l.info.ProfileID,
		TerminalID: l.info.ID, Actor: candidate.actor, At: finishedAt,
		Detail: terminalpkg.EventDetail{
			CommandID: id, ExitCause: "unknown", DurationMS: duration,
			DetectedBy: "idle", Approval: row.Approval,
		},
	})
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
