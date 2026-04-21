package transcript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

type EntryKind string

const (
	EntryKindAssistantMessage  EntryKind = "assistant_message"
	EntryKindAssistantThinking EntryKind = "assistant_thinking"
	EntryKindToolCall          EntryKind = "tool_call"
	EntryKindStderrEvent       EntryKind = "stderr_event"
	EntryKindRuntimeNotice     EntryKind = "runtime_notice"
)

type SessionViewSnapshot struct {
	Revision int
	Entries  []Entry
	Plan     SessionPlanState
	Session  SessionMetaState
}

type SessionPlanState struct {
	Entries      []model.SessionPlanEntry
	PendingCount int
	RunningCount int
	DoneCount    int
}

type SessionMetaState struct {
	CurrentModeID     string
	AvailableCommands []model.SessionAvailableCommand
	Status            model.SessionStatus
}

type Entry struct {
	ID            string
	Kind          EntryKind
	Title         string
	Preview       string
	ToolCallID    string
	ToolCallState model.ToolCallState
	Blocks        []model.ContentBlock
}

type ViewModel struct {
	entries        []sessionViewEntry
	planEntries    []model.SessionPlanEntry
	commands       []model.SessionAvailableCommand
	currentModeID  string
	status         model.SessionStatus
	runtimeNoticeN int
	revision       int
}

type sessionViewEntry struct {
	ID            string
	Kind          EntryKind
	ToolCallID    string
	ToolCallState model.ToolCallState
	Blocks        []model.ContentBlock
}

func NewViewModel() *ViewModel {
	return &ViewModel{}
}

func (m *ViewModel) LoadSnapshot(snapshot SessionViewSnapshot) {
	if m == nil {
		return
	}
	m.entries = restoreEntries(snapshot.Entries)
	m.planEntries = clonePlanEntries(snapshot.Plan.Entries)
	m.commands = cloneAvailableCommands(snapshot.Session.AvailableCommands)
	m.currentModeID = snapshot.Session.CurrentModeID
	m.status = snapshot.Session.Status
	m.runtimeNoticeN = countRuntimeNotices(snapshot.Entries)
	m.revision = snapshot.Revision
}

func (m *ViewModel) Apply(update model.SessionUpdate) (SessionViewSnapshot, bool) {
	if !m.apply(update) {
		return SessionViewSnapshot{}, false
	}
	m.revision++
	return m.Snapshot(), true
}

func (m *ViewModel) apply(update model.SessionUpdate) bool {
	changed := m.applyStatus(update.Status)
	if m.applyKind(update) {
		changed = true
	}
	if update.Status == model.StatusFailed && m.appendStatusNotice("Session reported failed status") {
		changed = true
	}
	return changed
}

func (m *ViewModel) applyStatus(status model.SessionStatus) bool {
	if status == "" || m.status == status {
		return false
	}
	m.status = status
	return true
}

func (m *ViewModel) applyKind(update model.SessionUpdate) bool {
	switch update.Kind {
	case model.UpdateKindAgentMessageChunk:
		return m.applyMergedEntry(EntryKindAssistantMessage, update.Blocks)
	case model.UpdateKindAgentThoughtChunk:
		return m.applyMergedEntry(EntryKindAssistantThinking, update.ThoughtBlocks)
	case model.UpdateKindToolCallStarted:
		return m.upsertToolCall(update.ToolCallID, update.ToolCallState, update.Blocks, true)
	case model.UpdateKindToolCallUpdated:
		return m.upsertToolCall(update.ToolCallID, update.ToolCallState, update.Blocks, false)
	case model.UpdateKindPlanUpdated:
		return m.applyPlanEntries(update.PlanEntries)
	case model.UpdateKindAvailableCommandsUpdated:
		return m.applyAvailableCommands(update.AvailableCommands)
	case model.UpdateKindCurrentModeUpdated:
		return m.applyCurrentMode(update.CurrentModeID)
	default:
		return m.appendRuntimeNotice(update.Blocks)
	}
}

func (m *ViewModel) applyPlanEntries(entries []model.SessionPlanEntry) bool {
	if slices.Equal(m.planEntries, entries) {
		return false
	}
	m.planEntries = clonePlanEntries(entries)
	return true
}

func (m *ViewModel) applyAvailableCommands(commands []model.SessionAvailableCommand) bool {
	if slices.Equal(m.commands, commands) {
		return false
	}
	m.commands = cloneAvailableCommands(commands)
	return true
}

func (m *ViewModel) applyCurrentMode(currentModeID string) bool {
	if m.currentModeID == currentModeID {
		return false
	}
	m.currentModeID = currentModeID
	return true
}

func (m *ViewModel) applyMergedEntry(kind EntryKind, blocks []model.ContentBlock) bool {
	if len(blocks) == 0 {
		return false
	}
	if changed, handled := m.mergeIntoLast(kind, blocks); handled {
		return changed
	}
	m.entries = append(m.entries, sessionViewEntry{
		ID:     nextEntryID(kind, len(m.entries)),
		Kind:   kind,
		Blocks: cloneContentBlocks(blocks),
	})
	return true
}

func (m *ViewModel) mergeIntoLast(kind EntryKind, blocks []model.ContentBlock) (bool, bool) {
	if len(m.entries) == 0 || len(blocks) != 1 {
		return false, false
	}
	last := &m.entries[len(m.entries)-1]
	if last.Kind != kind || len(last.Blocks) != 1 {
		return false, false
	}

	merged, ok := mergeTextContentBlocks(last.Blocks[0], blocks[0])
	if !ok {
		return false, false
	}
	if bytes.Equal(last.Blocks[0].Data, merged.Data) {
		return false, true
	}
	last.Blocks[0] = merged
	return true, true
}

func (m *ViewModel) upsertToolCall(
	toolCallID string,
	state model.ToolCallState,
	blocks []model.ContentBlock,
	started bool,
) bool {
	if state == model.ToolCallStateUnknown && started {
		state = model.ToolCallStatePending
	}
	if idx := m.findToolEntry(toolCallID); idx >= 0 {
		entry := &m.entries[idx]
		changed := false
		if state != model.ToolCallStateUnknown && entry.ToolCallState != state {
			entry.ToolCallState = state
			changed = true
		}
		if started {
			if len(blocks) > 0 {
				nextBlocks := cloneContentBlocks(blocks)
				if !contentBlocksEqual(entry.Blocks, nextBlocks) {
					entry.Blocks = nextBlocks
					changed = true
				}
			}
		} else if len(blocks) > 0 {
			header := mergeToolUseHeaders(extractToolUseHeader(entry.Blocks), extractToolUseHeader(blocks))
			content := stripToolUseHeader(blocks)
			if len(content) == 0 {
				content = stripToolUseHeader(entry.Blocks)
			}
			nextBlocks := make([]model.ContentBlock, 0, len(header)+len(content))
			nextBlocks = append(nextBlocks, header...)
			nextBlocks = append(nextBlocks, content...)
			if !contentBlocksEqual(entry.Blocks, nextBlocks) {
				entry.Blocks = nextBlocks
				changed = true
			}
		}
		return changed
	}

	if !started {
		placeholder, err := missingToolCallBlocks(toolCallID)
		if err == nil {
			m.entries = append(m.entries, sessionViewEntry{
				ID:            nextEntryID(EntryKindToolCall, len(m.entries)),
				Kind:          EntryKindToolCall,
				ToolCallID:    toolCallID,
				ToolCallState: model.ToolCallStateFailed,
				Blocks:        placeholder,
			})
			return true
		}
	}

	if len(blocks) == 0 {
		return false
	}
	m.entries = append(m.entries, sessionViewEntry{
		ID:            nextEntryID(EntryKindToolCall, len(m.entries)),
		Kind:          EntryKindToolCall,
		ToolCallID:    toolCallID,
		ToolCallState: state,
		Blocks:        cloneContentBlocks(blocks),
	})
	return true
}

func (m *ViewModel) appendRuntimeNotice(blocks []model.ContentBlock) bool {
	if len(blocks) == 0 {
		return false
	}
	m.runtimeNoticeN++
	m.entries = append(m.entries, sessionViewEntry{
		ID:     fmt.Sprintf("runtime-%d", m.runtimeNoticeN),
		Kind:   EntryKindRuntimeNotice,
		Blocks: cloneContentBlocks(blocks),
	})
	return true
}

func (m *ViewModel) appendStatusNotice(text string) bool {
	block, err := model.NewContentBlock(model.TextBlock{Text: text})
	if err != nil {
		return false
	}
	if len(m.entries) > 0 {
		last := &m.entries[len(m.entries)-1]
		if last.Kind == EntryKindRuntimeNotice && len(last.Blocks) == 1 {
			if existing, blockErr := last.Blocks[0].AsText(); blockErr == nil && existing.Text == text {
				return false
			}
		}
	}
	m.runtimeNoticeN++
	m.entries = append(m.entries, sessionViewEntry{
		ID:     fmt.Sprintf("runtime-%d", m.runtimeNoticeN),
		Kind:   EntryKindRuntimeNotice,
		Blocks: []model.ContentBlock{block},
	})
	return true
}

func (m *ViewModel) findToolEntry(toolCallID string) int {
	if toolCallID == "" {
		return -1
	}
	for i := range m.entries {
		if m.entries[i].ToolCallID == toolCallID {
			return i
		}
	}
	return -1
}

func (m *ViewModel) Snapshot() SessionViewSnapshot {
	entries := buildVisibleEntries(m.entries)
	return SessionViewSnapshot{
		Revision: m.revision,
		Entries:  entries,
		Plan:     buildPlanState(m.planEntries),
		Session: SessionMetaState{
			CurrentModeID:     m.currentModeID,
			AvailableCommands: cloneAvailableCommands(m.commands),
			Status:            m.status,
		},
	}
}

func buildPlanState(entries []model.SessionPlanEntry) SessionPlanState {
	state := SessionPlanState{Entries: clonePlanEntries(entries)}
	for _, entry := range entries {
		switch entry.Status {
		case "completed":
			state.DoneCount++
		case "in_progress":
			state.RunningCount++
		default:
			state.PendingCount++
		}
	}
	return state
}

func buildVisibleEntries(entries []sessionViewEntry) []Entry {
	if len(entries) == 0 {
		return nil
	}

	visible := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		visible = append(visible, buildVisibleEntry(entry))
	}

	return visible
}

func restoreEntries(entries []Entry) []sessionViewEntry {
	if len(entries) == 0 {
		return nil
	}
	restored := make([]sessionViewEntry, 0, len(entries))
	for _, entry := range entries {
		restored = append(restored, sessionViewEntry{
			ID:            entry.ID,
			Kind:          entry.Kind,
			ToolCallID:    entry.ToolCallID,
			ToolCallState: entry.ToolCallState,
			Blocks:        cloneContentBlocks(entry.Blocks),
		})
	}
	return restored
}

func countRuntimeNotices(entries []Entry) int {
	total := 0
	for _, entry := range entries {
		if entry.Kind == EntryKindRuntimeNotice {
			total++
		}
	}
	return total
}

func buildVisibleEntry(entry sessionViewEntry) Entry {
	result := Entry{
		ID:            entry.ID,
		Kind:          entry.Kind,
		ToolCallID:    entry.ToolCallID,
		ToolCallState: entry.ToolCallState,
		Blocks:        cloneContentBlocks(entry.Blocks),
	}

	switch entry.Kind {
	case EntryKindAssistantMessage:
		result.Title = "Assistant"
		result.Preview = buildBlocksPreview(entry.Blocks)
	case EntryKindAssistantThinking:
		result.Title = "Thinking"
		result.Preview = buildBlocksPreview(entry.Blocks)
	case EntryKindToolCall:
		result.Title = extractToolTitle(entry.Blocks)
		result.Preview = buildToolCallPreview(entry.Blocks)
	case EntryKindStderrEvent:
		result.Title = "stderr"
		result.Preview = buildBlocksPreview(entry.Blocks)
	case EntryKindRuntimeNotice:
		result.Title = "Runtime"
		result.Preview = buildBlocksPreview(entry.Blocks)
	default:
		result.Title = "Entry"
		result.Preview = buildBlocksPreview(entry.Blocks)
	}
	return result
}

func buildBlocksPreview(blocks []model.ContentBlock) string {
	for _, block := range blocks {
		switch block.Type {
		case model.BlockText:
			textBlock, err := block.AsText()
			if err == nil {
				return truncateSingleLine(textBlock.Text)
			}
		case model.BlockToolUse:
			continue
		case model.BlockToolResult:
			toolResult, err := block.AsToolResult()
			if err == nil {
				return truncateSingleLine(toolResult.Content)
			}
		case model.BlockDiff:
			diffBlock, err := block.AsDiff()
			if err == nil {
				return truncateSingleLine("diff " + diffBlock.FilePath)
			}
		case model.BlockTerminalOutput:
			terminalBlock, err := block.AsTerminalOutput()
			if err == nil {
				if terminalBlock.Command != "" {
					return truncateSingleLine("$ " + terminalBlock.Command)
				}
				return truncateSingleLine(terminalBlock.Output)
			}
		case model.BlockImage:
			imageBlock, err := block.AsImage()
			if err == nil {
				return truncateSingleLine("image " + imageBlock.MimeType)
			}
		}
	}
	return ""
}

func extractToolTitle(blocks []model.ContentBlock) string {
	if len(blocks) > 0 && blocks[0].Type == model.BlockToolUse {
		toolUse, err := blocks[0].AsToolUse()
		if err == nil {
			return toolUseDisplayTitle(toolUse)
		}
	}
	return toolNameGeneric
}

func buildToolCallPreview(blocks []model.ContentBlock) string {
	if len(blocks) > 0 && blocks[0].Type == model.BlockToolUse {
		toolUse, err := blocks[0].AsToolUse()
		if err == nil {
			if summary := toolUseSummary(toolUse); summary != "" {
				return summary
			}
		}
	}
	return buildBlocksPreview(stripToolUseHeader(blocks))
}

func cloneContentBlocks(blocks []model.ContentBlock) []model.ContentBlock {
	if len(blocks) == 0 {
		return nil
	}
	// ContentBlock.Data is immutable once constructed, so snapshots only need a
	// slice clone to prevent entry-level slice mutation from leaking.
	return slices.Clone(blocks)
}

func clonePlanEntries(entries []model.SessionPlanEntry) []model.SessionPlanEntry {
	if len(entries) == 0 {
		return nil
	}
	return slices.Clone(entries)
}

func cloneAvailableCommands(commands []model.SessionAvailableCommand) []model.SessionAvailableCommand {
	if len(commands) == 0 {
		return nil
	}
	return slices.Clone(commands)
}

func truncateSingleLine(text string) string {
	lines := splitRenderedText(text)
	if len(lines) == 0 {
		return ""
	}
	return truncateString(strings.TrimSpace(lines[0]), 96)
}

func nextEntryID(kind EntryKind, index int) string {
	return fmt.Sprintf("%s-%d", kind, index+1)
}

func extractToolUseHeader(blocks []model.ContentBlock) []model.ContentBlock {
	if len(blocks) == 0 || blocks[0].Type != model.BlockToolUse {
		return nil
	}
	return cloneContentBlocks(blocks[:1])
}

func stripToolUseHeader(blocks []model.ContentBlock) []model.ContentBlock {
	if len(blocks) == 0 {
		return nil
	}
	if blocks[0].Type != model.BlockToolUse {
		return cloneContentBlocks(blocks)
	}
	return cloneContentBlocks(blocks[1:])
}

func mergeToolUseHeaders(existing []model.ContentBlock, updated []model.ContentBlock) []model.ContentBlock {
	if len(updated) == 0 {
		return existing
	}
	if len(existing) == 0 {
		return updated
	}

	existingTool, err := existing[0].AsToolUse()
	if err != nil {
		return updated
	}
	updatedTool, err := updated[0].AsToolUse()
	if err != nil {
		return updated
	}

	merged := existingTool
	if name := strings.TrimSpace(updatedTool.Name); name != "" && name != "Tool Call" {
		merged.Name = name
	}
	if title := strings.TrimSpace(updatedTool.Title); title != "" && title != toolNameGeneric {
		merged.Title = title
	}
	if toolName := strings.TrimSpace(updatedTool.ToolName); toolName != "" {
		merged.ToolName = toolName
	}
	if input := mergeToolUseInputJSON(existingTool.Input, updatedTool.Input); len(input) > 0 {
		merged.Input = input
	}
	if rawInput := cloneRawJSON(updatedTool.RawInput); len(rawInput) > 0 {
		merged.RawInput = rawInput
	}

	block, err := model.NewContentBlock(merged)
	if err != nil {
		return updated
	}
	return []model.ContentBlock{block}
}

func mergeToolUseInputJSON(existing json.RawMessage, updated json.RawMessage) json.RawMessage {
	if len(updated) == 0 {
		return append(json.RawMessage(nil), existing...)
	}
	if len(existing) == 0 {
		return append(json.RawMessage(nil), updated...)
	}

	existingMap, existingIsObject := decodeJSONObjectPayload(existing)
	updatedMap, updatedIsObject := decodeJSONObjectPayload(updated)
	switch {
	case !updatedIsObject:
		if isJSONNullPayload(updated) {
			return append(json.RawMessage(nil), existing...)
		}
		return append(json.RawMessage(nil), updated...)
	case !existingIsObject:
		return append(json.RawMessage(nil), updated...)
	}
	for key, value := range updatedMap {
		existingMap[key] = value
	}
	merged, err := json.Marshal(existingMap)
	if err != nil {
		return append(json.RawMessage(nil), updated...)
	}
	return merged
}

func decodeJSONObjectPayload(payload json.RawMessage) (map[string]any, bool) {
	if len(payload) == 0 {
		return nil, false
	}

	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, false
	}
	record, ok := decoded.(map[string]any)
	if !ok {
		return nil, false
	}
	return record, true
}

func isJSONNullPayload(payload json.RawMessage) bool {
	trimmed := bytes.TrimSpace(payload)
	return bytes.Equal(trimmed, []byte("null"))
}

func cloneRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func missingToolCallBlocks(toolCallID string) ([]model.ContentBlock, error) {
	header, err := model.NewContentBlock(model.ToolUseBlock{
		ID:    toolCallID,
		Name:  toolNameGeneric,
		Title: "Tool call not found",
	})
	if err != nil {
		return nil, fmt.Errorf("creating placeholder blocks: %w", err)
	}
	result, err := model.NewContentBlock(model.ToolResultBlock{
		ToolUseID: toolCallID,
		Content:   "Tool call not found",
		IsError:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("creating placeholder blocks: %w", err)
	}
	return []model.ContentBlock{header, result}, nil
}

func mergeTextContentBlocks(existing, incoming model.ContentBlock) (model.ContentBlock, bool) {
	if existing.Type != model.BlockText || incoming.Type != model.BlockText {
		return model.ContentBlock{}, false
	}

	existingText, err := existing.AsText()
	if err != nil {
		return model.ContentBlock{}, false
	}
	incomingText, err := incoming.AsText()
	if err != nil {
		return model.ContentBlock{}, false
	}

	merged, err := model.NewContentBlock(model.TextBlock{
		Text: mergeNarrativeText(existingText.Text, incomingText.Text),
	})
	if err != nil {
		return model.ContentBlock{}, false
	}
	return merged, true
}

func mergeNarrativeText(existing, incoming string) string {
	switch {
	case incoming == "":
		return existing
	case existing == "":
		return incoming
	case incoming == existing:
		return existing
	case strings.HasPrefix(incoming, existing):
		return incoming
	}

	existingRunes := []rune(existing)
	incomingRunes := []rune(incoming)
	if overlap := longestSuffixPrefixOverlap(existingRunes, incomingRunes); overlap > 0 {
		return string(existingRunes) + string(incomingRunes[overlap:])
	}

	if shouldReplaceNarrativeText(existingRunes, incomingRunes) {
		return incoming
	}

	return existing + incoming
}

func shouldReplaceNarrativeText(existing, incoming []rune) bool {
	shorter := min(len(existing), len(incoming))
	if shorter == 0 {
		return false
	}

	prefix := commonPrefixLength(existing, incoming)
	switch {
	case prefix == 0:
		return false
	case prefix == shorter:
		return shorter >= 8
	case shorter < 12:
		return false
	default:
		return prefix >= 12 && prefix*4 >= shorter
	}
}

func commonPrefixLength(left, right []rune) int {
	limit := min(len(left), len(right))
	for i := range limit {
		if left[i] != right[i] {
			return i
		}
	}
	return limit
}

func longestSuffixPrefixOverlap(existing, incoming []rune) int {
	limit := min(len(existing), len(incoming))
	for size := limit; size > 0; size-- {
		if slices.Equal(existing[len(existing)-size:], incoming[:size]) {
			return size
		}
	}
	return 0
}

func contentBlocksEqual(left, right []model.ContentBlock) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Type != right[i].Type {
			return false
		}
		if !bytes.Equal(left[i].Data, right[i].Data) {
			return false
		}
	}
	return true
}

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen == 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}
