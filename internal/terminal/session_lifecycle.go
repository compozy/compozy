package terminal

import (
	"context"
	"errors"
	"fmt"
	"time"

	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
	"github.com/compozy/compozy/internal/toolruntime"
)

const outputDrainGrace = 200 * time.Millisecond

func (s *session) waitProcess(outputDone <-chan struct{}) {
	ptyExit, waitErr := s.proc.Wait(s.ctx)
	s.flow.ResumeProducer()
	select {
	case <-outputDone:
	case <-time.After(outputDrainGrace):
		if err := s.proc.Close(); err != nil {
			s.manager.logger.Debug("terminal: close output after wait", "terminal_id", s.info.ID, "error", err)
		}
		<-outputDone
	}
	exit := Exit{Cause: ptyExit.Cause, Code: ptyExit.Code, Signal: ptyExit.Signal, At: s.manager.now()}
	if waitErr != nil {
		s.manager.logger.Warn("terminal: wait process", "terminal_id", s.info.ID, "error", waitErr)
		if exit.Cause == "" {
			exit.Cause = "unknown"
		}
	}
	s.finalize(exit)
}

func (s *session) finalize(exit Exit) {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.exit = cloneExit(&exit)
		s.info.Exit = cloneExit(&exit)
		s.info.State = "exited"
		s.bumpRevisionLocked()
		subscribers := make([]*subscription, 0, len(s.subscribers))
		for _, subscriber := range s.subscribers {
			subscribers = append(subscribers, subscriber)
		}
		s.subscribers = make(map[uint64]*subscription)
		reason := s.closeReason
		if reason == "" {
			reason = exit.Cause
		}
		actor := s.closeActor
		if actor.ProfileID == "" {
			actor = s.origin
		}
		if actor.ProfileID == "" {
			actor = Actor{Kind: ActorKindSystem, ID: "terminal-process", ProfileID: s.info.ProfileID}
		}
		emitClosed := !s.closedEmitted
		s.closedEmitted = true
		info := s.infoSnapshotLocked()
		s.mu.Unlock()
		s.recordingMu.Lock()
		s.recordingSealed = true
		s.recordingMu.Unlock()
		s.recordingWG.Wait()
		recordingCtx, cancelRecording := boundedCleanupContext(s.ctx, recordingPersistenceTimeout)
		_, recordingErr := s.stopRecordingForFinalization(recordingCtx, actor, "terminal_closed")
		cancelRecording()
		if recordingErr != nil &&
			!isRecordingNotActive(recordingErr) {
			s.manager.logger.Warn("terminal: stop recording on exit", "terminal_id", info.ID, "error", recordingErr)
		}
		journalCtx, cancelJournal := boundedCleanupContext(s.ctx, defaultJournalShutdownTimeout)
		s.persistCommandBeforeExit(journalCtx, info)
		if err := s.manager.closeJournalTerminal(journalCtx, s); err != nil {
			s.finalizationMu.Lock()
			s.journalClosePending = true
			s.finalizationMu.Unlock()
			s.audit.SetBlocked(true)
			s.manager.logger.Error("terminal: close journal lane", "terminal_id", info.ID, "error", err)
		}
		cancelJournal()
		for _, subscriber := range subscribers {
			subscriber.finish(exit)
		}
		if err := s.vt.Close(); err != nil {
			s.manager.logger.Debug("terminal: close emulator", "terminal_id", s.info.ID, "error", err)
		}
		if err := s.proc.Close(); err != nil {
			s.manager.logger.Debug("terminal: close process resources", "terminal_id", s.info.ID, "error", err)
		}
		if s.processRecord != nil {
			completion := toolruntime.ProcessCompletion{ExitCode: exit.Code}
			if exit.Cause == "unknown" {
				completion.Err = errors.New("terminal process exit cause unknown")
			}
			completeCtx, cancelComplete := boundedCleanupContext(s.ctx, processCleanupTimeout)
			if err := s.processRecord.Complete(completeCtx, completion); err != nil {
				s.manager.logger.Warn("terminal: complete process record", "terminal_id", s.info.ID, "error", err)
			}
			cancelComplete()
		}
		if emitClosed {
			s.manager.events.Notify(s.ctx, Event{
				Kind: EventKindClosed, WorkspaceID: info.WS, ProfileID: info.ProfileID,
				ProfileName: s.profileName,
				TerminalID:  info.ID, Actor: actor, Info: &info, Exit: cloneExit(&exit), Reason: reason,
				At: s.manager.now(),
			})
		}
		close(s.done)
		s.cancel()
	})
}

func (s *session) close(ctx context.Context, signal Signal, reason string, actor Actor) (*Exit, error) {
	if err := requestContextError(ctx, "close"); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.exit != nil {
		exit := cloneExit(s.exit)
		s.mu.Unlock()
		// Close is idempotent: an already-exited terminal reports its recorded
		// exit; only a failed journal repair surfaces as an error.
		if err := s.awaitRemoval(ctx); err != nil {
			return exit, err
		}
		return exit, nil
	}
	if s.closeReason == "" {
		s.closeReason = reason
		s.closeActor = actor
	}
	s.mu.Unlock()
	if signal == "" {
		signal = SignalHUP
	}
	if s.processRecord != nil {
		if err := s.processRecord.Checkpoint(ctx, toolruntime.ProcessCheckpoint{
			State: toolruntime.ProcessStateInterrupting, Error: reason,
		}); err != nil {
			s.manager.logger.Warn("terminal: checkpoint interrupt", "terminal_id", s.info.ID, "error", err)
		}
	}
	if err := s.proc.Kill(terminalSignal(signal)); err != nil {
		return nil, fmt.Errorf("terminal: close %q: %w", s.info.ID, err)
	}
	select {
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case <-s.done:
		if err := s.awaitRemoval(ctx); err != nil {
			return nil, err
		}
		s.mu.RLock()
		defer s.mu.RUnlock()
		return cloneExit(s.exit), nil
	}
}

func terminalSignal(signal Signal) terminalpty.Signal { return terminalpty.Signal(signal) }

func cloneExit(exit *Exit) *Exit {
	if exit == nil {
		return nil
	}
	copyOfExit := *exit
	return &copyOfExit
}
