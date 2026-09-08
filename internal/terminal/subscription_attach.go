package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

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
		mode = terminalAccessRead
	}
	if mode != terminalAccessRead && mode != terminalAccessWrite {
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
		Rows: rows, Mode: info.Mode, Preamble: string(replay.Preamble),
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
