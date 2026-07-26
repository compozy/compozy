package core

import (
	"net/http"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/transcript"
	"github.com/gin-gonic/gin"
)

// SessionRecap returns a deterministic recap composed from persisted session state.
func (h *BaseHandlers) SessionRecap(c *gin.Context) {
	_, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	limit, err := parseOptionalPositiveIntQuery(c, "limit", defaultSessionRecapLimit, maxSessionRecapLimit)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	page, err := h.Sessions.TranscriptPage(
		c.Request.Context(),
		sessionID,
		transcript.PageQuery{Limit: recapTranscriptReadLimit(limit)},
	)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	entries := page.Entries
	recentEntries := recentTranscriptEntries(entries, limit)
	markers := recentTranscriptMarkersFromEntries(entries, 5)
	queueSummary, err := h.Sessions.InputQueueSummary(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	pendingMarkers := pendingTranscriptMarkerCountFromEntries(entries)
	eventCursor := page.MaxSequence
	payload := contract.RecapPayload{
		Session:        SessionPayloadFromInfo(info),
		RecentMarkers:  markers,
		RecentMessages: transcript.MessagesFromEntries(recentEntries),
		PendingInputs:  queueSummary.PendingInputs,
		PendingMarkers: pendingMarkers,
		Snapshot: contract.RecapSnapshotPayload{
			GeneratedAt:      h.Now().UTC(),
			EventCursor:      eventCursor,
			TranscriptCursor: eventCursor,
			QueueGeneration:  queueSummary.QueueGeneration,
			Consistency:      recapConsistencyPersistedReads,
		},
	}
	c.JSON(http.StatusOK, contract.SessionRecapResponse{Recap: payload})
}

func recapTranscriptReadLimit(limit int) int {
	if limit < pendingTranscriptMarkerLimit {
		return pendingTranscriptMarkerLimit
	}
	return limit
}

func recentTranscriptEntries(entries []transcript.Entry, limit int) []transcript.Entry {
	if limit <= 0 || len(entries) <= limit {
		return transcript.CloneEntries(entries)
	}
	return transcript.CloneEntries(entries[len(entries)-limit:])
}

func pendingTranscriptMarkerCountFromEntries(entries []transcript.Entry) int {
	allMarkers := markerEntriesWithSequence(entries)
	if len(allMarkers) == 0 {
		return 0
	}
	sort.SliceStable(allMarkers, func(i int, j int) bool {
		return allMarkers[i].Sequence < allMarkers[j].Sequence
	})

	var runtimeRecoveredAfter int64
	redactedAfterByKind := make(map[string]int64)
	for _, item := range allMarkers {
		kind := strings.TrimSpace(item.Marker.Kind)
		if item.EventType == events.TranscriptMarkerCreated &&
			kind == transcript.MarkerSessionRecovered &&
			item.Sequence > runtimeRecoveredAfter {
			runtimeRecoveredAfter = item.Sequence
			continue
		}
		if item.EventType == events.TranscriptMarkerRedacted && item.Sequence > redactedAfterByKind[kind] {
			redactedAfterByKind[kind] = item.Sequence
		}
	}

	pending := 0
	for _, item := range allMarkers {
		if item.EventType != events.TranscriptMarkerCreated {
			continue
		}
		kind := strings.TrimSpace(item.Marker.Kind)
		if item.Sequence <= redactedAfterByKind[kind] {
			continue
		}
		if transcriptMarkerRuntimeOpenKind(kind) && item.Sequence > runtimeRecoveredAfter {
			pending++
			continue
		}
		if transcriptMarkerDurableOpenKind(kind) {
			pending++
		}
	}
	return pending
}

func markerEntriesWithSequence(entries []transcript.Entry) []markerWithSequence {
	allMarkers := make([]markerWithSequence, 0, len(entries))
	for _, entry := range entries {
		if !isTranscriptMarkerEventType(entry.EventType) || entry.Marker == nil {
			continue
		}
		marker := entry.Marker.Normalize()
		allMarkers = append(allMarkers, markerWithSequence{
			Sequence:  entry.Sequence,
			EventType: entry.EventType,
			Marker:    marker,
		})
	}
	return allMarkers
}

type markerWithSequence struct {
	Sequence  int64
	EventType string
	Marker    transcript.Marker
}

func transcriptMarkerRuntimeOpenKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case transcript.MarkerSessionUnhealthy,
		transcript.MarkerPromptTimeout:
		return true
	default:
		return false
	}
}

func transcriptMarkerDurableOpenKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case transcript.MarkerProviderFailure,
		transcript.MarkerMCPAuthRequired:
		return true
	default:
		return false
	}
}

func recentTranscriptMarkersFromEntries(entries []transcript.Entry, limit int) []contract.TranscriptMarkerPayload {
	if limit <= 0 {
		return []contract.TranscriptMarkerPayload{}
	}
	markers := make([]contract.TranscriptMarkerPayload, 0, limit)
	for index := len(entries) - 1; index >= 0 && len(markers) < limit; index-- {
		entry := entries[index]
		if !isTranscriptMarkerEventType(entry.EventType) || entry.Marker == nil {
			continue
		}
		markers = append(markers, transcriptMarkerPayload(*entry.Marker))
	}
	return markers
}

func transcriptMarkerPayload(marker transcript.Marker) contract.TranscriptMarkerPayload {
	normalized := marker.Normalize()
	return contract.TranscriptMarkerPayload{
		Kind:       normalized.Kind,
		OccurredAt: normalized.OccurredAt,
		Summary:    normalized.Summary,
		Evidence:   normalized.Evidence,
		Diagnostic: normalized.Diagnostic,
	}
}

func isTranscriptMarkerEventType(eventType string) bool {
	return eventType == events.TranscriptMarkerCreated || eventType == events.TranscriptMarkerRedacted
}
