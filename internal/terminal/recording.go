package terminal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/redact"
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
	stopped   sync.Once
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
	ref, err := s.beginRecording(ctx)
	if err != nil {
		return RecordingRef{}, err
	}
	s.emitRecordingEvent(ctx, EventKindRecordingStarted, actor, ref, "", false)
	return ref, nil
}

func (s *session) activeRecording() (RecordingRef, bool) {
	info := s.Info()
	s.recordingMu.Lock()
	defer s.recordingMu.Unlock()
	if s.recording == nil {
		return RecordingRef{}, false
	}
	return RecordingRef{
		ID: s.recording.id, TerminalID: info.ID, ProfileID: info.ProfileID,
		StartedAt: s.recording.startedAt,
	}, true
}

func (s *session) beginRecording(ctx context.Context) (RecordingRef, error) {
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
	info := s.Info()
	id, err := s.manager.journal.ReserveRecordingID(ctx, info.WS)
	if err != nil {
		return RecordingRef{}, fmt.Errorf("terminal: reserve recording id: %w", err)
	}
	releaseID := true
	defer func() {
		if releaseID {
			s.manager.journal.ReleaseRecordingID(info.WS, id)
		}
	}()
	startedAt := s.manager.now()
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
	releaseID = false
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
	s.persistTruncatedRecording(recording)
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
	go func() {
		defer s.recordingWG.Done()
		actor := Actor{Kind: ActorKindSystem, ID: "terminal-recorder", ProfileID: info.ProfileID}
		persistCtx, cancel := boundedCleanupContext(s.ctx, recordingPersistenceTimeout)
		defer cancel()
		if _, err := s.persistRecording(persistCtx, actor, recording, "storage_stall", true); err != nil {
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
			failureRef := recordingFailureRef(ref, contents)
			s.emitRecordingStoppedOnce(ctx, actor, recording, failureRef, "storage_error")
			s.manager.logger.Warn("terminal: recording artifact retained in memory after storage failure",
				"workspace_id", info.WS, "profile_id", info.ProfileID, "terminal_id", info.ID,
				"recording_id", recording.id, "digest", failureRef.Digest, "bytes", failureRef.Bytes,
				"reason", "storage_error", "error", err,
			)
		}
		return RecordingRef{}, fmt.Errorf("terminal: persist recording %q: %w", recording.id, err)
	}
	if emit {
		s.emitRecordingStoppedOnce(ctx, actor, recording, persisted, reason)
	}
	return persisted, nil
}

func recordingFailureRef(ref RecordingRef, contents []byte) RecordingRef {
	redacted := []byte(redact.String(string(contents)))
	digest := sha256.Sum256(redacted)
	ref.Digest = hex.EncodeToString(digest[:])
	ref.Bytes = int64(len(redacted))
	return ref
}

func (s *session) emitRecordingStoppedOnce(
	ctx context.Context,
	actor Actor,
	recording *activeRecording,
	ref RecordingRef,
	reason string,
) {
	recording.stopped.Do(func() {
		s.emitRecordingEvent(ctx, EventKindRecordingStopped, actor, ref, reason, recording.truncated)
	})
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
