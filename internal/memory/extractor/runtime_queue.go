package extractor

import (
	"context"
	"errors"

	"strconv"
	"strings"

	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// HandleSessionMessagePersisted converts the durable-message hook into an extractor turn.
func (r *Runtime) HandleSessionMessagePersisted(
	ctx context.Context,
	payload hookspkg.SessionMessagePersistedPayload,
) error {
	if r == nil {
		return errors.New("memory extractor: runtime is required")
	}
	if ctx == nil {
		return errors.New("memory extractor: hook context is required")
	}
	if strings.TrimSpace(payload.SessionType) == sessionTypeDream ||
		strings.TrimSpace(payload.SessionType) == sessionTypeSystem {
		return nil
	}
	if strings.TrimSpace(payload.ParentSessionID) != "" || strings.TrimSpace(payload.ActorKind) == actorKindSubagent {
		return nil
	}
	seq := payload.MessageSeq
	if seq <= 0 {
		return errors.New("memory extractor: persisted message sequence is required")
	}
	sessionID := firstNonEmpty(payload.SessionID, payload.RootSessionID)
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("memory extractor: persisted message session id is required")
	}
	if r.consumeToolWrite(sessionID, seq) {
		return nil
	}
	role := firstNonEmpty(payload.Role, messageRoleAgent)
	rootSessionID := firstNonEmpty(payload.RootSessionID, sessionID)
	actorKind := firstNonEmpty(payload.ActorKind, actorKindRoot)
	turn := memcontract.TurnRecord{
		SessionID:       sessionID,
		RootSessionID:   rootSessionID,
		AgentID:         firstNonEmpty(payload.AgentName, payload.ActorID, sessionID),
		ActorKind:       actorKind,
		WorkspaceID:     payload.WorkspaceID,
		SinceMessageSeq: seq,
		UntilMessageSeq: seq,
		Snapshot: memcontract.TranscriptSnapshot{
			Messages: []memcontract.TranscriptMessage{{
				Sequence: seq,
				Role:     role,
				Content:  payload.Text,
				At:       payload.Timestamp,
			}},
		},
		Trigger: memcontract.TriggerPostMessage,
	}
	return r.Enqueue(ctx, turn)
}

// RecordToolWrite marks an explicit root-agent memory tool write for turn-level mutual exclusion.
func (r *Runtime) RecordToolWrite(sessionID string, turnSeq int64) {
	if r == nil {
		return
	}
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.toolWrites == nil {
		r.toolWrites = make(map[string]toolWriteMarkers)
	}
	markers := r.toolWrites[trimmed]
	if turnSeq <= 0 {
		markers.consumeNext = true
	} else {
		if markers.sequences == nil {
			markers.sequences = make(map[int64]struct{})
		}
		markers.sequences[turnSeq] = struct{}{}
	}
	r.toolWrites[trimmed] = markers
}

func (r *Runtime) consumeToolWrite(sessionID string, turnSeq int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	markers, exists := r.toolWrites[sessionID]
	if !exists {
		return false
	}
	matched := false
	if turnSeq > 0 {
		if _, ok := markers.sequences[turnSeq]; ok {
			delete(markers.sequences, turnSeq)
			matched = true
		}
		for seq := range markers.sequences {
			if seq < turnSeq {
				delete(markers.sequences, seq)
			}
		}
	}
	if markers.consumeNext {
		markers.consumeNext = false
		matched = true
	}
	if markers.empty() {
		delete(r.toolWrites, sessionID)
	} else {
		r.toolWrites[sessionID] = markers
	}
	return matched
}

// Enqueue requests extraction for one transcript turn using one in-flight plus one queued item per session.
func (r *Runtime) Enqueue(ctx context.Context, turn memcontract.TurnRecord) error {
	if r == nil {
		return errors.New("memory extractor: runtime is required")
	}
	if ctx == nil {
		return errors.New("memory extractor: enqueue context is required")
	}
	req, err := newRequest(turn, r.now)
	if err != nil {
		return err
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return errors.New("memory extractor: runtime is closed")
	}
	if r.draining {
		r.mu.Unlock()
		return errRuntimeDraining
	}
	if !turnHasExtractableContent(req.turn) {
		r.skippedTurns++
		r.mu.Unlock()
		r.recordEvent(ctx, Event{
			Op:   EventDropped,
			Turn: req.turn,
			Metadata: map[string]string{
				"message_count": strconv.Itoa(len(req.turn.Snapshot.Messages)),
				"reason":        "empty_snapshot",
			},
		})
		return nil
	}
	sessionID := req.turn.SessionID
	state := r.sessions[sessionID]
	if state == nil {
		state = &sessionState{}
		r.sessions[sessionID] = state
	}
	if !state.inFlight && state.queued == nil {
		state.inFlight = true
		r.wg.Add(1)
		r.mu.Unlock()
		go r.runSession(sessionID, req)
		return nil
	}
	event := r.queueRequestLocked(state, req)
	if !state.inFlight && state.queued != nil && r.queuedReady(*state.queued) {
		next := *state.queued
		state.queued = nil
		r.stopThrottleFlushLocked(state)
		state.inFlight = true
		r.wg.Add(1)
		r.mu.Unlock()
		r.recordQueuedEvent(ctx, event)
		go r.runSession(sessionID, next)
		return nil
	}
	if !state.inFlight && state.queued != nil {
		r.scheduleThrottleFlushLocked(sessionID, state)
	}
	r.mu.Unlock()
	r.recordQueuedEvent(ctx, event)
	return nil
}

func (r *Runtime) queueRequestLocked(state *sessionState, req request) *Event {
	if state.queued == nil {
		queued := req
		state.queued = &queued
		return nil
	}
	if state.queued.coalesceCount+2 > r.coalesceMax {
		dropped := *state.queued
		queued := req
		state.queued = &queued
		r.droppedTurns++
		return &Event{
			Op:   EventDropped,
			Turn: dropped.turn,
			Metadata: map[string]string{
				"dropped_until_message_seq":  strconv.FormatInt(dropped.turn.UntilMessageSeq, 10),
				"retained_until_message_seq": strconv.FormatInt(req.turn.UntilMessageSeq, 10),
			},
		}
	}
	merged := mergeRequests(*state.queued, req)
	state.queued = &merged
	r.coalescedTurns++
	return &Event{
		Op:   EventCoalesced,
		Turn: merged.turn,
		Metadata: map[string]string{
			"coalesced_count": strconv.Itoa(merged.coalesceCount),
		},
	}
}

func (r *Runtime) recordQueuedEvent(ctx context.Context, event *Event) {
	if event == nil {
		return
	}
	r.recordEvent(ctx, *event)
}

func (r *Runtime) queuedReady(req request) bool {
	return r.throttleTurns <= 1 || req.coalesceCount+1 >= r.throttleTurns
}

func (r *Runtime) scheduleThrottleFlushLocked(sessionID string, state *sessionState) {
	if state == nil || state.flushTimer != nil || r.throttleTurns <= 1 || r.draining {
		return
	}
	state.flushTimer = time.AfterFunc(r.throttleFlushWait, func() {
		r.flushSession(sessionID)
	})
}

func (r *Runtime) stopThrottleFlushLocked(state *sessionState) {
	if state == nil || state.flushTimer == nil {
		return
	}
	state.flushTimer.Stop()
	state.flushTimer = nil
}
