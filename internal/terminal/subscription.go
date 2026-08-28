package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

type subscription struct {
	session    *session
	id         uint64
	mode       string
	actor      Actor
	queue      *terminalwire.Queue
	leaseToken uint64
	cols       uint16
	rows       uint16
	removeOnce sync.Once
	finishOnce sync.Once
}

var _ Subscription = (*subscription)(nil)

type attachedFramePayload struct {
	Seq       string     `json:"seq"`
	Truncated bool       `json:"truncated"`
	Cols      uint16     `json:"cols"`
	Rows      uint16     `json:"rows"`
	Lease     LeaseState `json:"lease"`
	Mode      Mode       `json:"mode"`
	Preamble  string     `json:"preamble,omitempty"`
}

type exitFramePayload struct {
	Cause    string  `json:"cause"`
	ExitCode *int    `json:"exit_code"`
	Signal   *string `json:"signal"`
	Seq      string  `json:"seq"`
}

type presenceFramePayload struct {
	Viewers int `json:"viewers"`
}

type redactedInputFramePayload struct {
	Seq        string `json:"seq"`
	Characters int    `json:"characters"`
}

func (s *session) Attach(ctx context.Context, options AttachOptions) (Subscription, error) {
	if err := requestContextError(ctx, "attach"); err != nil {
		return nil, err
	}
	if err := s.authorizeProfile(options.Actor); err != nil {
		return nil, err
	}
	if s.Info().Mode != ModePTY {
		mode := s.Info().Mode
		return nil, &Error{
			Code:    ErrorCodeNotInteractive,
			Message: errorMessageNotInteractive,
			Mode:    mode,
			Err:     ErrNotInteractive,
		}
	}
	if err := s.runningGate(); err != nil {
		return nil, err
	}
	mode, flow, err := normalizeAttachOptions(options)
	if err != nil {
		return nil, err
	}
	if mode == terminalAccessWrite {
		if err := s.lease.authorize(options.Actor); err != nil {
			return nil, err
		}
	}
	settings := s.settings(ctx)
	if err := requestContextError(ctx, "attach"); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.reaping {
		s.mu.Unlock()
		return nil, &Error{Code: ErrorCodeExpired, Message: errorMessageExpired, Err: ErrExpired}
	}
	if s.exit != nil {
		s.mu.Unlock()
		return nil, &Error{Code: ErrorCodeExited, Message: errorMessageExited, Err: ErrExited}
	}
	if len(s.subscribers) >= settings.MaxSubscribers {
		current := len(s.subscribers)
		s.mu.Unlock()
		s.emitSubscriberLimit(options.Actor, current, settings.MaxSubscribers)
		return nil, &Error{
			Code:    ErrorCodeSubscriberLimitReached,
			Message: "terminal subscriber limit reached",
			Current: current,
			Max:     settings.MaxSubscribers,
			Err:     ErrSubscriberLimit,
		}
	}
	s.nextSubID++
	subscriber := &subscription{session: s, id: s.nextSubID, mode: mode, actor: options.Actor}
	subscriber.queue = terminalwire.NewQueue(terminalwire.QueueOptions{
		Flow: terminalwire.Flow(flow), Now: s.manager.now,
		Demoted: subscriber.demoted, Evicted: subscriber.evict,
	})
	s.flow.Add(subscriber.queue)
	if mode == terminalAccessWrite {
		subscriber.leaseToken = s.lease.attachWriter(options.Actor)
	}
	s.subscribers[subscriber.id] = subscriber
	s.info.Viewers = len(s.subscribers)
	s.lastActivity = s.manager.now()
	info := s.infoSnapshotLocked()
	cols, rows := s.cols, s.rows
	if err := subscriber.enqueueInitialFrames(options, info, cols, rows); err != nil {
		s.mu.Unlock()
		closeErr := subscriber.Close()
		return nil, errors.Join(err, closeErr)
	}
	s.mu.Unlock()
	return s.finishAttach(ctx, options, subscriber, mode)
}

func (s *session) finishAttach(
	ctx context.Context,
	options AttachOptions,
	subscriber *subscription,
	mode string,
) (Subscription, error) {
	if err := requestContextError(ctx, "attach"); err != nil {
		return nil, errors.Join(err, subscriber.Close())
	}
	s.broadcastPresence()
	if options.Cols > 0 && options.Rows > 0 && mode == terminalAccessWrite {
		if err := requestContextError(ctx, "attach"); err != nil {
			return nil, errors.Join(err, subscriber.Close())
		}
		if err := subscriber.Resize(options.Cols, options.Rows); err != nil {
			closeErr := subscriber.Close()
			return nil, errors.Join(err, closeErr)
		}
	}
	return subscriber, nil
}

func normalizeAttachOptions(options AttachOptions) (string, string, error) {
	mode := options.Mode
	if mode == "" {
		mode = "read"
	}
	if mode != "read" && mode != terminalAccessWrite {
		return "", "", fmt.Errorf("terminal attach mode must be read or write: %w", ErrUnsupported)
	}
	flow := options.Flow
	if flow == "" && mode == terminalAccessWrite {
		flow = string(terminalwire.FlowAck)
	}
	if flow == "" {
		flow = string(terminalwire.FlowDrop)
	}
	if flow != string(terminalwire.FlowAck) && flow != string(terminalwire.FlowDrop) {
		return "", "", fmt.Errorf("terminal flow must be ack or drop: %w", ErrUnsupported)
	}
	return mode, flow, nil
}

func (s *subscription) enqueueInitialFrames(options AttachOptions, info Info, cols, rows uint16) error {
	replay := s.session.ring.ReplayFrom(options.AfterSeq)
	attached, err := json.Marshal(attachedFramePayload{
		Seq: terminalSequenceString(replay.Seq), Truncated: replay.Truncated, Cols: cols,
		Rows: rows, Lease: info.Lease, Mode: info.Mode, Preamble: string(replay.Preamble),
	})
	if err != nil {
		return fmt.Errorf("terminal: encode ATTACHED frame: %w", err)
	}
	s.queue.Enqueue(Frame{Op: terminalwire.ServerOpAttached, Seq: replay.Seq, Payload: attached}, replay.Seq)
	for _, segment := range replay.Segments {
		if segment.Segment.Kind == OutputSegmentBytes {
			s.queue.Enqueue(Frame{
				Op: terminalwire.ServerOpOutput, Seq: segment.Seq, Payload: []byte(segment.Segment.Text),
			}, segment.End)
			continue
		}
		payload, encodeErr := json.Marshal(redactedInputFramePayload{
			Seq: terminalSequenceString(segment.Seq), Characters: segment.Segment.Characters,
		})
		if encodeErr != nil {
			return fmt.Errorf("terminal: encode redacted input replay frame: %w", encodeErr)
		}
		s.queue.Enqueue(Frame{
			Op: terminalwire.ServerOpRedactedInput, Seq: segment.Seq, Payload: payload,
		}, segment.End)
	}
	return nil
}

func terminalSequenceString(sequence uint64) string {
	return strconv.FormatUint(sequence, 10)
}

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
