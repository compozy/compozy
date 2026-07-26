package acpmock

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/acp"
)

// DiagnosticsRecord captures one protocol, lifecycle, or prompt event emitted by the ACP mock driver.
type DiagnosticsRecord struct {
	AGHSessionID      string             `json:"agh_session_id,omitempty"`
	AgentName         string             `json:"agent_name"`
	SessionID         string             `json:"session_id"`
	ProtocolMethod    string             `json:"protocol_method,omitempty"`
	ConfigOptionID    string             `json:"config_option_id,omitempty"`
	ConfigOptionValue string             `json:"config_option_value,omitempty"`
	LifecycleEvent    string             `json:"lifecycle_event,omitempty"`
	MCPServers        []acpsdk.McpServer `json:"mcp_servers,omitempty"`
	NetworkEnv        []string           `json:"network_env,omitempty"`
	PromptIndex       int                `json:"prompt_index"`
	Prompt            string             `json:"prompt"`
	PromptMeta        acp.PromptMeta     `json:"prompt_meta"`
	TurnName          string             `json:"turn_name,omitempty"`
	Match             TurnMatch          `json:"match"`
	Steps             []DiagnosticsStep  `json:"steps"`
}

// DiagnosticsForAGHSession returns records owned by one daemon session in append order.
func DiagnosticsForAGHSession(records []DiagnosticsRecord, aghSessionID string) []DiagnosticsRecord {
	owner := strings.TrimSpace(aghSessionID)
	filtered := make([]DiagnosticsRecord, 0, len(records))
	if owner == "" {
		return filtered
	}
	for _, record := range records {
		if strings.TrimSpace(record.AGHSessionID) != owner {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}

// ProtocolDiagnostics returns diagnostics emitted when the driver receives ACP protocol methods.
func ProtocolDiagnostics(records []DiagnosticsRecord) []DiagnosticsRecord {
	filtered := make([]DiagnosticsRecord, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.ProtocolMethod) == "" {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}

// DiagnosticsStep captures one executed fixture step with any observed runtime outputs.
type DiagnosticsStep struct {
	Kind         StepKind            `json:"kind"`
	Text         string              `json:"text,omitempty"`
	ToolCallID   string              `json:"tool_call_id,omitempty"`
	Decision     string              `json:"decision,omitempty"`
	Command      string              `json:"command,omitempty"`
	Args         []string            `json:"args,omitempty"`
	ExitCode     *int                `json:"exit_code,omitempty"`
	Output       string              `json:"output,omitempty"`
	Error        string              `json:"error,omitempty"`
	DriverAction DriverControlAction `json:"driver_action,omitempty"`
}

// ReadDiagnostics decodes newline-delimited diagnostics written by the mock driver.
func ReadDiagnostics(path string) (records []DiagnosticsRecord, err error) {
	file, err := os.Open(strings.TrimSpace(path))
	if err != nil {
		return nil, fmt.Errorf("acpmock: open diagnostics %q: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("acpmock: close diagnostics %q: %w", path, closeErr))
		}
	}()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	records = make([]DiagnosticsRecord, 0, 4)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record DiagnosticsRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("acpmock: decode diagnostics line %d: %w", lineNo, err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("acpmock: scan diagnostics %q: %w", path, err)
	}
	return records, nil
}

// PromptDiagnostics returns diagnostics emitted for prompt execution turns only.
func PromptDiagnostics(records []DiagnosticsRecord) []DiagnosticsRecord {
	filtered := make([]DiagnosticsRecord, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.LifecycleEvent) != "" {
			continue
		}
		if record.PromptIndex <= 0 {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}
