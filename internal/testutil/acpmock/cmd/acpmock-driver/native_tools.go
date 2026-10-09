package main

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (a *mockAgent) usesNativeTools() bool {
	for _, turn := range a.agent.Turns {
		for _, step := range turn.Steps {
			if step.Kind == acpmock.StepKindNativeToolCall {
				return true
			}
		}
	}
	return false
}

func (a *mockAgent) prepareNativeTools(
	ctx context.Context,
	sessionID string,
	servers []acpsdk.McpServer,
	cwd string,
) error {
	if !a.usesNativeTools() {
		return nil
	}
	if err := a.closeSessionNativeTools(sessionID); err != nil {
		return err
	}
	for _, server := range servers {
		if server.Stdio == nil || server.Stdio.Name != mcppkg.HostedServerName {
			continue
		}
		stdio := server.Stdio
		//nolint:gosec // Execute the daemon-injected hosted MCP command, as an ACP provider does.
		command := exec.CommandContext(a.lifecycleCtx, stdio.Command, stdio.Args...)
		command.Dir = cwd
		command.Env = os.Environ()
		for _, env := range stdio.Env {
			command.Env = append(command.Env, env.Name+"="+env.Value)
		}
		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "acpmock", Version: "1"}, nil)
		connection, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: command}, nil)
		if err != nil {
			return fmt.Errorf("acpmock: connect hosted native tools: %w", err)
		}
		a.mu.Lock()
		a.sessions[sessionID].nativeTools = connection
		a.mu.Unlock()
		return nil
	}
	return errors.New("acpmock: native_tool_call requires the injected compozy-hosted-tools stdio server")
}

func (a *mockAgent) closeSessionNativeTools(sessionID string) error {
	a.mu.Lock()
	var client *sdkmcp.ClientSession
	if session := a.sessions[sessionID]; session != nil {
		client = session.nativeTools
		session.nativeTools = nil
	}
	a.mu.Unlock()
	if client != nil {
		return client.Close()
	}
	return nil
}

func (a *mockAgent) closeNativeTools() error {
	a.mu.Lock()
	ids := make([]string, 0, len(a.sessions))
	for id := range a.sessions {
		ids = append(ids, id)
	}
	a.mu.Unlock()
	var err error
	for _, id := range ids {
		err = errors.Join(err, a.closeSessionNativeTools(id))
	}
	return err
}

func (a *mockAgent) callNativeTool(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	step acpmock.Step,
) (acpmock.DiagnosticsStep, error) {
	diagnostics := acpmock.DiagnosticsStep{Kind: step.Kind, ToolCallID: step.ToolCallID}
	a.mu.Lock()
	var client *sdkmcp.ClientSession
	if session := a.sessions[string(sessionID)]; session != nil {
		client = session.nativeTools
	}
	a.mu.Unlock()
	if client == nil {
		return diagnostics, errors.New("acpmock: native tool connection is unavailable")
	}
	var arguments map[string]any
	if err := json.Unmarshal(step.RawInput, &arguments); err != nil {
		return diagnostics, err
	}
	title := cmp.Or(step.Title, step.ToolID)
	id := acpsdk.ToolCallId(step.ToolCallID)
	if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
		SessionId: sessionID,
		Update: acpsdk.StartToolCall(id, title,
			acpsdk.WithStartKind(acpsdk.ToolKindOther),
			acpsdk.WithStartStatus(acpsdk.ToolCallStatusInProgress),
			acpsdk.WithStartRawInput(arguments)),
	}); err != nil {
		return diagnostics, err
	}
	params := &sdkmcp.CallToolParams{Name: step.ToolID, Arguments: arguments}
	params.Meta = sdkmcp.Meta{"toolCallId": step.ToolCallID}
	result, callErr := client.CallTool(ctx, params)
	status := acpsdk.ToolCallStatusCompleted
	var output any
	var text []string
	if callErr != nil {
		status = acpsdk.ToolCallStatusFailed
		output = map[string]string{"error": callErr.Error()}
		text = append(text, callErr.Error())
	} else {
		if result.IsError {
			status = acpsdk.ToolCallStatusFailed
		}
		output = result.StructuredContent
		if output == nil {
			output = result.Content
		}
		for _, block := range result.Content {
			if content, ok := block.(*sdkmcp.TextContent); ok {
				text = append(text, content.Text)
			}
		}
	}
	updateErr := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
		SessionId: sessionID,
		Update: acpsdk.UpdateToolCall(id,
			acpsdk.WithUpdateTitle(title),
			acpsdk.WithUpdateStatus(status),
			acpsdk.WithUpdateRawOutput(output),
			acpsdk.WithUpdateContent(textToolContent(strings.Join(text, "\n")))),
	})
	return diagnostics, errors.Join(callErr, updateErr)
}
