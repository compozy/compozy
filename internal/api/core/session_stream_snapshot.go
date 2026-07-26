package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/transcript"
)

type transcriptSnapshotResult struct {
	cursor     int64
	generation int64
}

type sessionStreamEventObserver interface {
	OnAgentEvent(ctx context.Context, sessionID string, event any)
}

type transcriptSnapshotServedPayload struct {
	WorkspaceID        string `json:"workspace_id,omitempty"`
	WorkspacePath      string `json:"workspace_path,omitempty"`
	SessionID          string `json:"session_id"`
	Epoch              int64  `json:"epoch"`
	Generation         int64  `json:"generation"`
	Reset              bool   `json:"reset"`
	Reason             string `json:"reason,omitempty"`
	MaxSequence        int64  `json:"max_sequence"`
	HasOlder           bool   `json:"has_older"`
	NextBeforeSequence int64  `json:"next_before_sequence,omitempty"`
}

func (h *BaseHandlers) writeTranscriptSnapshot(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	limit int,
	reset bool,
	reason string,
) (transcriptSnapshotResult, error) {
	startedAt := time.Now()
	page, err := h.Sessions.TranscriptPage(ctx, sessionID, transcript.PageQuery{Limit: limit})
	h.logTranscriptAssembly(
		ctx,
		sessionID,
		info,
		"snapshot",
		0,
		0,
		len(page.Entries),
		time.Since(startedAt),
		err,
	)
	if err != nil {
		return transcriptSnapshotResult{}, fmt.Errorf("query transcript snapshot: %w", err)
	}
	workspaceID, workspacePath := h.streamWorkspaceFields(info)
	payload := contract.TranscriptSnapshotPayload{
		SessionID:          sessionID,
		WorkspaceID:        workspaceID,
		WorkspacePath:      workspacePath,
		Epoch:              transcriptEpoch(info),
		Generation:         page.Generation,
		Entries:            transcript.CloneEntries(page.Entries),
		MaxSequence:        page.MaxSequence,
		HasOlder:           page.HasOlder,
		NextBeforeSequence: page.NextBeforeSequence,
		Reset:              reset,
		Reason:             reason,
	}
	result := transcriptSnapshotResult{cursor: page.MaxSequence, generation: page.Generation}
	if err := WriteSSE(writer, SSEMessage{
		ID:   strconv.FormatInt(page.MaxSequence, 10),
		Name: contract.SessionStreamEventTranscriptSnapshot,
		Data: payload,
	}); err != nil {
		return result, err
	}
	h.emitTranscriptSnapshotServed(ctx, sessionID, info, transcriptSnapshotServedPayload{
		WorkspaceID:        workspaceID,
		WorkspacePath:      workspacePath,
		SessionID:          sessionID,
		Epoch:              payload.Epoch,
		Generation:         payload.Generation,
		Reset:              reset,
		Reason:             reason,
		MaxSequence:        page.MaxSequence,
		HasOlder:           page.HasOlder,
		NextBeforeSequence: page.NextBeforeSequence,
	})
	return result, nil
}

func (h *BaseHandlers) emitTranscriptSnapshotServed(
	ctx context.Context,
	sessionID string,
	info *session.Info,
	payload transcriptSnapshotServedPayload,
) {
	if h == nil || h.Observer == nil {
		return
	}
	observer, ok := h.Observer.(sessionStreamEventObserver)
	if !ok {
		return
	}
	if payload.WorkspaceID == "" && h.Sessions != nil {
		if latest, err := h.Sessions.Status(ctx, sessionID); err == nil && latest != nil {
			payload.WorkspaceID = latest.WorkspaceID
			payload.WorkspacePath = latest.Workspace
		}
	}
	if info != nil && payload.Epoch == 0 {
		payload.Epoch = transcriptEpoch(info)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("api: marshal transcript snapshot event failed", "session_id", sessionID, "error", err)
		}
		return
	}
	observer.OnAgentEvent(ctx, sessionID, acp.AgentEvent{Type: events.SessionStreamSnapshotServed, Raw: raw})
}

func transcriptReconnectResetReason(
	cursor int64,
	expectedEpoch *int64,
	expectedGeneration *int64,
	epoch int64,
	generation int64,
	maxSequence int64,
) string {
	if cursor == 0 {
		return ""
	}
	if expectedEpoch == nil || expectedGeneration == nil {
		return contract.TranscriptSnapshotReasonFenceMissing
	}
	if *expectedEpoch != epoch {
		return contract.TranscriptSnapshotReasonEpochMismatch
	}
	if *expectedGeneration != generation {
		return contract.TranscriptSnapshotReasonGenerationMismatch
	}
	if cursor > maxSequence {
		return contract.TranscriptSnapshotReasonSequenceReset
	}
	return ""
}

func transcriptEpoch(info *session.Info) int64 {
	if info == nil {
		return 0
	}
	return info.TranscriptEpoch
}
