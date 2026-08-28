package terminal

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	recorderBufferLimit         = 1 << 20
	recordingPersistenceTimeout = 5 * time.Second
)

type activeRecording struct {
	id        string
	startedAt time.Time
	buffer    bytes.Buffer
	truncated bool
}

func newActiveRecording(id string, info Info, cols, rows uint16, startedAt time.Time) (*activeRecording, error) {
	header := map[string]any{
		"version": 2, "width": cols, "height": rows, "timestamp": startedAt.Unix(),
		"env": map[string]string{"SHELL": info.Shell, "TERM": "xterm-256color"},
	}
	encoded, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("terminal: encode recording header: %w", err)
	}
	recording := &activeRecording{id: id, startedAt: startedAt}
	recording.buffer.Grow(min(recorderBufferLimit, len(encoded)+1))
	recording.buffer.Write(encoded)
	recording.buffer.WriteByte('\n')
	return recording, nil
}

func (r *activeRecording) appendOutput(at time.Time, output []byte) bool {
	if r == nil || len(output) == 0 || r.truncated {
		return r != nil && r.truncated
	}
	entry, err := json.Marshal([]any{at.Sub(r.startedAt).Seconds(), "o", string(output)})
	if err != nil || r.buffer.Len()+len(entry)+1 > recorderBufferLimit {
		r.truncated = true
		return true
	}
	r.buffer.Write(entry)
	r.buffer.WriteByte('\n')
	return false
}

func (r *activeRecording) appendMarker(at time.Time, marker OutputSegment) bool {
	if r == nil || r.truncated {
		return r != nil && r.truncated
	}
	entry, err := json.Marshal([]any{
		at.Sub(r.startedAt).Seconds(), "m",
		map[string]any{"kind": marker.Kind, "characters": marker.Characters},
	})
	if err != nil || r.buffer.Len()+len(entry)+1 > recorderBufferLimit {
		r.truncated = true
		return true
	}
	r.buffer.Write(entry)
	r.buffer.WriteByte('\n')
	return false
}

func (r *activeRecording) contents() []byte {
	if r == nil {
		return nil
	}
	return append([]byte(nil), r.buffer.Bytes()...)
}

func (s *session) startRecording(ctx context.Context, actor Actor) (RecordingRef, error) {
	ref, err := s.beginRecording()
	if err != nil {
		return RecordingRef{}, err
	}
	s.emitRecordingEvent(ctx, EventKindRecordingStarted, actor, ref, "", false)
	return ref, nil
}

func (s *session) beginRecording() (RecordingRef, error) {
	if !RecordingAvailable(s.Info().Capabilities) {
		info := s.Info()
		return RecordingRef{}, &Error{
			Code:    ErrorCodeRecordingUnavailable,
			Message: "recording is unavailable for this terminal",
			Mode:    info.Mode,
			Err:     ErrUnsupported,
		}
	}
	if err := s.runningGate(); err != nil {
		return RecordingRef{}, err
	}
	id, err := newRecordingID(s.manager.entropy)
	if err != nil {
		return RecordingRef{}, err
	}
	startedAt := s.manager.now()
	info := s.Info()
	s.mu.RLock()
	cols, rows := s.cols, s.rows
	s.mu.RUnlock()
	recording, err := newActiveRecording(id, info, cols, rows, startedAt)
	if err != nil {
		return RecordingRef{}, err
	}
	s.recordingMu.Lock()
	if s.recordingSealed {
		s.recordingMu.Unlock()
		return RecordingRef{}, &Error{Code: ErrorCodeExited, Message: errorMessageExited, Err: ErrExited}
	}
	if s.recording != nil || s.failedRecording != nil {
		s.recordingMu.Unlock()
		return RecordingRef{}, &Error{
			Code: ErrorCodeRecordingAlreadyStarted, Message: "terminal recording is already active", Err: ErrRecording,
		}
	}
	s.recording = recording
	s.recordingMu.Unlock()
	ref := RecordingRef{ID: id, TerminalID: info.ID, ProfileID: info.ProfileID, StartedAt: startedAt}
	return ref, nil
}

func (s *session) stopRecording(ctx context.Context, actor Actor, reason string) (RecordingRef, error) {
	recording, err := s.beginRecordingPersistence()
	if err != nil {
		return RecordingRef{}, err
	}
	defer s.recordingWG.Done()
	return s.persistStoppedRecording(ctx, actor, recording, reason)
}

func (s *session) beginRecordingPersistence() (*activeRecording, error) {
	s.recordingMu.Lock()
	defer s.recordingMu.Unlock()
	if s.recordingSealed {
		return nil, &Error{Code: ErrorCodeExited, Message: errorMessageExited, Err: ErrExited}
	}
	recording := s.takeRecordingLocked()
	if recording == nil {
		return nil, &Error{
			Code: ErrorCodeRecordingNotActive, Message: "terminal recording is not active", Err: ErrRecording,
		}
	}
	s.recordingWG.Add(1)
	return recording, nil
}

func (s *session) stopRecordingForFinalization(
	ctx context.Context,
	actor Actor,
	reason string,
) (RecordingRef, error) {
	s.recordingMu.Lock()
	recording := s.takeRecordingLocked()
	s.recordingMu.Unlock()
	if recording == nil {
		return RecordingRef{}, &Error{
			Code: ErrorCodeRecordingNotActive, Message: "terminal recording is not active", Err: ErrRecording,
		}
	}
	return s.persistStoppedRecording(ctx, actor, recording, reason)
}

func (s *session) takeRecordingLocked() *activeRecording {
	recording := s.recording
	if recording == nil {
		recording = s.failedRecording
		s.failedRecording = nil
	} else {
		s.recording = nil
	}
	return recording
}

func (s *session) persistStoppedRecording(
	ctx context.Context,
	actor Actor,
	recording *activeRecording,
	reason string,
) (RecordingRef, error) {
	persisted, err := s.persistRecording(ctx, actor, recording, reason, true)
	if err != nil {
		s.retainFailedRecording(recording)
	}
	return persisted, err
}

func (s *session) appendRecording(output []byte) {
	s.recordingMu.Lock()
	if s.recordingSealed {
		s.recordingMu.Unlock()
		return
	}
	recording := s.recording
	if recording == nil || !recording.appendOutput(s.manager.now(), output) {
		s.recordingMu.Unlock()
		return
	}
	s.recording = nil
	s.recordingWG.Add(1)
	s.recordingMu.Unlock()
	info := s.Info()
	stoppedAt := s.manager.now()
	s.emitRecordingEvent(s.ctx, EventKindRecordingStopped, Actor{
		Kind: ActorKindSystem, ID: "terminal-recorder", ProfileID: info.ProfileID,
	}, RecordingRef{
		ID: recording.id, TerminalID: info.ID, ProfileID: info.ProfileID,
		StartedAt: recording.startedAt, StoppedAt: &stoppedAt,
	}, "storage_stall", true)
	go func() {
		defer s.recordingWG.Done()
		actor := Actor{Kind: ActorKindSystem, ID: "terminal-recorder", ProfileID: info.ProfileID}
		persistCtx, cancel := boundedCleanupContext(s.ctx, recordingPersistenceTimeout)
		defer cancel()
		if _, err := s.persistRecording(persistCtx, actor, recording, "storage_stall", false); err != nil {
			s.retainFailedRecording(recording)
			s.manager.logger.Warn("terminal: persist truncated recording", "terminal_id", s.Info().ID, "error", err)
		}
	}()
}

func (s *session) appendRecordingMarker(marker OutputSegment) {
	s.recordingMu.Lock()
	if s.recordingSealed {
		s.recordingMu.Unlock()
		return
	}
	recording := s.recording
	if recording == nil || !recording.appendMarker(s.manager.now(), marker) {
		s.recordingMu.Unlock()
		return
	}
	s.recording = nil
	s.recordingWG.Add(1)
	s.recordingMu.Unlock()
	s.persistTruncatedRecording(recording)
}

func (s *session) persistTruncatedRecording(recording *activeRecording) {
	info := s.Info()
	stoppedAt := s.manager.now()
	s.emitRecordingEvent(s.ctx, EventKindRecordingStopped, Actor{
		Kind: ActorKindSystem, ID: "terminal-recorder", ProfileID: info.ProfileID,
	}, RecordingRef{
		ID: recording.id, TerminalID: info.ID, ProfileID: info.ProfileID,
		StartedAt: recording.startedAt, StoppedAt: &stoppedAt,
	}, "storage_stall", true)
	go func() {
		defer s.recordingWG.Done()
		actor := Actor{Kind: ActorKindSystem, ID: "terminal-recorder", ProfileID: info.ProfileID}
		persistCtx, cancel := boundedCleanupContext(s.ctx, recordingPersistenceTimeout)
		defer cancel()
		if _, err := s.persistRecording(persistCtx, actor, recording, "storage_stall", false); err != nil {
			s.retainFailedRecording(recording)
			s.manager.logger.Warn("terminal: persist truncated recording", "terminal_id", s.Info().ID, "error", err)
		}
	}()
}

func (s *session) retainFailedRecording(recording *activeRecording) {
	if recording == nil {
		return
	}
	s.recordingMu.Lock()
	if s.failedRecording == nil {
		s.failedRecording = recording
	}
	s.recordingMu.Unlock()
}

func (s *session) persistRecording(
	ctx context.Context,
	actor Actor,
	recording *activeRecording,
	reason string,
	emit bool,
) (RecordingRef, error) {
	if ctx == nil {
		return RecordingRef{}, errors.New("terminal: recording persistence context is required")
	}
	info := s.Info()
	stoppedAt := s.manager.now()
	settings := s.settings(ctx)
	ref := RecordingRef{
		ID: recording.id, TerminalID: info.ID, ProfileID: info.ProfileID,
		StartedAt: recording.startedAt, StoppedAt: &stoppedAt,
		ExpiresAt: stoppedAt.AddDate(0, 0, settings.RecordingRetentionDays),
	}
	contents := recording.contents()
	persisted, err := s.manager.journal.PersistRecording(ctx, info.WS, info.ID, ref, contents)
	if err != nil {
		if emit {
			s.emitRecordingEvent(ctx, EventKindRecordingStopped, actor, ref, "storage_error", recording.truncated)
		}
		return RecordingRef{}, fmt.Errorf("terminal: persist recording %q: %w", recording.id, err)
	}
	if emit {
		s.emitRecordingEvent(ctx, EventKindRecordingStopped, actor, persisted, reason, recording.truncated)
	}
	return persisted, nil
}

func (s *session) emitRecordingEvent(
	ctx context.Context,
	kind EventKind,
	actor Actor,
	ref RecordingRef,
	reason string,
	truncated bool,
) {
	info := s.Info()
	s.manager.events.Notify(ctx, Event{
		Kind: kind, WorkspaceID: info.WS, ProfileID: info.ProfileID, ProfileName: s.profileName,
		TerminalID: info.ID, Actor: actor, Info: &info, Reason: reason, At: s.manager.now(),
		Detail: &EventDetail{RecordingID: ref.ID, Digest: ref.Digest, Bytes: ref.Bytes, Truncated: truncated},
	})
}

func newRecordingID(entropy io.Reader) (string, error) {
	raw := make([]byte, 8)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		return "", fmt.Errorf("terminal: generate recording id: %w", err)
	}
	return "rec-" + hex.EncodeToString(raw), nil
}
