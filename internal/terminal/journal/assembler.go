package journal

import (
	"context"
	"strings"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

const commandDetectionMarker = "marker"

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
	emit func(terminalpkg.Event),
) {
	if s == nil || info.ID == "" {
		return
	}
	s.ensureLane(info, setBlocked, emit)
}

// CloseTerminal flushes and removes the writer lane owned by one terminal.
func (s *Service) CloseTerminal(ctx context.Context, info terminalpkg.Info) error {
	if s == nil {
		return nil
	}
	key := terminalLaneKey(info)
	s.mu.Lock()
	lane := s.lanes[key]
	s.mu.Unlock()
	if lane == nil {
		s.removeTerminalLiveTails(info.WS, info.ID)
		return nil
	}
	err := lane.close(ctx)
	s.removeStoppedLane(key, lane)
	if err == nil {
		s.removeTerminalLiveTails(info.WS, info.ID)
	}
	return err
}

func (s *Service) ensureLane(
	info terminalpkg.Info,
	setBlocked func(bool),
	emit func(terminalpkg.Event),
) (*terminalLane, bool) {
	key := terminalLaneKey(info)
	s.mu.Lock()
	defer s.mu.Unlock()
	if lane := s.lanes[key]; lane != nil {
		return lane, false
	}
	lane := newTerminalLane(s.laneCtx, s, info, setBlocked, emit)
	s.lanes[key] = lane
	return lane, true
}

// ConsumeMarkerFacts assembles authenticated shell facts without parsing bytes again.
func (s *Service) ConsumeMarkerFacts(
	ctx context.Context,
	info terminalpkg.Info,
	facts []terminalpkg.MarkerFacts,
) error {
	if s == nil || len(facts) == 0 {
		return nil
	}
	lane := s.lane(info)
	if lane == nil {
		return &terminalpkg.Error{
			Code: terminalpkg.ErrorCodeJournalUnavailable, Message: "terminal journal lane is unavailable",
			Err: terminalpkg.ErrJournalUnavailable,
		}
	}
	for _, fact := range facts {
		switch fact.Kind {
		case "S":
			lane.cancelIdleCandidate()
			id, err := s.reserveObservedCommandID(ctx, info.WS)
			if err != nil {
				s.logger.Error("terminal journal: reserve command id", "terminal_id", info.ID, "error", err)
				lane.setAuditBlocked()
				continue
			}
			if !lane.setAssembly(commandAssembly{
				id: id, command: scrubCommand(fact.Command), cwd: fact.Cwd, startedAt: s.now(),
				actor: lane.actor(), detectedBy: commandDetectionMarker,
			}) {
				s.ReleaseCommandID(info.WS, id)
				continue
			}
			lane.emitEvent(terminalpkg.Event{
				Kind: terminalpkg.EventKindCommandStarted, WorkspaceID: info.WS, ProfileID: info.ProfileID,
				TerminalID: info.ID, Actor: lane.actor(), At: s.now(),
				Detail: &terminalpkg.EventDetail{
					CommandID:  id,
					Command:    scrubCommand(fact.Command),
					Cwd:        fact.Cwd,
					DetectedBy: commandDetectionMarker,
				},
			})
		case "F":
			lane.finishAssembly(fact.Exit, s.now())
		}
	}
	return nil
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
		detectedBy = commandDetectionMarker
	}
	row := terminalpkg.CommandRow{
		ID: assembly.id, TerminalID: terminalIDPointer(l.info.ID), ProfileID: l.info.ProfileID,
		Actor: actor, Command: assembly.command, Cwd: assembly.cwd,
		StartedAt: assembly.startedAt, DurationMs: &duration, ExitCode: exitCode,
		ExitCause: exitCause, DetectedBy: detectedBy, Approval: approvalForActor(actor),
		OutputTail: l.takeOutputTail(),
	}
	l.finishCommand(row, finishedAt)
}

func (l *terminalLane) finishCommand(row terminalpkg.CommandRow, finishedAt time.Time) {
	duration := int64(0)
	if row.DurationMs != nil {
		duration = *row.DurationMs
	}
	result := l.enqueue(row)
	select {
	case err := <-result:
		if err != nil {
			l.service.ReleaseCommandID(l.info.WS, row.ID)
			return
		}
	default:
	}
	l.emitEvent(terminalpkg.Event{
		Kind: terminalpkg.EventKindCommandFinished, WorkspaceID: l.info.WS, ProfileID: l.info.ProfileID,
		TerminalID: l.info.ID, Actor: row.Actor, At: finishedAt,
		Detail: &terminalpkg.EventDetail{
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
