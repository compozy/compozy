package extractor

import (
	"errors"
	"fmt"

	"strconv"
	"strings"

	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func normalizeTurn(turn memcontract.TurnRecord, now func() time.Time) (memcontract.TurnRecord, error) {
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	turn.SessionID = strings.TrimSpace(turn.SessionID)
	turn.RootSessionID = firstNonEmpty(turn.RootSessionID, turn.SessionID)
	turn.ParentSessionID = strings.TrimSpace(turn.ParentSessionID)
	turn.AgentID = strings.TrimSpace(turn.AgentID)
	turn.ActorKind = firstNonEmpty(turn.ActorKind, actorKindRoot)
	turn.WorkspaceID = strings.TrimSpace(turn.WorkspaceID)
	turn.Trigger = turn.Trigger.Normalize()
	if turn.SessionID == "" {
		return memcontract.TurnRecord{}, errors.New("memory extractor: session id is required")
	}
	if turn.UntilMessageSeq <= 0 {
		return memcontract.TurnRecord{}, errors.New("memory extractor: until message sequence is required")
	}
	if turn.SinceMessageSeq <= 0 {
		turn.SinceMessageSeq = turn.UntilMessageSeq
	}
	if turn.SinceMessageSeq > turn.UntilMessageSeq {
		return memcontract.TurnRecord{}, errors.New("memory extractor: since message sequence exceeds until sequence")
	}
	if turn.Trigger == "" {
		turn.Trigger = memcontract.TriggerPostMessage
	}
	if err := turn.Trigger.Validate(); err != nil {
		return memcontract.TurnRecord{}, fmt.Errorf("memory extractor: trigger: %w", err)
	}
	for idx := range turn.Snapshot.Messages {
		turn.Snapshot.Messages[idx].Role = strings.TrimSpace(turn.Snapshot.Messages[idx].Role)
		turn.Snapshot.Messages[idx].Content = strings.TrimSpace(turn.Snapshot.Messages[idx].Content)
		if turn.Snapshot.Messages[idx].At.IsZero() {
			turn.Snapshot.Messages[idx].At = now().UTC()
		}
	}
	return turn, nil
}

func turnHasExtractableContent(turn memcontract.TurnRecord) bool {
	for _, message := range turn.Snapshot.Messages {
		if strings.TrimSpace(message.Content) != "" {
			return true
		}
	}
	return false
}

func mergeRequests(existing request, next request) request {
	merged := existing
	if next.turn.SinceMessageSeq < merged.turn.SinceMessageSeq {
		merged.turn.SinceMessageSeq = next.turn.SinceMessageSeq
	}
	if next.turn.UntilMessageSeq > merged.turn.UntilMessageSeq {
		merged.turn.UntilMessageSeq = next.turn.UntilMessageSeq
	}
	merged.turn.Snapshot.Messages = append(merged.turn.Snapshot.Messages, next.turn.Snapshot.Messages...)
	merged.coalesceCount++
	return merged
}

func enrichCandidate(
	candidate memcontract.Candidate,
	turn memcontract.TurnRecord,
	now func() time.Time,
) memcontract.Candidate {
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	candidate.WorkspaceID = firstNonEmpty(candidate.WorkspaceID, turn.WorkspaceID)
	candidate.Origin = candidate.Origin.Normalize()
	if candidate.Origin == "" {
		candidate.Origin = memcontract.OriginExtractor
	}
	if candidate.SubmittedAt.IsZero() {
		candidate.SubmittedAt = now().UTC()
	}
	if candidate.Metadata == nil {
		candidate.Metadata = map[string]string{}
	}
	candidate.Metadata["session_id"] = turn.SessionID
	candidate.Metadata["root_session_id"] = turn.RootSessionID
	if strings.TrimSpace(turn.ParentSessionID) != "" {
		candidate.Metadata["parent_session_id"] = turn.ParentSessionID
	}
	candidate.Metadata["actor_kind"] = turn.ActorKind
	candidate.Metadata["agent_id"] = turn.AgentID
	candidate.Metadata["since_message_seq"] = strconv.FormatInt(turn.SinceMessageSeq, 10)
	candidate.Metadata["until_message_seq"] = strconv.FormatInt(turn.UntilMessageSeq, 10)
	candidate.Metadata["trigger"] = string(turn.Trigger.Normalize())
	return candidate
}
