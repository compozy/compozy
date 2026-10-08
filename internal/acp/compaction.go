package acp

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/redact"
)

const EventTypeCompaction = "compaction"
const compactionSummaryLimit = 16 * 1024
const compactionTruncationMark = " [summary truncated]"
const compactionStatusCancelled = "cancelled" //nolint:misspell // ACP wire spelling.

type CompactionObservation struct {
	CompactionID string
	Status       string
	Summary      string
	Error        string
	Terminal     bool
}

type compactionState struct {
	snapshot     CompactionObservation
	buffer       string
	truncated    bool
	terminalSeen bool
	bufferDirty  bool
}

func (p *AgentProcess) handleCompactionUpdate(raw wireSessionNotification, kind string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw.Update, &fields); err != nil {
		return fmt.Errorf("acp: decode compaction update: %w", err)
	}
	var id string
	if err := json.Unmarshal(fields["compactionId"], &id); err != nil || id == "" {
		p.emitRawSystemUpdate(raw, kind)
		return nil
	}
	p.compactionMu.Lock()
	defer p.compactionMu.Unlock()
	if p.compactions == nil {
		p.compactions = make(map[string]*compactionState)
	}
	state, exists := p.compactions[id]
	if !exists {
		state = &compactionState{snapshot: CompactionObservation{CompactionID: id}}
		p.compactions[id] = state
	}
	before := state.snapshot
	if kind == "compaction_summary_chunk" {
		if isCompactionTerminal(state.snapshot.Status) {
			if p.logger != nil {
				p.logger.Warn("acp.compaction.late_chunk", "session_id", raw.SessionID, "compaction_id", redact.ClaimTokens(id))
			}
			return nil
		}
		state.appendSummary(compactionText(fields["content"]))
		state.bufferDirty = true
		// A chunk-first frame establishes the id without exposing partial summary text.
		if exists {
			return nil
		}
	} else {
		state.applyPatch(fields)
	}
	if exists && before.Status == state.snapshot.Status &&
		before.Summary == state.snapshot.Summary && before.Error == state.snapshot.Error {
		return nil
	}
	snapshot := state.snapshot
	snapshot.CompactionID = redact.ClaimTokens(snapshot.CompactionID)
	snapshot.Status = redact.ClaimTokens(snapshot.Status)
	snapshot.Terminal = isCompactionTerminal(state.snapshot.Status) && !state.terminalSeen
	if snapshot.Terminal {
		state.terminalSeen = true
	}
	p.emitPromptEvent(AgentEvent{
		Type: EventTypeCompaction, SessionID: string(raw.SessionID), TurnID: p.activeTurnID(),
		Timestamp: timeNowUTC(), Compaction: &snapshot,
	})
	return nil
}

func (s *compactionState) applyPatch(fields map[string]json.RawMessage) {
	if status, ok := fields["status"]; ok {
		if string(status) == "null" {
			s.snapshot.Status = ""
		} else {
			var value string
			if json.Unmarshal(status, &value) == nil {
				s.snapshot.Status = redact.ClaimTokens(value)
			}
		}
	}
	if summary, ok := fields["summary"]; ok {
		s.buffer = ""
		s.truncated = false
		s.appendSummary(compactionText(summary))
		s.snapshot.Summary = s.summary()
		s.bufferDirty = false
	} else if isCompactionTerminal(s.snapshot.Status) && (!s.terminalSeen || s.bufferDirty) {
		s.snapshot.Summary = s.summary()
		s.bufferDirty = false
	}
	if failure, ok := fields["error"]; ok {
		if string(failure) == "null" {
			s.snapshot.Error = ""
		} else {
			var value string
			if json.Unmarshal(failure, &value) == nil {
				s.snapshot.Error = redact.ClaimTokens(value)
			}
		}
	}
}

func (s *compactionState) appendSummary(text string) {
	if s.truncated {
		return
	}
	available := compactionSummaryLimit - len(s.buffer)
	if len(text) > available {
		text = text[:available]
		for !utf8.ValidString(text) && len(text) > 0 {
			text = text[:len(text)-1]
		}
		s.truncated = true
	}
	s.buffer += text
}

func (s *compactionState) summary() string {
	text := redact.ClaimTokens(s.buffer)
	if s.truncated || len(text) > compactionSummaryLimit {
		end := min(len(text), compactionSummaryLimit-len(compactionTruncationMark))
		text = text[:end]
		for !utf8.ValidString(text) && len(text) > 0 {
			text = text[:len(text)-1]
		}
		text += compactionTruncationMark
	}
	return text
}

func isCompactionTerminal(status string) bool {
	return status == "completed" || status == "failed" || status == compactionStatusCancelled
}

func compactionText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var blocks []json.RawMessage
	if json.Unmarshal(raw, &blocks) == nil && blocks != nil {
		var result strings.Builder
		for _, block := range blocks {
			result.WriteString(compactionText(block))
		}
		return result.String()
	}
	var block struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &block) == nil && (block.Type == "text" || block.Type == "") {
		return block.Text
	}
	return ""
}
