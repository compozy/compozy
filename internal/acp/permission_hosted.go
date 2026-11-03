package acp

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/toolmeta"
)

// claudeDynamicMCPSource is the Claude Code config scope of servers the host
// passes at launch (--mcp-config), which is how the ACP adapter forwards the
// session/new MCP servers Compozy sends. Configured servers never read it.
const claudeDynamicMCPSource = "dynamic"

const (
	// hostedApprovalTTL bounds how long an operator approval of a hosted call at
	// the provider gate waits for the same call to reach the daemon gate.
	hostedApprovalTTL = 2 * time.Minute
	// maxHostedApprovals caps remembered provider-gate approvals and announced
	// hosted calls per process.
	maxHostedApprovals = 64
)

// hostedToolCall is a provider permission request for one call to a tool
// served by Compozy's hosted MCP server.
type hostedToolCall struct {
	toolID    string
	arguments []byte
}

// hostedToolCall identifies a provider permission request for Compozy's hosted
// MCP server from the provider's structured identity, never from the title: the
// Claude adapter reports the serving MCP server and its config source, and
// Codex reports the server and tool it dispatches to, in the request or in the
// tool call it announced under the same id. The process must also have sent
// that server in session/new or session/load.
func (p *AgentProcess) hostedToolCall(request acpsdk.RequestPermissionRequest) (hostedToolCall, bool) {
	if p == nil || !p.sentHostedMCPServer() {
		return hostedToolCall{}, false
	}
	if call, ok := claudeHostedToolCall(request); ok {
		return call, true
	}
	if call, ok := codexHostedToolCall(request.ToolCall.RawInput); ok {
		return call, true
	}
	if request.ToolCall.RawInput != nil {
		return hostedToolCall{}, false
	}
	return p.hostedAnnounced.lookup(string(request.ToolCall.ToolCallId))
}

// observeHostedToolCall remembers a hosted call the provider announced on the
// bound session. Codex asks permission for an MCP call with only the tool call
// id; the announcement carries the server, tool, and arguments.
func (p *AgentProcess) observeHostedToolCall(update acpsdk.SessionUpdate) {
	var (
		id       acpsdk.ToolCallId
		rawInput any
	)
	switch {
	case update.ToolCall != nil:
		id, rawInput = update.ToolCall.ToolCallId, update.ToolCall.RawInput
	case update.ToolCallUpdate != nil:
		id, rawInput = update.ToolCallUpdate.ToolCallId, update.ToolCallUpdate.RawInput
	default:
		return
	}
	if call, ok := codexHostedToolCall(rawInput); ok && strings.TrimSpace(string(id)) != "" {
		p.hostedAnnounced.record(string(id), call)
	}
}

func (p *AgentProcess) sentHostedMCPServer() bool {
	for _, server := range p.sessionMCPServers() {
		if mcpServerName(server) == toolmeta.HostedServerName {
			return true
		}
	}
	return false
}

func mcpServerName(server acpsdk.McpServer) string {
	switch {
	case server.Stdio != nil:
		return server.Stdio.Name
	case server.Http != nil:
		return server.Http.Name
	case server.Sse != nil:
		return server.Sse.Name
	default:
		return ""
	}
}

func claudeHostedToolCall(request acpsdk.RequestPermissionRequest) (hostedToolCall, bool) {
	claude, ok := permissionRequestMetaMap(request.ToolCall.Meta)["claudeCode"].(map[string]any)
	if !ok {
		return hostedToolCall{}, false
	}
	server, ok := claude["mcpServer"].(map[string]any)
	if !ok || stringMapValue(server, "name") != toolmeta.HostedServerName ||
		stringMapValue(server, "source") != claudeDynamicMCPSource {
		return hostedToolCall{}, false
	}
	toolID, ok := strings.CutPrefix(stringMapValue(claude, "toolName"), "mcp__"+toolmeta.HostedServerName+"__")
	if !ok || strings.TrimSpace(toolID) == "" {
		return hostedToolCall{}, false
	}
	return newHostedToolCall(toolID, request.ToolCall.RawInput)
}

func codexHostedToolCall(rawInput any) (hostedToolCall, bool) {
	input, ok := rawInput.(map[string]any)
	if !ok || stringMapValue(input, "server") != toolmeta.HostedServerName {
		return hostedToolCall{}, false
	}
	toolID := stringMapValue(input, "tool")
	if toolID == "" {
		return hostedToolCall{}, false
	}
	return newHostedToolCall(toolID, input["arguments"])
}

func newHostedToolCall(toolID string, arguments any) (hostedToolCall, bool) {
	canonical, ok := canonicalHostedArguments(arguments)
	if !ok {
		return hostedToolCall{}, false
	}
	return hostedToolCall{toolID: strings.TrimSpace(toolID), arguments: canonical}, true
}

// canonicalHostedArguments encodes decoded JSON arguments with sorted keys, so
// the provider gate and the daemon gate compare the same call byte for byte.
func canonicalHostedArguments(arguments any) ([]byte, bool) {
	if arguments == nil {
		arguments = map[string]any{}
	}
	if raw, ok := arguments.(json.RawMessage); ok {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, false
		}
		arguments = decoded
	}
	encoded, err := json.Marshal(arguments)
	if err != nil {
		return nil, false
	}
	return encoded, true
}

// answeredByHostedToolGate reports whether a level-table prompt for a hosted
// tool call is already answered by the other gate: a provider request the
// native tool policy will decide, or a daemon request repeating a call the
// operator approved at the provider gate. Permission rules still apply first.
func (p *AgentProcess) answeredByHostedToolGate(
	request acpsdk.RequestPermissionRequest,
	actor string,
	resolvedBy string,
) bool {
	if resolvedBy != permissionSourcePolicy {
		return false
	}
	if actor == permissionActorSystem {
		return p.consumeHostedApproval(request)
	}
	return p.defersHostedTool(request)
}

// defersHostedTool reports whether the provider gate hands a hosted tool call to
// Compozy's native tool policy instead of asking. At approve-edits and auto the
// native policy, which knows each tool's declared risk, is the only gate.
func (p *AgentProcess) defersHostedTool(request acpsdk.RequestPermissionRequest) bool {
	switch p.permissions.mode {
	case compozyconfig.PermissionModeApproveEdits, compozyconfig.PermissionModeAuto:
	default:
		return false
	}
	_, ok := p.hostedToolCall(request)
	return ok
}

// hostedAnnouncements maps recent provider tool call ids to the hosted call
// they announced, oldest evicted first.
type hostedAnnouncements struct {
	mu    sync.Mutex
	order []string
	calls map[string]hostedToolCall
}

func (a *hostedAnnouncements) record(id string, call hostedToolCall) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.calls == nil {
		a.calls = make(map[string]hostedToolCall)
	}
	if _, ok := a.calls[id]; !ok {
		if len(a.order) >= maxHostedApprovals {
			delete(a.calls, a.order[0])
			a.order = a.order[1:]
		}
		a.order = append(a.order, id)
	}
	a.calls[id] = call
}

func (a *hostedAnnouncements) lookup(id string) (hostedToolCall, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	call, ok := a.calls[strings.TrimSpace(id)]
	return call, ok
}

// hostedApprovals remembers hosted calls the operator approved at the provider
// gate so the daemon gate does not ask again for the same call.
type hostedApprovals struct {
	mu      sync.Mutex
	entries []hostedApproval
}

type hostedApproval struct {
	call      hostedToolCall
	expiresAt time.Time
}

func (a *hostedApprovals) record(call hostedToolCall, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pruneLocked(now)
	if len(a.entries) >= maxHostedApprovals {
		a.entries = a.entries[1:]
	}
	a.entries = append(a.entries, hostedApproval{call: call, expiresAt: now.Add(hostedApprovalTTL)})
}

// consume removes and reports one unexpired approval of the same tool and
// arguments, so each operator approval answers exactly one daemon request.
func (a *hostedApprovals) consume(call hostedToolCall, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pruneLocked(now)
	for index, entry := range a.entries {
		if entry.call.toolID == call.toolID && bytes.Equal(entry.call.arguments, call.arguments) {
			a.entries = append(a.entries[:index], a.entries[index+1:]...)
			return true
		}
	}
	return false
}

func (a *hostedApprovals) pruneLocked(now time.Time) {
	kept := a.entries[:0]
	for _, entry := range a.entries {
		if now.Before(entry.expiresAt) {
			kept = append(kept, entry)
		}
	}
	a.entries = kept
}

// rememberHostedApproval records an operator approval of a hosted call made at
// the provider gate.
func (p *AgentProcess) rememberHostedApproval(request acpsdk.RequestPermissionRequest, decision permissionDecision) {
	if decision != decisionAllowOnce && decision != decisionAllowAlways {
		return
	}
	if call, ok := p.hostedToolCall(request); ok {
		p.hostedApprovals.record(call, timeNowUTC())
	}
}

// consumeHostedApproval reports whether a daemon-originated Compozy tool
// approval repeats a hosted call the operator already approved at the provider
// gate. The daemon request carries the canonical tool ID and the call input.
func (p *AgentProcess) consumeHostedApproval(request acpsdk.RequestPermissionRequest) bool {
	toolID, ok := request.Meta[PermissionToolIDMetaKey].(string)
	if !ok {
		return false
	}
	call, ok := newHostedToolCall(toolID, request.ToolCall.RawInput)
	if !ok || call.toolID == "" {
		return false
	}
	return p.hostedApprovals.consume(call, timeNowUTC())
}
