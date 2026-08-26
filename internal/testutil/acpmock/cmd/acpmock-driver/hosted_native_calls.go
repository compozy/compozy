package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	toolspkg "github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (a *mockAgent) executeHostedNativeCall(
	ctx context.Context,
	sessionID string,
	step acpmock.Step,
) (diagnostic acpmock.DiagnosticsStep, err error) {
	toolID, err := hostedNativeToolID(step.Kind)
	if err != nil {
		return diagnostic, err
	}
	server, err := a.hostedMCPServer(sessionID)
	if err != nil {
		return diagnostic, err
	}
	arguments := make(map[string]any)
	if err := json.Unmarshal(step.RawInput, &arguments); err != nil {
		return diagnostic, fmt.Errorf("acpmock-driver: decode %s input: %w", step.Kind, err)
	}
	command := exec.CommandContext(ctx, server.Command, server.Args...)
	command.Env = append(os.Environ(), mcpServerEnvironment(server)...)
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "acpmock-driver", Version: "1.0.0"},
		&sdkmcp.ClientOptions{Capabilities: &sdkmcp.ClientCapabilities{}},
	)
	clientSession, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: command}, nil)
	if err != nil {
		return diagnostic, fmt.Errorf("acpmock-driver: connect hosted MCP: %w", err)
	}
	defer func() {
		err = errors.Join(err, clientSession.Close())
	}()
	result, err := clientSession.CallTool(ctx, &sdkmcp.CallToolParams{Name: toolID, Arguments: arguments})
	if err != nil {
		return diagnostic, fmt.Errorf("acpmock-driver: call %s: %w", toolID, err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return diagnostic, fmt.Errorf("acpmock-driver: encode %s result: %w", toolID, err)
	}
	output := string(encoded)
	diagnostic = acpmock.DiagnosticsStep{Kind: step.Kind, Output: output}
	if expected := strings.TrimSpace(step.ExpectOutputContains); expected != "" && !strings.Contains(output, expected) {
		return diagnostic, fmt.Errorf("acpmock-driver: %s output does not contain %q", toolID, expected)
	}
	if result.IsError {
		expected := strings.TrimSpace(step.ExpectErrorContains)
		if expected != "" && strings.Contains(output, expected) {
			diagnostic.Error = expected
			return diagnostic, nil
		}
		return diagnostic, fmt.Errorf("acpmock-driver: %s returned an error: %s", toolID, output)
	}
	if expected := strings.TrimSpace(step.ExpectErrorContains); expected != "" {
		return diagnostic, fmt.Errorf("acpmock-driver: %s succeeded, want error containing %q", toolID, expected)
	}
	return diagnostic, nil
}

func hostedNativeToolID(kind acpmock.StepKind) (string, error) {
	switch kind {
	case acpmock.StepKindCallReturn:
		return toolspkg.ToolIDCallReturn.String(), nil
	case acpmock.StepKindAgentMessage:
		return toolspkg.ToolIDAgentMessage.String(), nil
	case acpmock.StepKindAgentCall:
		return toolspkg.ToolIDAgentCall.String(), nil
	default:
		return "", fmt.Errorf("acpmock-driver: hosted native call kind %q is unsupported", kind)
	}
}

func (a *mockAgent) hostedMCPServer(sessionID string) (acpsdk.McpServerStdio, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	state := a.sessions[strings.TrimSpace(sessionID)]
	if state == nil {
		return acpsdk.McpServerStdio{}, fmt.Errorf("acpmock-driver: session %q was not found", sessionID)
	}
	for _, server := range state.MCPServers {
		if server.Stdio != nil && server.Stdio.Name == mcppkg.HostedServerName {
			return *server.Stdio, nil
		}
	}
	return acpsdk.McpServerStdio{}, errors.New("acpmock-driver: hosted MCP server is unavailable")
}

func mcpServerEnvironment(server acpsdk.McpServerStdio) []string {
	environment := make([]string, 0, len(server.Env))
	for _, variable := range server.Env {
		name := strings.TrimSpace(variable.Name)
		if name != "" {
			environment = append(environment, name+"="+variable.Value)
		}
	}
	return environment
}
