package core

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/transcript"
)

func (h *BaseHandlers) writeTranscriptChangePages(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	afterSequence int64,
	limit int,
	first *transcript.ChangePage,
) (int64, int64, error) {
	cursor := afterSequence
	var generation int64
	generationSet := false
	page := first
	for {
		if page == nil {
			next, err := h.queryTranscriptChangesPage(ctx, sessionID, info, cursor, limit)
			if err != nil {
				return cursor, generation, err
			}
			page = &next
		}
		if generationSet && page.Generation != generation {
			return cursor, generation, fmt.Errorf(
				"%w: generation changed from %d to %d while draining changes",
				transcript.ErrProjectionIncompatible,
				generation,
				page.Generation,
			)
		}
		generation = page.Generation
		generationSet = true

		nextCursor := page.NextAfter
		if !page.HasMore && page.MaxSequence > nextCursor {
			nextCursor = page.MaxSequence
		}
		if page.HasMore && nextCursor <= cursor {
			return cursor, generation, fmt.Errorf(
				"%w: change page did not advance beyond %d",
				transcript.ErrProjectionCorrupt,
				cursor,
			)
		}
		if nextCursor < cursor {
			return cursor, generation, fmt.Errorf(
				"%w: change cursor regressed from %d to %d",
				transcript.ErrProjectionCorrupt,
				cursor,
				nextCursor,
			)
		}
		if nextCursor > cursor || len(page.Entries) > 0 {
			if err := h.writeTranscriptDeltaPage(writer, sessionID, info, *page, nextCursor); err != nil {
				return cursor, generation, err
			}
			cursor = nextCursor
		}
		if !page.HasMore {
			return cursor, generation, nil
		}
		page = nil
	}
}

func (h *BaseHandlers) queryTranscriptChangesPage(
	ctx context.Context,
	sessionID string,
	info *session.Info,
	afterSequence int64,
	limit int,
) (transcript.ChangePage, error) {
	startedAt := time.Now()
	page, err := h.Sessions.TranscriptChanges(ctx, sessionID, transcript.ChangeQuery{
		AfterSequence: afterSequence,
		Limit:         limit,
	})
	h.logTranscriptAssembly(
		ctx,
		sessionID,
		info,
		"delta",
		afterSequence,
		0,
		len(page.Entries),
		time.Since(startedAt),
		err,
	)
	if err != nil {
		return transcript.ChangePage{}, fmt.Errorf("query transcript changes: %w", err)
	}
	return page, nil
}

func (h *BaseHandlers) writeTranscriptDeltaPage(
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	page transcript.ChangePage,
	cursor int64,
) error {
	workspaceID, workspacePath := h.streamWorkspaceFields(info)
	entries := page.Entries
	if entries == nil {
		entries = []transcript.Entry{}
	}
	return WriteSSE(writer, SSEMessage{
		ID:   strconv.FormatInt(cursor, 10),
		Name: contract.SessionStreamEventTranscriptDelta,
		Data: contract.TranscriptDeltaPayload{
			SessionID:     sessionID,
			WorkspaceID:   workspaceID,
			WorkspacePath: workspacePath,
			Epoch:         transcriptEpoch(info),
			Generation:    page.Generation,
			Entries:       entries,
			Cursor:        cursor,
			MaxSequence:   page.MaxSequence,
			HasMore:       page.HasMore,
		},
	})
}
