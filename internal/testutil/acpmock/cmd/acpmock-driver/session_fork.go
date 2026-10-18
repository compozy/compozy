package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/testutil/acpmock"
)

func (a *mockAgent) sessionCapabilities() acpsdk.SessionCapabilities {
	var caps acpsdk.SessionCapabilities
	if a.agent.ForkSession {
		caps.Fork = &acpsdk.SessionForkCapabilities{}
	}
	return caps
}

// UnstableForkSession clones a session the way OpenCode's adapter does: the clone's
// command list and history replay are sent on this connection before the response.
func (a *mockAgent) UnstableForkSession(
	ctx context.Context,
	params acpsdk.UnstableForkSessionRequest,
) (acpsdk.UnstableForkSessionResponse, error) {
	if !a.agent.ForkSession {
		return acpsdk.UnstableForkSessionResponse{}, acpsdk.NewMethodNotFound(acpsdk.AgentMethodSessionFork)
	}
	if message := strings.TrimSpace(a.agent.ForkError); message != "" {
		return acpsdk.UnstableForkSessionResponse{}, errors.New(message)
	}
	source := strings.TrimSpace(string(params.SessionId))
	if source == "" {
		return acpsdk.UnstableForkSessionResponse{}, errors.New("acpmock-driver: fork source session id is required")
	}
	a.mu.Lock()
	a.nextSession++
	cloneID := fmt.Sprintf("%s-fork-%d", a.agent.Name, a.nextSession)
	a.sessions[cloneID] = &sessionState{ConfigOptions: cloneSessionConfigOptions(a.configTemplate)}
	a.mu.Unlock()
	clone := acpsdk.SessionId(cloneID)
	for _, update := range []acpsdk.SessionUpdate{
		{AvailableCommandsUpdate: &acpsdk.SessionAvailableCommandsUpdate{
			SessionUpdate:     "available_commands_update",
			AvailableCommands: []acpsdk.AvailableCommand{{Name: "review", Description: "Review"}},
		}},
		acpsdk.UpdateAgentMessageText("replayed history for " + cloneID),
	} {
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{SessionId: clone, Update: update}); err != nil {
			return acpsdk.UnstableForkSessionResponse{}, err
		}
	}
	if err := a.writeDiagnostics(acpmock.DiagnosticsRecord{
		AgentName: a.agent.Name, SessionID: cloneID, LifecycleEvent: "session_fork",
	}); err != nil {
		return acpsdk.UnstableForkSessionResponse{}, err
	}
	return acpsdk.UnstableForkSessionResponse{SessionId: clone}, nil
}
