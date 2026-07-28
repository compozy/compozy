package acp

import (
	"context"
	"encoding/json"

	"fmt"

	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

func (p *AgentProcess) interceptReadTextFileRequest(
	ctx context.Context,
	request acpsdk.ReadTextFileRequest,
) (acpsdk.ReadTextFileRequest, error) {
	if p == nil || p.toolGateway == nil {
		request.Path = strings.TrimSpace(request.Path)
		return request, nil
	}

	input, err := json.Marshal(readTextFileToolInput{
		Path:  request.Path,
		Line:  request.Line,
		Limit: request.Limit,
	})
	if err != nil {
		return acpsdk.ReadTextFileRequest{}, fmt.Errorf("acp: marshal read_text_file input: %w", err)
	}

	patched, err := p.interceptProviderNativeTool(ctx, ToolExecutionRequest{
		ToolID:   providerNativeToolIDRead,
		ReadOnly: true,
		Input:    input,
	})
	if err != nil {
		return acpsdk.ReadTextFileRequest{}, err
	}

	var next readTextFileToolInput
	if err := json.Unmarshal(patched.Input, &next); err != nil {
		return acpsdk.ReadTextFileRequest{}, fmt.Errorf(
			"%w: invalid %s tool patch: %w",
			ErrPermissionDenied,
			providerNativeToolIDRead,
			err,
		)
	}

	request.Path = strings.TrimSpace(next.Path)
	request.Line = next.Line
	request.Limit = next.Limit
	return request, nil
}

func (p *AgentProcess) interceptWriteTextFileRequest(
	ctx context.Context,
	request acpsdk.WriteTextFileRequest,
) (acpsdk.WriteTextFileRequest, error) {
	if p == nil || p.toolGateway == nil {
		request.Path = strings.TrimSpace(request.Path)
		return request, nil
	}

	input, err := json.Marshal(writeTextFileToolInput{
		Path:    request.Path,
		Content: request.Content,
	})
	if err != nil {
		return acpsdk.WriteTextFileRequest{}, fmt.Errorf("acp: marshal write_text_file input: %w", err)
	}

	patched, err := p.interceptProviderNativeTool(ctx, ToolExecutionRequest{
		ToolID: providerNativeToolIDWrite,
		Input:  input,
	})
	if err != nil {
		return acpsdk.WriteTextFileRequest{}, err
	}

	var next writeTextFileToolInput
	if err := json.Unmarshal(patched.Input, &next); err != nil {
		return acpsdk.WriteTextFileRequest{}, fmt.Errorf(
			"%w: invalid %s tool patch: %w",
			ErrPermissionDenied,
			providerNativeToolIDWrite,
			err,
		)
	}

	request.Path = strings.TrimSpace(next.Path)
	request.Content = next.Content
	return request, nil
}

func (p *AgentProcess) interceptCreateTerminalRequest(
	ctx context.Context,
	request acpsdk.CreateTerminalRequest,
) (acpsdk.CreateTerminalRequest, error) {
	if p == nil || p.toolGateway == nil {
		request.Command = strings.TrimSpace(request.Command)
		request.Args = cloneNonEmptyStringSlice(request.Args)
		request.Cwd = cloneStringPtr(request.Cwd)
		request.Env = cloneNonEmptyEnvSlice(request.Env)
		request.OutputByteLimit = cloneIntPtr(request.OutputByteLimit)
		return request, nil
	}

	input, err := json.Marshal(createTerminalToolInput{
		Command:         request.Command,
		Args:            request.Args,
		Cwd:             request.Cwd,
		Env:             request.Env,
		OutputByteLimit: request.OutputByteLimit,
	})
	if err != nil {
		return acpsdk.CreateTerminalRequest{}, fmt.Errorf("acp: marshal terminal/create input: %w", err)
	}

	patched, err := p.interceptProviderNativeTool(ctx, ToolExecutionRequest{
		ToolID: providerNativeToolIDBash,
		Input:  input,
	})
	if err != nil {
		return acpsdk.CreateTerminalRequest{}, err
	}

	var next createTerminalToolInput
	if err := json.Unmarshal(patched.Input, &next); err != nil {
		return acpsdk.CreateTerminalRequest{}, fmt.Errorf(
			"%w: invalid %s tool patch: %w",
			ErrPermissionDenied,
			providerNativeToolIDBash,
			err,
		)
	}

	request.Command = strings.TrimSpace(next.Command)
	request.Args = append([]string(nil), next.Args...)
	request.Cwd = next.Cwd
	request.Env = append([]acpsdk.EnvVariable(nil), next.Env...)
	request.OutputByteLimit = next.OutputByteLimit
	return request, nil
}

func (p *AgentProcess) emitPermissionEvent(
	sessionID string,
	turnID string,
	requestID string,
	title string,
	toolCallID string,
	resource string,
	decision permissionDecision,
	raw json.RawMessage,
) {
	p.emitPromptEvent(AgentEvent{
		Type:       EventTypePermission,
		SessionID:  sessionID,
		TurnID:     turnID,
		RequestID:  requestID,
		Timestamp:  timeNowUTC(),
		Title:      title,
		ToolCallID: toolCallID,
		Action:     string(permissionRequestToolGrant),
		Resource:   resource,
		Decision:   string(decision),
		Raw:        CloneRawMessage(raw),
	})
}
