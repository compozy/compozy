package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.SessionMessageLedger = (*SessionRepo)(nil)

// SessionMessageWindow counts one sender's durable admissions into a target since a cutoff.
func (g *SessionRepo) SessionMessageWindow(
	ctx context.Context,
	targetWorkspaceID, targetSessionID, senderSessionID string,
	since time.Time,
) (store.SessionMessageWindow, error) {
	if err := g.checkReady(ctx, "count session messages"); err != nil {
		return store.SessionMessageWindow{}, err
	}
	row, err := sqlcgen.New(g.db).SessionMessageRateWindow(ctx, sqlcgen.SessionMessageRateWindowParams{
		WorkspaceID:     targetWorkspaceID,
		SessionID:       targetSessionID,
		Since:           store.FormatTimestamp(since),
		SenderSessionID: senderSessionID,
	})
	if err != nil {
		return store.SessionMessageWindow{}, fmt.Errorf("store: count session messages: %w", err)
	}
	window := store.SessionMessageWindow{Count: int(row.MessageCount)}
	if oldest := strings.TrimSpace(row.OldestCreatedAt); oldest != "" {
		parsed, err := store.ParseTimestamp(oldest)
		if err != nil {
			return store.SessionMessageWindow{}, err
		}
		window.OldestAt = parsed
	}
	return window, nil
}

// LookupSessionPromptAdmissionByMessageID reads one admission by its message identity.
func (g *SessionRepo) LookupSessionPromptAdmissionByMessageID(
	ctx context.Context,
	workspaceID, sessionID, messageID string,
) (store.SessionPromptAdmission, bool, error) {
	if err := g.checkReady(ctx, "look up session prompt admission by message"); err != nil {
		return store.SessionPromptAdmission{}, false, err
	}
	row, err := sqlcgen.New(g.db).GetSessionPromptAdmissionByMessageID(
		ctx,
		sqlcgen.GetSessionPromptAdmissionByMessageIDParams{
			WorkspaceID: workspaceID, SessionID: sessionID, MessageID: messageID,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionPromptAdmission{}, false, nil
	}
	if err != nil {
		return store.SessionPromptAdmission{}, false, err
	}
	admission, err := sessionPromptAdmissionFromGenerated(&row)
	return admission, err == nil, err
}
