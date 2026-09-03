package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"

	terminalvt "github.com/compozy/compozy/internal/terminal/vt"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

const outputReadBytes = 32 * 1024

func (s *session) readOutput() {
	reads := make(chan outputRead, 1)
	go readProcessOutput(s.ctx, s.proc.Reader(), reads, s.flow)
	coalescer := newOutputCoalescer(s.acceptOutput)
	for {
		select {
		case read := <-reads:
			filtered := s.filter.Filter(read.data)
			if len(filtered.MarkerFacts) > 0 {
				if err := s.manager.journal.ConsumeMarkerFacts(
					s.ctx, s.Info(), filtered.MarkerFacts,
				); err != nil {
					s.audit.SetBlocked(true)
					s.manager.logger.Error(
						"terminal: consume authenticated marker facts",
						"terminal_id", s.info.ID,
						"error", err,
					)
				}
			}
			if len(read.data) > 0 && len(filtered.DisplayBytes) == 0 {
				s.acceptFilteredOutput()
			}
			coalescer.Push(filtered.DisplayBytes)
			if read.err != nil {
				coalescer.Flush()
				if !errors.Is(read.err, io.EOF) && !errors.Is(read.err, io.ErrClosedPipe) {
					s.manager.logger.Debug("terminal: output reader ended", "terminal_id", s.info.ID, "error", read.err)
				}
				s.markReaderEnded()
				return
			}
		case <-coalescer.Ready():
			coalescer.Flush()
		}
	}
}

type outputRead struct {
	data []byte
	err  error
}

func readProcessOutput(ctx context.Context, reader io.Reader, output chan<- outputRead, flow *terminalwire.Group) {
	buffer := make([]byte, outputReadBytes)
	for {
		if err := flow.WaitProducer(ctx); err != nil {
			output <- outputRead{err: err}
			return
		}
		count, err := reader.Read(buffer)
		read := outputRead{err: err}
		if count > 0 {
			read.data = append([]byte(nil), buffer[:count]...)
		}
		output <- read
		if err != nil {
			return
		}
	}
}

func (s *session) acceptOutput(input []byte) {
	if len(input) == 0 {
		return
	}
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	if s.captureOutput {
		s.captureMu.Lock()
		s.appendCaptureLocked(input)
		s.captureMu.Unlock()
	}
	s.appendRecording(input)
	s.manager.observeJournalOutput(s.Info(), input)
	s.mu.Lock()
	s.ring.SetModePreamble(input)
	start, end := s.ring.Append(input)
	s.lastActivity = s.manager.now()
	s.bumpRevisionLocked()
	subscribers := make([]*subscription, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()
	vtInput := slices.Concat(s.vtCarry, input)
	complete, carry := splitCompleteUTF8(vtInput)
	s.vtCarry = append(s.vtCarry[:0], carry...)
	completeEnd := end - uint64(len(carry))
	if _, err := s.vt.WriteAt(complete, completeEnd); err != nil && !errors.Is(err, terminalvt.ErrClosed) {
		s.manager.logger.Warn("terminal: feed emulator", "terminal_id", s.info.ID, "error", err)
	}
	frame := Frame{Op: terminalwire.ServerOpOutput, Seq: start, Payload: append([]byte(nil), input...)}
	for _, subscriber := range subscribers {
		subscriber.deliver(frame, end)
	}
}

func (s *session) acceptRedactedInput(characters int) {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	marker := RedactedInputMarker(characters)
	rendered := []byte(RenderOutputSegment(marker))
	s.appendRecordingMarker(marker)
	s.mu.Lock()
	start, end := s.ring.AppendRedactedInput(marker.Characters)
	s.lastActivity = s.manager.now()
	s.bumpRevisionLocked()
	subscribers := make([]*subscription, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()
	if _, err := s.vt.WriteAt(rendered, end); err != nil && !errors.Is(err, terminalvt.ErrClosed) {
		s.manager.logger.Warn("terminal: feed redacted marker to emulator", "terminal_id", s.info.ID, "error", err)
	}
	payload, err := json.Marshal(redactedInputFramePayload{
		Seq: terminalSequenceString(start), Characters: marker.Characters,
	})
	if err != nil {
		s.manager.logger.Error("terminal: encode redacted input frame", "terminal_id", s.info.ID, "error", err)
		return
	}
	frame := Frame{Op: terminalwire.ServerOpRedactedInput, Seq: start, Payload: payload}
	for _, subscriber := range subscribers {
		subscriber.deliver(frame, end)
	}
}

func (s *session) acceptFilteredOutput() {
	s.manager.observeJournalOutput(s.Info(), nil)
	s.mu.Lock()
	s.lastActivity = s.manager.now()
	s.bumpRevisionLocked()
	s.mu.Unlock()
}

func (s *session) programTitleChanged(title string) {
	if title == "" {
		return
	}
	s.mu.Lock()
	if s.titlePinned || s.info.Title == title {
		s.mu.Unlock()
		return
	}
	s.info.Title = title
	info := s.infoSnapshotLocked()
	subscribers := make([]*subscription, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()
	payload, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		s.manager.logger.Warn("terminal: encode title frame", "terminal_id", info.ID, "error", err)
		return
	}
	for _, subscriber := range subscribers {
		subscriber.deliver(Frame{Op: terminalwire.ServerOpTitle, Payload: payload}, 0)
	}
	s.manager.events.Notify(s.ctx, Event{
		Kind: EventKindTitleChanged, WorkspaceID: info.WS, ProfileID: info.ProfileID,
		ProfileName: s.profileName,
		TerminalID:  info.ID, Actor: Actor{Kind: ActorKindSystem, ID: "terminal-program", ProfileID: info.ProfileID},
		Info: &info, Detail: &EventDetail{Title: title}, At: s.manager.now(),
	})
}
