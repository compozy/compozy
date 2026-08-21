package loop

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type TimelineTier string

const (
	TimelineNotable  TimelineTier = "notable"
	TimelineActivity TimelineTier = "activity"
	TimelineChatter  TimelineTier = "chatter"
)

type TimelineView string

const (
	TimelineViewNotable TimelineView = "notable"
	TimelineViewAll     TimelineView = "all"
)

var (
	ErrTimelineBranchChanged      = errors.New("timeline_branch_changed")
	ErrInvalidTimelineCursor      = errors.New("invalid_cursor")
	ErrTimelinePositionBeyondHead = errors.New("timeline_position_beyond_head")
)

var timelineTiers = map[RunEventKind]TimelineTier{
	RunEventNodeRunning:          TimelineActivity,
	RunEventNodeSucceeded:        TimelineNotable,
	RunEventNodeFailed:           TimelineNotable,
	RunEventGateVerdict:          TimelineNotable,
	RunEventGenerationStarted:    TimelineNotable,
	RunEventChannelMsg:           TimelineActivity,
	RunEventTokenTick:            TimelineChatter,
	RunEventNeedsApproval:        TimelineNotable,
	RunEventStatusChanged:        TimelineNotable,
	RunEventGoalTurnStarted:      TimelineActivity,
	RunEventGoalTurnCompleted:    TimelineActivity,
	RunEventGoalStatusChanged:    TimelineNotable,
	RunEventRuntimeApplied:       TimelineChatter,
	RunEventPredicateDiagnostic:  TimelineChatter,
	RunEventRouteTaken:           TimelineNotable,
	RunEventNodeRetryScheduled:   TimelineNotable,
	RunEventNodePaused:           TimelineNotable,
	RunEventNodeResumed:          TimelineNotable,
	RunEventNodeCanceled:         TimelineNotable,
	RunEventNodeKilled:           TimelineNotable,
	RunEventNodeQuarantined:      TimelineNotable,
	RunEventNodeRequeued:         TimelineNotable,
	RunEventNodeWaitStarted:      TimelineActivity,
	RunEventNodeWaitResumed:      TimelineActivity,
	RunEventNodeAttentionFlagged: TimelineNotable,
	RunEventNodeAttentionCleared: TimelineNotable,
	RunEventEffectResults:        TimelineActivity,
	RunEventCustomEvent:          TimelineActivity,
	RunEventDuplicateSuppressed:  TimelineChatter,
	RunEventTargetBreaker:        TimelineNotable,
	RunEventStaleScheduleDropped: TimelineChatter,
	RunEventLateArrival:          TimelineChatter,
	RunEventRequestOpened:        TimelineNotable,
	RunEventRequestAnswered:      TimelineNotable,
	RunEventRequestExpired:       TimelineNotable,
	RunEventRequestCanceled:      TimelineNotable,
	RunEventNodeAmended:          TimelineNotable,
	RunEventBranchPruned:         TimelineNotable,
	RunEventRunForked:            TimelineNotable,
}

type TimelineQuery struct {
	View     TimelineView
	Cursor   string
	Limit    int
	AfterSeq int64
}
type TimelineEntry struct {
	Seq        int64        `json:"seq"`
	FirstSeq   int64        `json:"first_seq,omitempty"`
	Kind       RunEventKind `json:"kind"`
	Generation int64        `json:"generation,omitempty"`
	NodeID     NodeID       `json:"node_id,omitempty"`
	Attempt    int          `json:"attempt,omitempty"`
	Title      string       `json:"title"`
	At         time.Time    `json:"at"`
}
type TimelinePage struct {
	RunID      RunID           `json:"run_id"`
	HeadSeq    int64           `json:"head_seq"`
	Entries    []TimelineEntry `json:"entries"`
	NextCursor string          `json:"next_cursor,omitempty"`
}
type timelineCursor struct {
	RunID        RunID        `json:"run_id"`
	View         TimelineView `json:"view"`
	FixedHeadSeq int64        `json:"fixed_head_seq"`
	BeforeSeq    int64        `json:"before_seq"`
}

type TimelinePositionError struct {
	Position int64
	Head     int64
}

func (e *TimelinePositionError) Error() string {
	return fmt.Sprintf("position %d is beyond this run's history (head: %d)", e.Position, e.Head)
}

func (e *TimelinePositionError) Unwrap() error {
	return ErrTimelinePositionBeyondHead
}

func TimelineTierFor(kind RunEventKind) (TimelineTier, bool) {
	tier, ok := timelineTiers[kind]
	return tier, ok
}

// ProjectTimelineEvent applies the server-owned tier and meaning projection to one live event.
func ProjectTimelineEvent(event RunEvent, view TimelineView) (*TimelineEntry, error) {
	if view == "" {
		view = TimelineViewNotable
	}
	if view != TimelineViewNotable && view != TimelineViewAll {
		return nil, fmt.Errorf("%w: invalid timeline view %q", ErrValidation, view)
	}
	tier, ok := TimelineTierFor(RunEventKind(event.Kind))
	if !ok {
		return nil, fmt.Errorf("%w: unclassified event kind %q", ErrValidation, event.Kind)
	}
	if view == TimelineViewNotable && tier != TimelineNotable {
		return nil, nil
	}
	entry, err := timelineEntry(event)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// ProjectTimeline creates a snapshot-fenced newest-first page from durable events.
func ProjectTimeline(runID RunID, events []RunEvent, query TimelineQuery) (TimelinePage, error) {
	head := int64(0)
	for _, event := range events {
		if event.LoopRunID == runID && event.Seq > head {
			head = event.Seq
		}
	}
	return projectTimelineWithHead(runID, head, events, query)
}

func projectTimelineWithHead(runID RunID, head int64, events []RunEvent, query TimelineQuery) (TimelinePage, error) {
	query, err := normalizeTimelineQuery(query)
	if err != nil {
		return TimelinePage{}, err
	}
	if query.AfterSeq > head {
		return TimelinePage{}, &TimelinePositionError{Position: query.AfterSeq, Head: head}
	}
	fixedHead, before := head, head+1
	if query.Cursor != "" {
		cursor, err := decodeTimelineCursor(query.Cursor)
		if err != nil {
			return TimelinePage{}, err
		}
		if cursor.RunID != runID || cursor.View != query.View {
			return TimelinePage{}, fmt.Errorf("%w: cursor belongs to another timeline", ErrTimelineBranchChanged)
		}
		fixedHead, before = cursor.FixedHeadSeq, cursor.BeforeSeq
		if fixedHead > head || before > fixedHead+1 {
			return TimelinePage{}, fmt.Errorf("%w: cursor head is beyond run head %d", ErrInvalidTimelineCursor, head)
		}
	}
	filtered := make([]RunEvent, 0, len(events))
	for _, event := range events {
		tier, ok := timelineTiers[RunEventKind(event.Kind)]
		if !ok {
			return TimelinePage{}, fmt.Errorf("%w: unclassified event kind %q", ErrValidation, event.Kind)
		}
		if event.LoopRunID != runID || event.Seq > fixedHead || event.Seq >= before || event.Seq <= query.AfterSeq {
			continue
		}
		if query.View == TimelineViewNotable && tier != TimelineNotable {
			continue
		}
		filtered = append(filtered, event)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Seq > filtered[j].Seq
	})
	entries, err := coalesceTimeline(filtered)
	if err != nil {
		return TimelinePage{}, err
	}
	page := TimelinePage{RunID: runID, HeadSeq: fixedHead, Entries: []TimelineEntry{}}
	if len(entries) > query.Limit {
		page.Entries = entries[:query.Limit]
		cursor := timelineCursor{
			RunID: runID, View: query.View, FixedHeadSeq: fixedHead,
			BeforeSeq: page.Entries[len(page.Entries)-1].FirstSeq,
		}
		encoded, err := encodeTimelineCursor(cursor)
		if err != nil {
			return TimelinePage{}, err
		}
		page.NextCursor = encoded
	} else {
		page.Entries = entries
	}
	return page, nil
}

func normalizeTimelineQuery(query TimelineQuery) (TimelineQuery, error) {
	if query.View == "" {
		query.View = TimelineViewNotable
	}
	if query.View != TimelineViewNotable && query.View != TimelineViewAll {
		return TimelineQuery{}, fmt.Errorf("%w: invalid timeline view %q", ErrValidation, query.View)
	}
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 500 {
		return TimelineQuery{}, fmt.Errorf("%w: timeline limit must be between 1 and 500", ErrValidation)
	}
	if query.AfterSeq < 0 {
		return TimelineQuery{}, fmt.Errorf("%w: timeline position must not be negative", ErrValidation)
	}
	return query, nil
}

func coalesceTimeline(events []RunEvent) ([]TimelineEntry, error) {
	out := []TimelineEntry{}
	for _, event := range events {
		entry, err := timelineEntry(event)
		if err != nil {
			return nil, err
		}
		if len(out) > 0 && heartbeatKind(entry.Kind) && out[len(out)-1].Kind == entry.Kind {
			previous := &out[len(out)-1]
			if previous.FirstSeq == 0 {
				previous.FirstSeq = previous.Seq
			}
			if entry.Seq < previous.FirstSeq {
				previous.FirstSeq = entry.Seq
			}
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func heartbeatKind(kind RunEventKind) bool {
	return kind == RunEventTokenTick || kind == RunEventRuntimeApplied || kind == RunEventPredicateDiagnostic
}

func timelineEntry(event RunEvent) (TimelineEntry, error) {
	var payload struct {
		Generation   int64  `json:"generation"`
		NodeID       NodeID `json:"node_id"`
		Attempt      int    `json:"attempt"`
		GateID       string `json:"gate_id"`
		Status       string `json:"status"`
		Verdict      string `json:"verdict"`
		RelatedRunID string `json:"related_run_id"`
		ForkRunID    string `json:"fork_run_id"`
	}
	if len(event.Payload) > 0 {
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return TimelineEntry{}, fmt.Errorf("decode timeline event %d payload: %w", event.Seq, err)
		}
	}
	title := strings.ReplaceAll(event.Kind, "_", " ")
	switch RunEventKind(event.Kind) {
	case RunEventNodeRunning, RunEventNodeSucceeded, RunEventNodeFailed, RunEventNodePaused,
		RunEventNodeResumed, RunEventNodeCanceled, RunEventNodeKilled, RunEventNodeQuarantined,
		RunEventNodeRequeued:
		state := strings.TrimPrefix(event.Kind, "node_")
		if payload.NodeID != "" {
			title = fmt.Sprintf("step %s %s", payload.NodeID, strings.ReplaceAll(state, "_", " "))
		}
	case RunEventNodeRetryScheduled:
		if payload.NodeID != "" {
			title = fmt.Sprintf("step %s retry scheduled", payload.NodeID)
		}
	case RunEventNeedsApproval:
		if payload.GateID != "" {
			title = fmt.Sprintf("approval %q opened", payload.GateID)
		}
	case RunEventGateVerdict:
		if payload.GateID != "" {
			title = fmt.Sprintf("gate %q %s", payload.GateID, strings.TrimSpace(payload.Verdict))
		}
	case RunEventStatusChanged:
		if payload.Status != "" {
			title = "run status: " + payload.Status
		}
	case RunEventGenerationStarted:
		if payload.Generation > 0 {
			title = fmt.Sprintf("round %d started", payload.Generation)
		}
	case RunEventRunForked:
		related := payload.RelatedRunID
		if related == "" {
			related = payload.ForkRunID
		}
		if related != "" {
			title = fmt.Sprintf("Run forked to %s", related)
		}
	}
	return TimelineEntry{
		Seq: event.Seq, FirstSeq: event.Seq, Kind: RunEventKind(event.Kind),
		Generation: payload.Generation, NodeID: payload.NodeID, Attempt: payload.Attempt,
		Title: title, At: event.At.UTC(),
	}, nil
}

func encodeTimelineCursor(cursor timelineCursor) (string, error) {
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode timeline cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeTimelineCursor(value string) (timelineCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return timelineCursor{}, fmt.Errorf("%w: malformed timeline cursor", ErrInvalidTimelineCursor)
	}
	var cursor timelineCursor
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor.RunID == "" ||
		cursor.FixedHeadSeq < 0 || cursor.BeforeSeq < 1 {
		return timelineCursor{}, fmt.Errorf("%w: malformed timeline cursor", ErrInvalidTimelineCursor)
	}
	return cursor, nil
}
