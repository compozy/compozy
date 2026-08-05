package session

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func (m *Manager) resumeCommittedConversationRewind(
	ctx context.Context,
	sessionID string,
	stored store.ConversationRewindResult,
) (ConversationRewindResult, error) {
	resumed, active := m.Get(sessionID)
	if !active || resumed.Info().State != StateActive {
		meta, err := m.readMetaWithContext(ctx, sessionID)
		if err != nil {
			return ConversationRewindResult{}, err
		}
		sanitized := clearedConversationMeta(meta, m.now())
		metaPath := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, sessionID))
		if err := store.WriteSessionMeta(metaPath, sanitized); err != nil {
			return ConversationRewindResult{}, fmt.Errorf(
				"session: persist rewound metadata for %q: %w",
				sessionID,
				err,
			)
		}
		if err := m.persistSessionCatalogFromMeta(ctx, sanitized); err != nil {
			return ConversationRewindResult{}, err
		}
		resumed, err = m.restartRewoundConversation(ctx, sanitized)
		if err != nil {
			return ConversationRewindResult{}, fmt.Errorf(
				"session: restart rewound session %q: %w",
				sessionID,
				err,
			)
		}
	}
	epoch, err := m.ensureTranscriptEpoch(ctx, resumed, sessionID, stored.TranscriptEpoch)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	owner := store.SessionDBOwner{SessionID: sessionID, WorkspaceID: resumed.Info().WorkspaceID}
	if err := m.discardOwnedMaterializedSessionLedger(ctx, owner, resumed.DBPath()); err != nil {
		return ConversationRewindResult{}, err
	}
	if err := m.recordConversationRewindEvent(ctx, resumed, stored, epoch); err != nil {
		return ConversationRewindResult{}, err
	}
	page, err := m.TranscriptPage(ctx, sessionID, transcript.PageQuery{Limit: 1})
	if err != nil {
		return ConversationRewindResult{}, err
	}
	stored.Generation = page.Generation
	stored.MaxSequence = page.MaxSequence
	return conversationRewindResult(resumed, stored, epoch), nil
}

func (m *Manager) restartRewoundConversation(
	ctx context.Context,
	meta store.SessionMeta,
) (*Session, error) {
	spec, err := m.prepareResumeStart(ctx, meta)
	if err != nil {
		return nil, err
	}
	spec.acpSessionID = ""
	spec.resumeReplay = true
	spec.resumeReplayReason = "conversation_rewind"
	return m.startSession(ctx, &spec)
}

func conversationRewindResult(
	resumed *Session,
	stored store.ConversationRewindResult,
	epoch int64,
) ConversationRewindResult {
	return ConversationRewindResult{
		Session: resumed, TranscriptEpoch: epoch, TargetMessageID: stored.TargetMessageID,
		ArchivedFrom: stored.ArchivedFromSequence, ArchivedThrough: stored.ArchivedToSequence,
		ArchivedEvents: stored.ArchivedEventCount, Generation: stored.Generation,
		MaxSequence: stored.MaxSequence, DraftText: stored.DraftText, Replayed: stored.Replayed,
	}
}

func (m *Manager) recordConversationRewindEvent(
	ctx context.Context,
	session *Session,
	result store.ConversationRewindResult,
	transcriptEpoch int64,
) error {
	payload, err := json.Marshal(map[string]any{
		"target_message_id":         result.TargetMessageID,
		"archived_from_sequence":    result.ArchivedFromSequence,
		"archived_through_sequence": result.ArchivedToSequence,
		"archived_event_count":      result.ArchivedEventCount,
		"generation":                result.Generation,
		"transcript_epoch":          transcriptEpoch,
	})
	if err != nil {
		return fmt.Errorf("session: marshal conversation rewind event: %w", err)
	}
	eventID := "rewind:" + result.IdempotencyKey
	_, err = m.appendDurableSessionEvent(ctx, session.ID, store.SessionEvent{
		ID: eventID, SessionID: session.ID, TurnID: eventID,
		Type: eventspkg.SessionConversationRewound, AgentName: session.Info().AgentName,
		Content: string(payload), Archived: true, Timestamp: result.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("session: persist conversation rewind event: %w", err)
	}
	return nil
}
