package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

var errTranscriptProjectionUnavailable = errors.New(
	"session: event recorder does not expose the materialized transcript projection",
)

// TranscriptPage returns one bounded materialized transcript window.
func (m *Manager) TranscriptPage(
	ctx context.Context,
	id string,
	query transcript.PageQuery,
) (transcript.Page, error) {
	target := strings.TrimSpace(id)
	var page transcript.Page
	err := m.withTranscriptReader(ctx, target, func(reader transcript.Reader) error {
		var readErr error
		page, readErr = reader.TranscriptPage(ctx, query)
		return readErr
	})
	if err != nil {
		return transcript.Page{}, err
	}
	return page, nil
}

// TranscriptChanges returns materialized entries shaped after an event cursor.
func (m *Manager) TranscriptChanges(
	ctx context.Context,
	id string,
	query transcript.ChangeQuery,
) (transcript.ChangePage, error) {
	target := strings.TrimSpace(id)
	var page transcript.ChangePage
	err := m.withTranscriptReader(ctx, target, func(reader transcript.Reader) error {
		var readErr error
		page, readErr = reader.TranscriptChanges(ctx, query)
		return readErr
	})
	if err != nil {
		return transcript.ChangePage{}, err
	}
	return page, nil
}

func (m *Manager) withTranscriptReader(
	ctx context.Context,
	sessionID string,
	read func(transcript.Reader) error,
) error {
	if ctx == nil {
		return errors.New("session: transcript context is required")
	}
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("session: transcript session id is required")
	}
	if read == nil {
		return errors.New("session: transcript read callback is required")
	}
	for attempt := range 2 {
		recorder, cleanup, err := m.openQueryRecorder(ctx, sessionID)
		if err != nil {
			return err
		}
		reader, ok := recorder.(transcript.Reader)
		if !ok {
			m.logTranscriptCleanupError(ctx, sessionID, cleanup())
			return fmt.Errorf("%w for %q", errTranscriptProjectionUnavailable, sessionID)
		}
		err = read(reader)
		m.logTranscriptCleanupError(ctx, sessionID, cleanup())
		if err == nil {
			return nil
		}
		if errors.Is(err, store.ErrClosed) && attempt == 0 {
			if _, waitErr := m.waitForSessionFinalization(ctx, sessionID); waitErr != nil {
				return fmt.Errorf(
					"session: wait for finalization for %q after closed transcript recorder: %w",
					sessionID,
					waitErr,
				)
			}
			continue
		}
		return fmt.Errorf("session: query transcript projection for %q: %w", sessionID, err)
	}
	return fmt.Errorf("session: query transcript projection for %q: %w", sessionID, store.ErrClosed)
}

func (m *Manager) logTranscriptCleanupError(ctx context.Context, sessionID string, err error) {
	if err == nil {
		return
	}
	logger := m.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.WarnContext(ctx, "session: transcript cleanup failed", "session_id", sessionID, "error", err)
}
