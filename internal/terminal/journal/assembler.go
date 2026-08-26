package journal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type commandAssembly struct {
	id         string
	command    string
	cwd        string
	startedAt  time.Time
	actor      terminalpkg.Actor
	detectedBy string
}

// RegisterTerminal creates the bounded writer lane before terminal bytes flow.
func (s *Service) RegisterTerminal(
	info terminalpkg.Info,
	setBlocked func(bool),
	emit func(terminalpkg.TerminalEvent),
) {
	if s == nil || info.ID == "" {
		return
	}
	s.ensureLane(info, setBlocked, emit)
}

func (s *Service) ensureLane(
	info terminalpkg.Info,
	setBlocked func(bool),
	emit func(terminalpkg.TerminalEvent),
) (*terminalLane, bool) {
	key := terminalLaneKey(info)
	s.mu.Lock()
	defer s.mu.Unlock()
	if lane := s.lanes[key]; lane != nil {
		return lane, false
	}
	lane := newTerminalLane(s, info, setBlocked, emit)
	s.lanes[key] = lane
	return lane, true
}

// ConsumeMarkerFacts assembles authenticated shell facts without parsing bytes again.
func (s *Service) ConsumeMarkerFacts(
	_ context.Context,
	info terminalpkg.Info,
	facts []terminalpkg.MarkerFacts,
) {
	if s == nil || len(facts) == 0 {
		return
	}
	lane := s.lane(info)
	if lane == nil {
		return
	}
	for _, fact := range facts {
		switch fact.Kind {
		case "S":
			lane.cancelIdleCandidate()
			id, err := randomCommandID()
			if err != nil {
				s.logger.Error("terminal journal: generate command id", "terminal_id", info.ID, "error", err)
				lane.setAuditBlocked(true)
				continue
			}
			lane.setAssembly(commandAssembly{
				id: id, command: scrubCommand(fact.Command), cwd: fact.Cwd, startedAt: s.now(),
				actor: lane.actor(), detectedBy: "marker",
			})
			lane.emitEvent(terminalpkg.TerminalEvent{
				Kind: terminalpkg.EventKindCommandStarted, WorkspaceID: info.WS, ProfileID: info.ProfileID,
				TerminalID: info.ID, Actor: lane.actor(), At: s.now(),
				Detail: terminalpkg.EventDetail{
					CommandID: id, Command: scrubCommand(fact.Command), Cwd: fact.Cwd, DetectedBy: "marker",
				},
			})
		case "F":
			lane.finishAssembly(fact.Exit, s.now())
		}
	}
}

func (l *terminalLane) finishAssembly(exitCode *int, finishedAt time.Time) {
	assembly, ok := l.takeAssembly()
	if !ok {
		return
	}
	duration := finishedAt.Sub(assembly.startedAt).Milliseconds()
	exitCause := "unknown"
	if exitCode != nil {
		exitCause = "exited"
	}
	actor := assembly.actor
	if actor.ID == "" {
		actor = l.actor()
	}
	detectedBy := assembly.detectedBy
	if detectedBy == "" {
		detectedBy = "marker"
	}
	row := terminalpkg.CommandRow{
		ID: assembly.id, TerminalID: terminalIDPointer(l.info.ID), ProfileID: l.info.ProfileID,
		Actor: actor, Command: assembly.command, Cwd: assembly.cwd,
		StartedAt: assembly.startedAt, DurationMs: &duration, ExitCode: exitCode,
		ExitCause: exitCause, DetectedBy: detectedBy, Approval: approvalForActor(actor),
	}
	l.finishCommand(row, finishedAt)
}

func (l *terminalLane) finishCommand(row terminalpkg.CommandRow, finishedAt time.Time) {
	duration := int64(0)
	if row.DurationMs != nil {
		duration = *row.DurationMs
	}
	l.enqueue(row)
	l.emitEvent(terminalpkg.TerminalEvent{
		Kind: terminalpkg.EventKindCommandFinished, WorkspaceID: l.info.WS, ProfileID: l.info.ProfileID,
		TerminalID: l.info.ID, Actor: row.Actor, At: finishedAt,
		Detail: terminalpkg.EventDetail{
			CommandID: row.ID, ExitCode: row.ExitCode, ExitCause: row.ExitCause,
			DurationMS: duration, DetectedBy: row.DetectedBy, Approval: row.Approval,
		},
	})
}

func (s *Service) lane(info terminalpkg.Info) *terminalLane {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lanes[terminalLaneKey(info)]
}

func terminalLaneKey(info terminalpkg.Info) string {
	return strings.Join([]string{info.WS, info.ProfileID, string(info.ID)}, "\x00")
}

func terminalIDPointer(id terminalpkg.ID) *terminalpkg.ID {
	result := id
	return &result
}

func randomCommandID() (string, error) {
	bytes := make([]byte, 3)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", fmt.Errorf("terminal journal: command id: %w", err)
	}
	return "cmd-" + hex.EncodeToString(bytes), nil
}
