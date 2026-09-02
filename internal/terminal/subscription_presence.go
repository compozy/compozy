package terminal

import (
	"encoding/json"
	"errors"
	"fmt"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

func (s *session) emitSubscriberLimit(actor Actor, current, maximum int) {
	info := s.Info()
	s.manager.events.Notify(s.ctx, Event{
		Kind: EventKindLimitRejected, WorkspaceID: info.WS, ProfileID: info.ProfileID,
		TerminalID: info.ID, Actor: actor,
		Detail: &EventDetail{Limit: "subscribers", Current: current, Max: maximum}, At: s.manager.now(),
	})
}

func (s *session) removeSubscriber(subscriber *subscription) {
	s.mu.Lock()
	previousCols, previousRows := s.cols, s.rows
	delete(s.subscribers, subscriber.id)
	s.info.Viewers = len(s.subscribers)
	s.lastActivity = s.manager.now()
	cols, rows := s.resizeVoteLocked()
	changed := cols != s.cols || rows != s.rows
	s.cols, s.rows = cols, rows
	s.mu.Unlock()
	s.flow.Remove(subscriber.queue)
	s.broadcastPresence()
	if subscriber.leaseToken != 0 {
		s.lease.detachWriter(subscriber.leaseToken)
	}
	if changed {
		if err := s.applyResize(cols, rows, previousCols, previousRows); err != nil {
			s.mu.Lock()
			if s.cols == cols && s.rows == rows {
				s.cols, s.rows = previousCols, previousRows
			}
			s.mu.Unlock()
			s.manager.logger.Warn(
				"terminal: resize after subscriber departure",
				"terminal_id",
				s.Info().ID,
				"error",
				err,
			)
		}
	}
}

func (s *session) broadcastPresence() {
	s.mu.RLock()
	payload, err := json.Marshal(presenceFramePayload{Viewers: len(s.subscribers)})
	subscribers := make([]*subscription, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.RUnlock()
	if err != nil {
		s.manager.logger.Warn("terminal: encode presence frame", "terminal_id", s.Info().ID, "error", err)
		return
	}
	for _, subscriber := range subscribers {
		subscriber.deliver(Frame{Op: terminalwire.ServerOpPresence, Payload: payload}, 0)
	}
}

func (s *session) resizeVoteLocked() (uint16, uint16) {
	cols, rows := s.cols, s.rows
	first := true
	for _, subscriber := range s.subscribers {
		if subscriber.mode != terminalAccessWrite || subscriber.cols == 0 || subscriber.rows == 0 {
			continue
		}
		if first {
			cols, rows, first = subscriber.cols, subscriber.rows, false
			continue
		}
		cols, rows = min(cols, subscriber.cols), min(rows, subscriber.rows)
	}
	return cols, rows
}

func (s *session) applyResize(cols, rows, previousCols, previousRows uint16) error {
	if err := s.proc.Resize(cols, rows); err != nil {
		return fmt.Errorf("terminal: resize process: %w", err)
	}
	if err := s.vt.Resize(s.ctx, int(cols), int(rows)); err != nil {
		rollbackErr := s.proc.Resize(previousCols, previousRows)
		return errors.Join(
			fmt.Errorf("terminal: resize emulator: %w", err),
			wrapResizeRollbackError(rollbackErr),
		)
	}
	payload, err := json.Marshal(map[string]uint16{"cols": cols, "rows": rows})
	if err != nil {
		return fmt.Errorf("terminal: encode RESIZED frame: %w", err)
	}
	s.mu.RLock()
	subscribers := make([]*subscription, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.RUnlock()
	for _, subscriber := range subscribers {
		subscriber.deliver(Frame{Op: terminalwire.ServerOpResized, Payload: payload}, 0)
	}
	return nil
}

func wrapResizeRollbackError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("terminal: roll back process resize: %w", err)
}

func (s *session) ringNext() uint64 {
	_, next := s.ring.Bounds()
	return next
}
