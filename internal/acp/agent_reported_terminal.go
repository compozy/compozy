package acp

import (
	"fmt"
	"strings"
	"unicode/utf8"

	acpsdk "github.com/coder/acp-go-sdk"
)

const maxAgentReportedTerminalBytes = 64 * 1024

// AgentReportedTerminal identifies observational terminal output supplied by an agent.
// Its ID belongs to the agent's report and is never a supervised terminal registry ID.
type AgentReportedTerminal struct {
	ID         string `json:"id"`
	Cwd        string `json:"cwd,omitempty"`
	TotalBytes int64  `json:"total_bytes"`
	Truncated  bool   `json:"truncated,omitempty"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	Signal     string `json:"signal,omitempty"`
}

// CloneAgentReportedTerminal returns an isolated copy of reported terminal metadata.
func CloneAgentReportedTerminal(terminal *AgentReportedTerminal) *AgentReportedTerminal {
	if terminal == nil {
		return nil
	}
	cloned := *terminal
	if terminal.ExitCode != nil {
		exitCode := *terminal.ExitCode
		cloned.ExitCode = &exitCode
	}
	return &cloned
}

type agentReportedTerminalState struct {
	id         string
	title      string
	cwd        string
	tail       []byte
	totalBytes int64
	exitCode   *int
	signal     string
}

type agentReportedTerminalMeta struct {
	terminalID string
	cwd        string
	data       string
	exitCode   *int
	signal     string
	hasInfo    bool
	hasOutput  bool
	hasExit    bool
}

func (p *AgentProcess) projectAgentReportedTerminal(
	notification acpsdk.SessionNotification,
) (*AgentEvent, bool) {
	meta, title, isToolCallUpdate := reportedTerminalMeta(notification.Update)
	if !meta.hasInfo && !meta.hasOutput && !meta.hasExit {
		return nil, false
	}

	p.reportedTerminalMu.Lock()
	defer p.reportedTerminalMu.Unlock()
	if p.reportedTerminals == nil {
		p.reportedTerminals = make(map[string]*agentReportedTerminalState)
	}
	state := p.reportedTerminals[meta.terminalID]
	if state == nil {
		state = &agentReportedTerminalState{id: meta.terminalID}
		p.reportedTerminals[meta.terminalID] = state
	}
	if title = strings.TrimSpace(title); title != "" {
		state.title = title
	}
	if cwd := strings.TrimSpace(meta.cwd); cwd != "" {
		state.cwd = cwd
	}
	if meta.hasOutput && meta.data != "" {
		state.append(meta.data)
	}
	if meta.hasExit {
		state.exitCode = cloneIntPtr(meta.exitCode)
		state.signal = strings.TrimSpace(meta.signal)
	}

	suppressStandard := meta.hasOutput && !meta.hasExit && isToolCallUpdate &&
		!toolCallUpdateFinished(notification.Update.ToolCallUpdate)
	if state.totalBytes == 0 {
		if meta.hasExit {
			delete(p.reportedTerminals, meta.terminalID)
		}
		return nil, suppressStandard
	}

	text, truncated := state.output()
	event := &AgentEvent{
		Type:      EventTypeAgentReportedTerminal,
		Origin:    AgentEventOriginAgentReported,
		SessionID: string(notification.SessionId),
		TurnID:    p.activeTurnID(),
		Timestamp: timeNowUTC(),
		Text:      text,
		Title:     state.title,
		ReportedTerminal: &AgentReportedTerminal{
			ID:         state.id,
			Cwd:        state.cwd,
			TotalBytes: state.totalBytes,
			Truncated:  truncated,
			ExitCode:   cloneIntPtr(state.exitCode),
			Signal:     state.signal,
		},
	}
	if meta.hasExit {
		delete(p.reportedTerminals, meta.terminalID)
	}
	return event, suppressStandard
}

func toolCallUpdateFinished(update *acpsdk.SessionToolCallUpdate) bool {
	if update == nil || update.Status == nil {
		return false
	}
	return *update.Status == acpsdk.ToolCallStatusCompleted || *update.Status == acpsdk.ToolCallStatusFailed
}

func (s *agentReportedTerminalState) append(chunk string) {
	bytes := []byte(chunk)
	s.totalBytes += int64(len(bytes))
	s.tail = append(s.tail, bytes...)
	if len(s.tail) <= maxAgentReportedTerminalBytes {
		return
	}
	s.tail = validUTF8Tail(s.tail[len(s.tail)-maxAgentReportedTerminalBytes:])
}

func (s *agentReportedTerminalState) output() (string, bool) {
	if s.totalBytes <= maxAgentReportedTerminalBytes {
		return string(s.tail), false
	}
	omitted := s.totalBytes - int64(len(s.tail))
	marker := fmt.Sprintf("⟨%d bytes omitted⟩\n", omitted)
	available := maxAgentReportedTerminalBytes - len(marker)
	if available <= 0 {
		return marker[:maxAgentReportedTerminalBytes], true
	}
	tail := s.tail
	if len(tail) > available {
		omitted += int64(len(tail) - available)
		marker = fmt.Sprintf("⟨%d bytes omitted⟩\n", omitted)
		available = maxAgentReportedTerminalBytes - len(marker)
		tail = validUTF8Tail(tail[len(tail)-available:])
	}
	return marker + string(tail), true
}

func validUTF8Tail(value []byte) []byte {
	for len(value) > 0 && !utf8.RuneStart(value[0]) {
		value = value[1:]
	}
	return value
}

func reportedTerminalMeta(update acpsdk.SessionUpdate) (agentReportedTerminalMeta, string, bool) {
	if update.ToolCall != nil {
		return decodeReportedTerminalMeta(update.ToolCall.Meta), update.ToolCall.Title, false
	}
	if update.ToolCallUpdate != nil {
		title := ""
		if update.ToolCallUpdate.Title != nil {
			title = *update.ToolCallUpdate.Title
		}
		return decodeReportedTerminalMeta(update.ToolCallUpdate.Meta), title, true
	}
	return agentReportedTerminalMeta{}, "", false
}

func decodeReportedTerminalMeta(meta map[string]any) agentReportedTerminalMeta {
	var result agentReportedTerminalMeta
	if info, ok := metadataObject(meta["terminal_info"]); ok {
		result.terminalID = metadataString(info, "terminal_id")
		result.cwd = metadataString(info, "cwd")
		result.hasInfo = result.terminalID != ""
	}
	if output, ok := metadataObject(meta["terminal_output"]); ok {
		result.terminalID = firstReportedValue(metadataString(output, "terminal_id"), result.terminalID)
		result.data = metadataString(output, "data")
		result.hasOutput = result.terminalID != ""
	}
	if exit, ok := metadataObject(meta["terminal_exit"]); ok {
		result.terminalID = firstReportedValue(metadataString(exit, "terminal_id"), result.terminalID)
		result.exitCode = metadataInt(exit, "exit_code")
		result.signal = metadataString(exit, "signal")
		result.hasExit = result.terminalID != ""
	}
	return result
}

func metadataObject(value any) (map[string]any, bool) {
	object, ok := value.(map[string]any)
	return object, ok
}

func metadataString(object map[string]any, key string) string {
	value, ok := object[key].(string)
	if !ok {
		return ""
	}
	return value
}

func metadataInt(object map[string]any, key string) *int {
	switch value := object[key].(type) {
	case float64:
		converted := int(value)
		return &converted
	case int:
		converted := value
		return &converted
	default:
		return nil
	}
}

func firstReportedValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
