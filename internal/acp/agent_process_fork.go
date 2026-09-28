package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

// ErrForkSessionUnbound reports a session/fork attempt on a process without a
// bound ACP session.
var ErrForkSessionUnbound = errors.New("acp: session/fork requires a bound ACP session")

// ForkSessionResult is the outcome of one unstable session/fork call.
type ForkSessionResult struct {
	// SessionID is the clone's ACP session id returned by the agent.
	SessionID acpsdk.SessionId
	// Updates are the notifications the agent sent for the clone id before the
	// response returned (command lists, history replay). Diagnostics only.
	Updates []acpsdk.SessionNotification
}

// wireForkSessionRequest mirrors acpsdk.UnstableForkSessionRequest with the
// stable McpServer shape the rest of the driver already builds; both encode to
// the same JSON. mcpServers is always sent, matching session/new and load.
type wireForkSessionRequest struct {
	Cwd        string             `json:"cwd"`
	McpServers []acpsdk.McpServer `json:"mcpServers"`
	SessionID  acpsdk.SessionId   `json:"sessionId"`
}

// ForkSession calls unstable session/fork on this (bound) process for the caller's cwd.
// While the call is open, and afterwards, notifications and callbacks whose sessionId is the
// returned clone id are delivered to the fork sink (captured in Updates) or quarantined; they
// are never emitted on the bound session's prompt stream and p.SessionID never changes.
func (p *AgentProcess) ForkSession(
	ctx context.Context,
	cwd string,
	mcpServers []acpsdk.McpServer,
) (ForkSessionResult, error) {
	if p == nil || p.conn == nil {
		return ForkSessionResult{}, errors.New("acp: session/fork requires a running agent process")
	}
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ForkSessionResult{}, errors.New("acp: session/fork requires a cwd")
	}
	source := p.boundSessionRoute()
	if source == "" {
		return ForkSessionResult{}, ErrForkSessionUnbound
	}
	if !p.CapsSnapshot().SupportsForkSession {
		return ForkSessionResult{}, fmt.Errorf(
			"%w: agent %q does not advertise session/fork",
			ErrAgentDoesNotSupportSession,
			p.AgentName,
		)
	}
	if mcpServers == nil {
		mcpServers = []acpsdk.McpServer{}
	}

	p.forkMu.Lock()
	defer p.forkMu.Unlock()
	capture := p.openForkCapture()
	// SendRequest drains every notification enqueued before the response, so the
	// capture holds all pre-response traffic when it closes.
	response, err := acpsdk.SendRequest[acpsdk.UnstableForkSessionResponse](
		p.conn,
		ctx,
		acpsdk.AgentMethodSessionFork,
		wireForkSessionRequest{Cwd: cwd, McpServers: mcpServers, SessionID: source},
	)
	captured := p.closeForkCapture(capture)
	clone := response.SessionId
	if err == nil && (clone == "" || clone == source) {
		err = fmt.Errorf("acp: session/fork returned invalid clone id %q", string(clone))
		clone = ""
	}
	for id, updates := range captured {
		if err == nil && id == clone {
			continue
		}
		for range updates {
			p.dropForeignSessionTraffic(id, acpsdk.ClientMethodSessionUpdate)
		}
	}
	if err != nil {
		return ForkSessionResult{}, fmt.Errorf("acp: session/fork %q: %w", string(source), err)
	}
	return ForkSessionResult{SessionID: clone, Updates: captured[clone]}, nil
}
