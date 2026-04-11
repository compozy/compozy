package executor

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const workflowEventStreamWaitTimeout = 5 * time.Second

type workflowEventStreamMode uint8

const (
	workflowEventStreamDisabled workflowEventStreamMode = iota
	workflowEventStreamLean
	workflowEventStreamRaw
)

type workflowJSONLEvent struct {
	Type    string          `json:"type"`
	RunID   string          `json:"run_id"`
	Seq     uint64          `json:"seq"`
	Time    time.Time       `json:"time"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type workflowEventStreamer struct {
	done         chan struct{}
	terminalSeen chan struct{}
	unsubscribe  func()
	errCh        chan error
}

func startWorkflowEventStreamer(
	bus *events.Bus[events.Event],
	cfg *config,
	dst io.Writer,
) *workflowEventStreamer {
	mode := workflowEventStreamModeFor(cfg)
	if mode == workflowEventStreamDisabled || bus == nil || dst == nil {
		return nil
	}

	_, updates, unsubscribe := bus.Subscribe()
	streamer := &workflowEventStreamer{
		done:         make(chan struct{}),
		terminalSeen: make(chan struct{}),
		unsubscribe:  unsubscribe,
		errCh:        make(chan error, 1),
	}

	go func() {
		defer close(streamer.done)

		encoder := json.NewEncoder(dst)
		terminalSeen := false
		for ev := range updates {
			if !shouldStreamWorkflowEvent(mode, ev) {
				continue
			}
			if err := encodeWorkflowEvent(encoder, mode, ev); err != nil {
				streamer.errCh <- err
				return
			}
			if isTerminalWorkflowEvent(ev.Kind) && !terminalSeen {
				terminalSeen = true
				close(streamer.terminalSeen)
			}
		}
		if !terminalSeen {
			close(streamer.terminalSeen)
		}
	}()

	return streamer
}

func (s *workflowEventStreamer) FinalizeAndStop() error {
	if s == nil {
		return nil
	}

	defer func() {
		if s.unsubscribe != nil {
			s.unsubscribe()
		}
		<-s.done
	}()

	select {
	case err := <-s.errCh:
		return err
	case <-s.terminalSeen:
		select {
		case err := <-s.errCh:
			return err
		default:
			return nil
		}
	case <-time.After(workflowEventStreamWaitTimeout):
		select {
		case err := <-s.errCh:
			return err
		default:
			return nil
		}
	}
}

func workflowEventStreamModeFor(cfg *config) workflowEventStreamMode {
	if cfg == nil {
		return workflowEventStreamDisabled
	}
	switch cfg.ResolvedOutputFormat() {
	case model.OutputFormatJSON:
		return workflowEventStreamLean
	case model.OutputFormatRawJSON:
		return workflowEventStreamRaw
	default:
		return workflowEventStreamDisabled
	}
}

func shouldStreamWorkflowEvent(mode workflowEventStreamMode, ev events.Event) bool {
	switch mode {
	case workflowEventStreamRaw:
		return true
	case workflowEventStreamLean:
		return shouldEmitLeanWorkflowEvent(ev)
	default:
		return false
	}
}

func shouldEmitLeanWorkflowEvent(ev events.Event) bool {
	switch ev.Kind {
	case events.EventKindRunStarted,
		events.EventKindRunCompleted,
		events.EventKindRunFailed,
		events.EventKindRunCancelled,
		events.EventKindJobStarted,
		events.EventKindJobRetryScheduled,
		events.EventKindJobCompleted,
		events.EventKindJobFailed,
		events.EventKindJobCancelled,
		events.EventKindSessionStarted,
		events.EventKindSessionCompleted,
		events.EventKindSessionFailed:
		return true
	case events.EventKindSessionUpdate:
		return shouldEmitLeanWorkflowSessionUpdate(ev.Payload)
	default:
		return false
	}
}

func shouldEmitLeanWorkflowSessionUpdate(raw json.RawMessage) bool {
	var payload kinds.SessionUpdatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	switch payload.Update.Kind {
	case kinds.UpdateKindUserMessageChunk,
		kinds.UpdateKindAgentMessageChunk,
		kinds.UpdateKindToolCallStarted,
		kinds.UpdateKindToolCallUpdated:
		return true
	case kinds.UpdateKindUnknown:
		return payload.Update.Status == kinds.StatusCompleted || payload.Update.Status == kinds.StatusFailed
	default:
		return false
	}
}

func encodeWorkflowEvent(encoder *json.Encoder, mode workflowEventStreamMode, ev events.Event) error {
	switch mode {
	case workflowEventStreamRaw:
		if err := encoder.Encode(ev); err != nil {
			return fmt.Errorf("encode raw workflow event: %w", err)
		}
		return nil
	case workflowEventStreamLean:
		payload := workflowJSONLEvent{
			Type:    string(ev.Kind),
			RunID:   ev.RunID,
			Seq:     ev.Seq,
			Time:    ev.Timestamp,
			Payload: ev.Payload,
		}
		if err := encoder.Encode(payload); err != nil {
			return fmt.Errorf("encode workflow jsonl event: %w", err)
		}
		return nil
	default:
		return nil
	}
}

func isTerminalWorkflowEvent(kind events.EventKind) bool {
	switch kind {
	case events.EventKindRunCompleted, events.EventKindRunFailed, events.EventKindRunCancelled:
		return true
	default:
		return false
	}
}
