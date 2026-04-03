package run

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

type transcriptEntryKind string

const (
	transcriptEntryAssistantMessage  transcriptEntryKind = "assistant_message"
	transcriptEntryAssistantThinking transcriptEntryKind = "assistant_thinking"
	transcriptEntryToolCall          transcriptEntryKind = "tool_call"
	transcriptEntryStderrEvent       transcriptEntryKind = "stderr_event"
	transcriptEntryRuntimeNotice     transcriptEntryKind = "runtime_notice"
)

type SessionViewSnapshot struct {
	Revision int
	Entries  []TranscriptEntry
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

type TranscriptEntry struct {
	ID            string
	Kind          transcriptEntryKind
	Title         string
	Preview       string
	ToolCallID    string
	ToolCallState model.ToolCallState
	Blocks        []model.ContentBlock
}

type sessionViewModel struct {
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
	Kind          transcriptEntryKind
	ToolCallID    string
	ToolCallState model.ToolCallState
	Blocks        []model.ContentBlock
}

func newSessionViewModel() *sessionViewModel {
	return &sessionViewModel{}
}

func (m *sessionViewModel) Apply(update model.SessionUpdate) (SessionViewSnapshot, bool) {
	if !m.apply(update) {
		return SessionViewSnapshot{}, false
	}
	m.revision++
	return m.snapshot(), true
}

func (m *sessionViewModel) apply(update model.SessionUpdate) bool {
	changed := m.applyStatus(update.Status)
	if m.applyKind(update) {
		changed = true
	}
	if update.Status == model.StatusFailed && m.appendStatusNotice("Session reported failed status") {
		changed = true
	}
	return changed
}

func (m *sessionViewModel) applyStatus(status model.SessionStatus) bool {
	if status == "" || m.status == status {
		return false
	}
	m.status = status
	return true
}

func (m *sessionViewModel) applyKind(update model.SessionUpdate) bool {
	switch update.Kind {
	case model.UpdateKindAgentMessageChunk:
		return m.applyMergedEntry(transcriptEntryAssistantMessage, update.Blocks)
	case model.UpdateKindAgentThoughtChunk:
		return m.applyMergedEntry(transcriptEntryAssistantThinking, update.ThoughtBlocks)
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

func (m *sessionViewModel) applyPlanEntries(entries []model.SessionPlanEntry) bool {
	if slices.Equal(m.planEntries, entries) {
		return false
	}
	m.planEntries = clonePlanEntries(entries)
	return true
}

func (m *sessionViewModel) applyAvailableCommands(commands []model.SessionAvailableCommand) bool {
	if slices.Equal(m.commands, commands) {
		return false
	}
	m.commands = cloneAvailableCommands(commands)
	return true
}

func (m *sessionViewModel) applyCurrentMode(currentModeID string) bool {
	if m.currentModeID == currentModeID {
		return false
	}
	m.currentModeID = currentModeID
	return true
}

func (m *sessionViewModel) applyMergedEntry(kind transcriptEntryKind, blocks []model.ContentBlock) bool {
	if len(blocks) == 0 {
		return false
	}
	if merged := m.mergeIntoLast(kind, blocks); merged {
		return true
	}
	m.entries = append(m.entries, sessionViewEntry{
		ID:     nextEntryID(kind, len(m.entries)),
		Kind:   kind,
		Blocks: cloneContentBlocks(blocks),
	})
	return true
}

func (m *sessionViewModel) mergeIntoLast(kind transcriptEntryKind, blocks []model.ContentBlock) bool {
	if len(m.entries) == 0 || len(blocks) != 1 {
		return false
	}
	last := &m.entries[len(m.entries)-1]
	if last.Kind != kind || len(last.Blocks) != 1 {
		return false
	}

	merged, ok := mergeTextContentBlocks(last.Blocks[0], blocks[0])
	if !ok {
		return false
	}
	last.Blocks[0] = merged
	return true
}

func (m *sessionViewModel) upsertToolCall(
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
			header := extractToolUseHeader(entry.Blocks)
			nextBlocks := make([]model.ContentBlock, 0, len(header)+len(blocks))
			nextBlocks = append(nextBlocks, header...)
			nextBlocks = append(nextBlocks, cloneContentBlocks(blocks)...)
			if !contentBlocksEqual(entry.Blocks, nextBlocks) {
				entry.Blocks = nextBlocks
				changed = true
			}
		}
		return changed
	}

	if len(blocks) == 0 {
		return false
	}
	m.entries = append(m.entries, sessionViewEntry{
		ID:            nextEntryID(transcriptEntryToolCall, len(m.entries)),
		Kind:          transcriptEntryToolCall,
		ToolCallID:    toolCallID,
		ToolCallState: state,
		Blocks:        cloneContentBlocks(blocks),
	})
	return true
}

func (m *sessionViewModel) appendRuntimeNotice(blocks []model.ContentBlock) bool {
	if len(blocks) == 0 {
		return false
	}
	m.runtimeNoticeN++
	m.entries = append(m.entries, sessionViewEntry{
		ID:     fmt.Sprintf("runtime-%d", m.runtimeNoticeN),
		Kind:   transcriptEntryRuntimeNotice,
		Blocks: cloneContentBlocks(blocks),
	})
	return true
}

func (m *sessionViewModel) appendStatusNotice(text string) bool {
	block, err := model.NewContentBlock(model.TextBlock{Text: text})
	if err != nil {
		return false
	}
	if len(m.entries) > 0 {
		last := &m.entries[len(m.entries)-1]
		if last.Kind == transcriptEntryRuntimeNotice && len(last.Blocks) == 1 {
			if existing, blockErr := last.Blocks[0].AsText(); blockErr == nil && existing.Text == text {
				return false
			}
		}
	}
	m.runtimeNoticeN++
	m.entries = append(m.entries, sessionViewEntry{
		ID:     fmt.Sprintf("runtime-%d", m.runtimeNoticeN),
		Kind:   transcriptEntryRuntimeNotice,
		Blocks: []model.ContentBlock{block},
	})
	return true
}

func (m *sessionViewModel) findToolEntry(toolCallID string) int {
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

func (m *sessionViewModel) snapshot() SessionViewSnapshot {
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

func buildVisibleEntries(entries []sessionViewEntry) []TranscriptEntry {
	if len(entries) == 0 {
		return nil
	}

	visible := make([]TranscriptEntry, 0, len(entries))
	for _, entry := range entries {
		visible = append(visible, buildVisibleEntry(entry))
	}

	return visible
}

func buildVisibleEntry(entry sessionViewEntry) TranscriptEntry {
	result := TranscriptEntry{
		ID:            entry.ID,
		Kind:          entry.Kind,
		ToolCallID:    entry.ToolCallID,
		ToolCallState: entry.ToolCallState,
		Blocks:        cloneContentBlocks(entry.Blocks),
	}

	switch entry.Kind {
	case transcriptEntryAssistantMessage:
		result.Title = "Assistant"
	case transcriptEntryAssistantThinking:
		result.Title = "Thinking"
	case transcriptEntryToolCall:
		result.Title = extractToolTitle(entry.Blocks)
	case transcriptEntryStderrEvent:
		result.Title = "stderr"
	case transcriptEntryRuntimeNotice:
		result.Title = "Runtime"
	default:
		result.Title = "Entry"
	}
	result.Preview = buildBlocksPreview(entry.Blocks)
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
		if err == nil && toolUse.Name != "" {
			return toolUse.Name
		}
	}
	return "Tool Call"
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

func nextEntryID(kind transcriptEntryKind, index int) string {
	return fmt.Sprintf("%s-%d", kind, index+1)
}

func extractToolUseHeader(blocks []model.ContentBlock) []model.ContentBlock {
	if len(blocks) == 0 || blocks[0].Type != model.BlockToolUse {
		return nil
	}
	return cloneContentBlocks(blocks[:1])
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

	merged, err := model.NewContentBlock(model.TextBlock{Text: existingText.Text + incomingText.Text})
	if err != nil {
		return model.ContentBlock{}, false
	}
	return merged, true
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
