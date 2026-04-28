package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
)

const (
	runUIToolNameGeneric = "tool"
	runUIStateDone       = "done"
)

var runUIEmptyObject = json.RawMessage(`{}`)

type runUIToolUseBlock struct {
	Type     string          `json:"type"`
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Title    string          `json:"title,omitempty"`
	ToolName string          `json:"toolName,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	RawInput json.RawMessage `json:"rawInput,omitempty"`
}

type runUIToolResultBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"toolUseId"`
	Content   string `json:"content"`
	IsError   bool   `json:"isError,omitempty"`
}

type runUITextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type runUIBlockPayload struct {
	EntryKind     apicore.SessionEntryKind `json:"entry_kind"`
	EntryID       string                   `json:"entry_id"`
	Title         string                   `json:"title,omitempty"`
	Preview       string                   `json:"preview,omitempty"`
	ToolCallID    string                   `json:"tool_call_id,omitempty"`
	ToolCallState apicore.ToolCallState    `json:"tool_call_state,omitempty"`
	Block         json.RawMessage          `json:"block"`
}

type runUIEventPayload struct {
	Type          string                   `json:"type"`
	EntryKind     apicore.SessionEntryKind `json:"entry_kind"`
	EntryID       string                   `json:"entry_id"`
	Title         string                   `json:"title,omitempty"`
	Text          string                   `json:"text,omitempty"`
	ToolCallID    string                   `json:"tool_call_id,omitempty"`
	ToolCallState apicore.ToolCallState    `json:"tool_call_state,omitempty"`
	Blocks        []apicore.ContentBlock   `json:"blocks,omitempty"`
}

type runUIToolOutputPayload struct {
	Blocks  []apicore.ContentBlock `json:"blocks,omitempty"`
	Summary string                 `json:"summary,omitempty"`
	IsError bool                   `json:"is_error,omitempty"`
}

// Transcript returns the canonical structured transcript for one run.
func (m *RunManager) Transcript(ctx context.Context, runID string) (apicore.RunTranscript, error) {
	listCtx := detachContext(ctx)
	row, err := m.globalDB.GetRun(listCtx, strings.TrimSpace(runID))
	if err != nil {
		return apicore.RunTranscript{}, err
	}
	runView, err := m.toCoreRun(listCtx, row, "")
	if err != nil {
		return apicore.RunTranscript{}, err
	}

	lease, err := m.acquireRunDB(listCtx, row.RunID)
	if err != nil {
		return apicore.RunTranscript{}, err
	}
	defer func() {
		_ = lease.Close()
	}()
	runDB := lease.DB()

	sessionEvents, err := runDB.ListCompactedSessionUpdateEvents(listCtx)
	if err != nil {
		return apicore.RunTranscript{}, err
	}
	lastEvent, err := runDB.LastEvent(listCtx)
	if err != nil {
		return apicore.RunTranscript{}, err
	}
	if err := m.persistRuntimeIntegrity(listCtx, row.RunID, m.scopeForRun(row.RunID)); err != nil {
		slog.Default().Warn("daemon transcript runtime integrity persistence failed", "run_id", row.RunID, "error", err)
	}
	integrity, err := m.loadCompactRunIntegrity(listCtx, row.RunID, runView, runDB, lastEvent)
	if err != nil {
		return apicore.RunTranscript{}, err
	}

	builder := newRunSnapshotBuilder()
	for _, item := range sessionEvents {
		if err := builder.applyEvent(item); err != nil {
			return apicore.RunTranscript{}, err
		}
	}

	session := aggregateRunTranscriptSession(builder.jobStates())
	var nextCursor *apicore.StreamCursor
	if lastEvent != nil {
		cursor := apicore.CursorFromEvent(*lastEvent)
		nextCursor = &cursor
	}
	return apicore.RunTranscript{
		RunID:             runView.RunID,
		Messages:          runUIMessagesFromSession(session),
		Session:           session,
		Incomplete:        integrity.Incomplete,
		IncompleteReasons: append([]string(nil), integrity.Reasons...),
		NextCursor:        nextCursor,
	}, nil
}

func aggregateRunTranscriptSession(jobs []apicore.RunJobState) apicore.SessionViewSnapshot {
	var result apicore.SessionViewSnapshot
	for _, job := range jobs {
		if job.Summary == nil {
			continue
		}
		session := job.Summary.Session
		if session.Revision > result.Revision {
			result.Revision = session.Revision
		}
		result.Plan.PendingCount += session.Plan.PendingCount
		result.Plan.RunningCount += session.Plan.RunningCount
		result.Plan.DoneCount += session.Plan.DoneCount
		result.Plan.Entries = append(result.Plan.Entries, session.Plan.Entries...)
		if result.Session.Status == "" {
			result.Session.Status = session.Session.Status
		}
		if result.Session.CurrentModeID == "" {
			result.Session.CurrentModeID = session.Session.CurrentModeID
		}
		result.Session.AvailableCommands = append(
			result.Session.AvailableCommands,
			session.Session.AvailableCommands...)
		for _, entry := range session.Entries {
			next := entry
			next.ID = fmt.Sprintf("job-%d-%s", job.Index, strings.TrimSpace(entry.ID))
			if next.Title == "" {
				next.Title = strings.TrimSpace(job.JobID)
			}
			result.Entries = append(result.Entries, next)
		}
	}
	return result
}

func runUIMessagesFromSession(session apicore.SessionViewSnapshot) []apicore.RunUIMessage {
	if len(session.Entries) == 0 {
		return []apicore.RunUIMessage{}
	}
	messages := make([]apicore.RunUIMessage, 0, len(session.Entries))
	for _, entry := range session.Entries {
		if message, ok := runUIMessageFromSessionEntry(entry); ok {
			messages = append(messages, message)
		}
	}
	return messages
}

func runUIMessageFromSessionEntry(entry apicore.SessionEntry) (apicore.RunUIMessage, bool) {
	parts := runUIMessagePartsFromEntry(entry)
	if len(parts) == 0 {
		return apicore.RunUIMessage{}, false
	}
	return apicore.RunUIMessage{
		ID:    strings.TrimSpace(entry.ID),
		Role:  contract.RunUIMessageRoleAssistant,
		Parts: parts,
	}, true
}

func runUIMessagePartsFromEntry(entry apicore.SessionEntry) []apicore.RunUIMessagePart {
	switch entry.Kind {
	case contract.SessionEntryKindAssistantMessage:
		return runUITextPartsFromBlocks(entry.Blocks)
	case contract.SessionEntryKindAssistantThinking:
		return runUIReasoningPartsFromBlocks(entry.Blocks)
	case contract.SessionEntryKindToolCall:
		return []apicore.RunUIMessagePart{runUIToolPartFromEntry(entry)}
	case contract.SessionEntryKindRuntimeNotice, contract.SessionEntryKindStderrEvent:
		return []apicore.RunUIMessagePart{runUIEventPartFromEntry(entry)}
	default:
		return runUIBlockPartsFromEntry(entry)
	}
}

func runUITextPartsFromBlocks(blocks []apicore.ContentBlock) []apicore.RunUIMessagePart {
	parts := make([]apicore.RunUIMessagePart, 0, len(blocks))
	for _, block := range blocks {
		if text, ok := runUITextFromBlock(block); ok {
			parts = append(parts, apicore.RunUIMessagePart{
				Type:  contract.RunUIMessagePartText,
				Text:  text,
				State: runUIStateDone,
			})
			continue
		}
		parts = append(parts, runUIBlockPart("", contract.SessionEntryKindAssistantMessage, block))
	}
	return parts
}

func runUIReasoningPartsFromBlocks(blocks []apicore.ContentBlock) []apicore.RunUIMessagePart {
	parts := make([]apicore.RunUIMessagePart, 0, len(blocks))
	for _, block := range blocks {
		text, ok := runUITextFromBlock(block)
		if !ok {
			raw, err := json.Marshal(block)
			if err != nil {
				continue
			}
			text = string(raw)
		}
		parts = append(parts, apicore.RunUIMessagePart{
			Type:  contract.RunUIMessagePartReasoning,
			Text:  text,
			State: runUIStateDone,
		})
	}
	return parts
}

func runUIBlockPartsFromEntry(entry apicore.SessionEntry) []apicore.RunUIMessagePart {
	parts := make([]apicore.RunUIMessagePart, 0, len(entry.Blocks))
	for _, block := range entry.Blocks {
		parts = append(parts, runUIBlockPart(entry.ID, entry.Kind, block))
	}
	return parts
}

func runUIToolPartFromEntry(entry apicore.SessionEntry) apicore.RunUIMessagePart {
	toolUse := firstToolUseBlock(entry.Blocks)
	toolCallID := firstNonEmpty(entry.ToolCallID, toolUse.ID, entry.ID)
	toolName := firstNonEmpty(toolUse.ToolName, toolUse.Name, runUIToolNameGeneric)
	input := cloneRaw(toolUse.Input)
	if len(input) == 0 {
		input = cloneRaw(runUIEmptyObject)
	}
	output, outputErr := runUIToolOutput(entry)
	part := apicore.RunUIMessagePart{
		Type:       contract.RunUIMessagePartDynamicTool,
		ToolName:   toolName,
		ToolCallID: toolCallID,
		Title:      firstNonEmpty(toolUse.Title, entry.Title, toolName),
		Input:      input,
		RawInput:   cloneRaw(toolUse.RawInput),
	}
	switch entry.ToolCallState {
	case apicore.ToolCallState("pending"), apicore.ToolCallState("in_progress"):
		part.State = string(contract.RunUIToolPartStateInputStreaming)
	case apicore.ToolCallState("waiting_for_confirmation"):
		part.State = string(contract.RunUIToolPartStateApprovalRequested)
		part.Output = nil
	case apicore.ToolCallState("failed"):
		part.State = string(contract.RunUIToolPartStateOutputError)
		part.ErrorText = firstNonEmpty(outputErr, entry.Preview, "Tool call failed")
	default:
		if len(output) > 0 {
			part.State = string(contract.RunUIToolPartStateOutputAvailable)
			part.Output = output
		} else {
			part.State = string(contract.RunUIToolPartStateInputAvailable)
		}
	}
	return part
}

func runUIEventPartFromEntry(entry apicore.SessionEntry) apicore.RunUIMessagePart {
	payload := runUIEventPayload{
		Type:          string(entry.Kind),
		EntryKind:     entry.Kind,
		EntryID:       entry.ID,
		Title:         entry.Title,
		Text:          entry.Preview,
		ToolCallID:    entry.ToolCallID,
		ToolCallState: entry.ToolCallState,
		Blocks:        entry.Blocks,
	}
	return apicore.RunUIMessagePart{
		Type: contract.RunUIMessagePartCompozyEvent,
		ID:   entry.ID,
		Data: mustJSONRaw(payload),
	}
}

func runUIBlockPart(
	entryID string,
	kind apicore.SessionEntryKind,
	block apicore.ContentBlock,
) apicore.RunUIMessagePart {
	raw, err := json.Marshal(block)
	if err != nil {
		raw = mustJSONRaw(map[string]string{
			"type":  string(block.Type),
			"error": err.Error(),
		})
	}
	payload := runUIBlockPayload{
		EntryKind: kind,
		EntryID:   entryID,
		Block:     raw,
	}
	return apicore.RunUIMessagePart{
		Type: contract.RunUIMessagePartCompozyBlock,
		Data: mustJSONRaw(payload),
	}
}

func runUITextFromBlock(block apicore.ContentBlock) (string, bool) {
	if string(block.Type) != "text" {
		return "", false
	}
	var payload runUITextBlock
	if err := json.Unmarshal(block.Data, &payload); err != nil {
		return "", false
	}
	if strings.TrimSpace(payload.Text) == "" {
		return "", false
	}
	return payload.Text, true
}

func firstToolUseBlock(blocks []apicore.ContentBlock) runUIToolUseBlock {
	for _, block := range blocks {
		if string(block.Type) != "tool_use" {
			continue
		}
		var payload runUIToolUseBlock
		if err := json.Unmarshal(block.Data, &payload); err == nil {
			return payload
		}
	}
	return runUIToolUseBlock{}
}

func runUIToolOutput(entry apicore.SessionEntry) (json.RawMessage, string) {
	outputBlocks := make([]apicore.ContentBlock, 0, len(entry.Blocks))
	var summary string
	var errorText string
	isError := false
	for _, block := range entry.Blocks {
		if string(block.Type) == "tool_use" {
			continue
		}
		outputBlocks = append(outputBlocks, block)
		if string(block.Type) != "tool_result" {
			if summary == "" {
				summary = entry.Preview
			}
			continue
		}
		var result runUIToolResultBlock
		if err := json.Unmarshal(block.Data, &result); err != nil {
			continue
		}
		if summary == "" {
			summary = strings.TrimSpace(result.Content)
		}
		if result.IsError {
			isError = true
			errorText = strings.TrimSpace(result.Content)
		}
	}
	if len(outputBlocks) == 0 {
		return nil, errorText
	}
	output := runUIToolOutputPayload{
		Blocks:  outputBlocks,
		Summary: firstNonEmpty(summary, entry.Preview),
		IsError: isError,
	}
	return mustJSONRaw(output), errorText
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func mustJSONRaw(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{"type":"serialization_error"}`)
	}
	return raw
}
