package terminal

import (
	"encoding/json"
	"fmt"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

func (s *subscription) Frames() <-chan Frame { return s.queue.Frames() }

func (s *subscription) Err() error {
	if s.queue.CloseReason() != "slow_consumer" {
		return nil
	}
	return &Error{
		Code: ErrorCodeSlowConsumer, Message: "terminal subscriber could not keep up with output",
		Err: ErrSlowConsumer,
	}
}

func (s *subscription) Ack(bytes int) { s.queue.Ack(bytes) }

func (s *subscription) Resize(cols, rows uint16) error {
	if s.mode != terminalAccessWrite {
		return fmt.Errorf("terminal resize requires a write attachment: %w", ErrWriteAttachmentRequired)
	}
	if err := s.session.lease.authorize(s.actor); err != nil {
		return err
	}
	if cols == 0 || rows == 0 {
		return nil
	}
	cols, rows, _ = terminalwire.ClampDimensions(cols, rows)
	s.session.mu.Lock()
	previousVoteCols, previousVoteRows := s.cols, s.rows
	previousCols, previousRows := s.session.cols, s.session.rows
	s.cols, s.rows = cols, rows
	nextCols, nextRows := s.session.resizeVoteLocked()
	changed := nextCols != previousCols || nextRows != previousRows
	s.session.cols, s.session.rows = nextCols, nextRows
	s.session.mu.Unlock()
	if !changed {
		return nil
	}
	if err := s.session.applyResize(nextCols, nextRows, previousCols, previousRows); err != nil {
		s.session.mu.Lock()
		s.cols, s.rows = previousVoteCols, previousVoteRows
		if s.session.cols == nextCols && s.session.rows == nextRows {
			s.session.cols, s.session.rows = previousCols, previousRows
		}
		s.session.mu.Unlock()
		return err
	}
	return nil
}

func (s *subscription) Close() error {
	s.removeOnce.Do(func() { s.session.removeSubscriber(s) })
	s.queue.Close()
	return nil
}

func (s *subscription) deliver(frame Frame, end uint64) {
	s.queue.Enqueue(frame, end)
}

func (s *subscription) finish(exit Exit) {
	payload, err := json.Marshal(exitFramePayload{
		Cause: exit.Cause, ExitCode: exit.Code, Signal: exit.Signal,
		Seq: terminalSequenceString(s.session.ringNext()),
	})
	if err != nil {
		if closeErr := s.Close(); closeErr != nil {
			s.session.manager.logger.Warn("terminal: close subscription after exit encoding failure", "error", closeErr)
		}
		return
	}
	s.finishOnce.Do(func() {
		s.queue.Enqueue(Frame{Op: terminalwire.ServerOpExit, Payload: payload}, 0)
		s.removeOnce.Do(func() { s.session.removeSubscriber(s) })
		s.queue.Finish()
	})
}

func (s *subscription) evict(reason string) {
	s.emitFlowTransition(reason)
	if err := s.Close(); err != nil {
		s.session.manager.logger.Warn("terminal: close evicted subscription", "error", err)
	}
}

func (s *subscription) demoted(reason string) {
	s.emitFlowTransition(reason)
}

func (s *subscription) emitFlowTransition(reason string) {
	info := s.session.Info()
	s.session.manager.events.Notify(s.session.ctx, Event{
		Kind: EventKindSubscriberEvicted, WorkspaceID: info.WS, ProfileID: info.ProfileID,
		ProfileName: s.session.profileName,
		TerminalID:  info.ID, Actor: s.actor,
		Reason: reason, Detail: &EventDetail{Flow: string(s.queue.Flow())}, At: s.session.manager.now(),
	})
}
