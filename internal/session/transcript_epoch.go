package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
)

func (m *Manager) nextTranscriptEpoch(ctx context.Context, session *Session, sessionID string) (int64, error) {
	current := int64(0)
	if session != nil {
		current = session.Info().TranscriptEpoch
	}
	if m != nil && m.transcriptEpochStore != nil {
		epoch, err := m.transcriptEpochStore.SessionTranscriptEpoch(ctx, sessionID)
		if err != nil && !errors.Is(err, store.ErrSessionNotFound) {
			return 0, fmt.Errorf("session: read transcript epoch for %q: %w", sessionID, err)
		}
		if epoch > current {
			current = epoch
		}
	}
	return current + 1, nil
}

func (m *Manager) ensureTranscriptEpoch(
	ctx context.Context,
	session *Session,
	sessionID string,
	target int64,
) (int64, error) {
	if m != nil && m.transcriptEpochStore != nil {
		epoch, err := m.transcriptEpochStore.EnsureSessionTranscriptEpoch(ctx, store.SessionTranscriptEpochUpdate{
			SessionID: sessionID,
			Minimum:   target,
		})
		if err != nil {
			return 0, fmt.Errorf("session: ensure transcript epoch for %q: %w", sessionID, err)
		}
		if session != nil {
			session.setTranscriptEpoch(epoch)
		}
		return epoch, nil
	}
	if session != nil {
		session.setTranscriptEpoch(target)
	}
	return target, nil
}

func (s *Session) setTranscriptEpoch(epoch int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.TranscriptEpoch = epoch
	s.mu.Unlock()
}
