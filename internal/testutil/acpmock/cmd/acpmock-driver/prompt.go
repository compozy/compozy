package main

import (
	"context"

	"errors"

	"fmt"

	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/testutil/acpmock"
)

func (a *mockAgent) Prompt(ctx context.Context, params acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	sessionID := strings.TrimSpace(string(params.SessionId))
	if sessionID == "" {
		return acpsdk.PromptResponse{}, errors.New("sessionId is required")
	}
	a.ensurePromptSession(sessionID)
	if err := a.writeProtocolDiagnostics(acpsdk.AgentMethodSessionPrompt, sessionID, "", ""); err != nil {
		return acpsdk.PromptResponse{}, err
	}

	promptMeta, err := decodePromptMeta(params.Meta)
	if err != nil {
		return acpsdk.PromptResponse{}, err
	}

	prompt := extractPromptText(params.Prompt)
	turn, promptIndex, err := a.selectTurn(sessionID, prompt, promptMeta)
	if err != nil {
		return acpsdk.PromptResponse{}, err
	}

	record := acpmock.DiagnosticsRecord{
		AgentName:   a.agent.Name,
		SessionID:   sessionID,
		PromptIndex: promptIndex,
		Prompt:      prompt,
		PromptMeta:  promptMeta,
		TurnName:    strings.TrimSpace(turn.Name),
		Match:       turn.Match.Normalize(),
		Steps:       make([]acpmock.DiagnosticsStep, 0, len(turn.Steps)),
	}

	promptCtx, cancelPrompt := context.WithCancel(ctx)
	defer cancelPrompt()

	for _, step := range turn.Steps {
		stepCtx, cancelStep, cancelID := a.stepContext(promptCtx, sessionID, step)
		entry, execErr := a.executeStep(stepCtx, acpsdk.SessionId(sessionID), step)
		if cancelStep != nil {
			a.clearPromptCancel(sessionID, cancelID)
			cancelStep()
		}
		if execErr != nil {
			record.Steps = append(record.Steps, acpmock.DiagnosticsStep{
				Kind:  acpmock.StepKind("error"),
				Error: execErr.Error(),
			})
			if diagErr := a.writeDiagnostics(record); diagErr != nil {
				return acpsdk.PromptResponse{}, diagErr
			}
			if errors.Is(execErr, context.Canceled) {
				return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonCancelled}, nil
			}
			return acpsdk.PromptResponse{}, execErr
		}
		record.Steps = append(record.Steps, entry)
	}

	if err := a.writeDiagnostics(record); err != nil {
		return acpsdk.PromptResponse{}, err
	}
	return acpsdk.PromptResponse{StopReason: stopReason(turn.StopReason), Usage: promptResponseUsage(turn)}, nil
}

func (a *mockAgent) ensurePromptSession(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.sessions[sessionID]
	if session == nil {
		session = &sessionState{
			ConfigOptions: cloneSessionConfigOptions(a.configTemplate),
		}
		a.sessions[sessionID] = session
	}
}

func (a *mockAgent) stepContext(
	promptCtx context.Context,
	sessionID string,
	step acpmock.Step,
) (context.Context, context.CancelFunc, uint64) {
	if !stepAcceptsExternalCancel(step) {
		return promptCtx, nil, 0
	}
	stepCtx, cancelStep := context.WithCancel(promptCtx)
	cancelID := a.registerPromptCancel(sessionID, cancelStep)
	return stepCtx, cancelStep, cancelID
}

func stepAcceptsExternalCancel(step acpmock.Step) bool {
	switch step.Kind {
	case acpmock.StepKindSandbox:
		return true
	case acpmock.StepKindDriverControl:
		return step.DriverControl.Action == acpmock.DriverControlBlockUntilCancel
	default:
		return false
	}
}

func (a *mockAgent) registerPromptCancel(sessionID string, cancel context.CancelFunc) uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.sessions[sessionID]
	if session == nil {
		session = &sessionState{
			ConfigOptions: cloneSessionConfigOptions(a.configTemplate),
		}
		a.sessions[sessionID] = session
	}
	a.nextCancel++
	cancelID := a.nextCancel
	session.activePromptCancel = cancel
	session.activePromptCancelID = cancelID
	return cancelID
}

func (a *mockAgent) clearPromptCancel(sessionID string, cancelID uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if session := a.sessions[sessionID]; session != nil && session.activePromptCancelID == cancelID {
		session.activePromptCancel = nil
		session.activePromptCancelID = 0
	}
}

func (a *mockAgent) selectTurn(
	sessionID string,
	prompt string,
	promptMeta acp.PromptMeta,
) (acpmock.TurnFixture, int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.sessions[sessionID]
	if session == nil {
		session = &sessionState{}
		a.sessions[sessionID] = session
	}
	promptIndex := session.PromptCount + 1
	turn, err := a.agent.SelectTurn(prompt, promptMeta)
	if err != nil {
		return acpmock.TurnFixture{}, promptIndex, err
	}
	session.PromptCount = promptIndex
	return turn, promptIndex, nil
}

func (a *mockAgent) executeStep(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	step acpmock.Step,
) (acpmock.DiagnosticsStep, error) {
	switch step.Kind {
	case acpmock.StepKindAssistant:
		return a.emitTextChunks(ctx, sessionID, acpsdk.UpdateAgentMessageText, step)
	case acpmock.StepKindThought:
		return a.emitTextChunks(ctx, sessionID, acpsdk.UpdateAgentThoughtText, step)
	case acpmock.StepKindBridgeContent:
		return a.emitTextChunks(ctx, sessionID, acpsdk.UpdateAgentMessageText, step)
	case acpmock.StepKindToolCall:
		return a.emitToolCall(ctx, sessionID, step)
	case acpmock.StepKindPermission:
		return a.requestPermission(ctx, sessionID, step)
	case acpmock.StepKindSandbox:
		return a.executeSandboxCommand(ctx, sessionID, step)
	case acpmock.StepKindDriverControl:
		return a.executeDriverControl(ctx, step)
	default:
		return acpmock.DiagnosticsStep{}, fmt.Errorf("unsupported step kind %s", step.Kind)
	}
}

func (a *mockAgent) emitTextChunks(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	update func(string) acpsdk.SessionUpdate,
	step acpmock.Step,
) (acpmock.DiagnosticsStep, error) {
	chunks := normalizedChunks(step)
	for _, chunk := range chunks {
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: sessionID,
			Update:    update(chunk),
		}); err != nil {
			return acpmock.DiagnosticsStep{}, err
		}
		if err := pauseForDelivery(ctx); err != nil {
			return acpmock.DiagnosticsStep{}, err
		}
	}
	return acpmock.DiagnosticsStep{Kind: step.Kind, Text: strings.Join(chunks, "")}, nil
}

func (a *mockAgent) emitToolCall(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	step acpmock.Step,
) (acpmock.DiagnosticsStep, error) {
	toolCallID := acpsdk.ToolCallId(strings.TrimSpace(step.ToolCallID))
	title := strings.TrimSpace(step.Title)

	startOpts := []acpsdk.ToolCallStartOpt{
		acpsdk.WithStartKind(toolKind(step.ToolKind, acpsdk.ToolKindOther)),
		acpsdk.WithStartStatus(acpsdk.ToolCallStatusPending),
	}
	if locations := toolLocations(step.Path); len(locations) > 0 {
		startOpts = append(startOpts, acpsdk.WithStartLocations(locations))
	}
	if rawInput, ok := parseRawJSON(step.RawInput); ok {
		startOpts = append(startOpts, acpsdk.WithStartRawInput(rawInput))
	}

	if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
		SessionId: sessionID,
		Update:    acpsdk.StartToolCall(toolCallID, title, startOpts...),
	}); err != nil {
		return acpmock.DiagnosticsStep{}, err
	}
	if err := pauseForDelivery(ctx); err != nil {
		return acpmock.DiagnosticsStep{}, err
	}

	finalStatus := strings.TrimSpace(step.Status)
	rawOutput, hasRawOutput := parseRawJSON(step.RawOutput)
	hasFinalUpdate := finalStatus != "" || strings.TrimSpace(step.ContentText) != "" || hasRawOutput
	if hasFinalUpdate {
		updateOpts := make([]acpsdk.ToolCallUpdateOpt, 0, 4)
		if finalStatus != "" {
			updateOpts = append(updateOpts, acpsdk.WithUpdateStatus(acpsdk.ToolCallStatus(finalStatus)))
		}
		if title != "" {
			updateOpts = append(updateOpts, acpsdk.WithUpdateTitle(title))
		}
		if text := strings.TrimSpace(step.ContentText); text != "" {
			updateOpts = append(updateOpts, acpsdk.WithUpdateContent(textToolContent(text)))
		}
		if hasRawOutput {
			updateOpts = append(updateOpts, acpsdk.WithUpdateRawOutput(rawOutput))
		}
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: sessionID,
			Update:    acpsdk.UpdateToolCall(toolCallID, updateOpts...),
		}); err != nil {
			return acpmock.DiagnosticsStep{}, err
		}
		if err := pauseForDelivery(ctx); err != nil {
			return acpmock.DiagnosticsStep{}, err
		}
	}

	return acpmock.DiagnosticsStep{
		Kind:       acpmock.StepKindToolCall,
		ToolCallID: string(toolCallID),
	}, nil
}

func (a *mockAgent) requestPermission(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	step acpmock.Step,
) (acpmock.DiagnosticsStep, error) {
	title := strings.TrimSpace(step.Title)
	if title == "" {
		title = "permission request"
	}
	toolKindValue := toolKind(step.ToolKind, acpsdk.ToolKindOther)
	statusValue := toolStatus(step.Status, acpsdk.ToolCallStatusPending)

	response, err := a.conn.RequestPermission(ctx, acpsdk.RequestPermissionRequest{
		SessionId: sessionID,
		Options:   defaultPermissionOptions(),
		ToolCall: acpsdk.ToolCallUpdate{
			ToolCallId: acpsdk.ToolCallId(strings.TrimSpace(step.ToolCallID)),
			Title:      new(title),
			Kind:       new(toolKindValue),
			Status:     new(statusValue),
			Locations:  toolLocations(step.Path),
			RawInput:   mustRawJSON(step.RawInput),
			RawOutput:  mustRawJSON(step.RawOutput),
		},
	})
	if err != nil {
		return acpmock.DiagnosticsStep{}, err
	}

	decision := selectedDecision(response)
	if expected := strings.TrimSpace(step.ExpectDecision); expected != "" && decision != expected {
		return acpmock.DiagnosticsStep{}, fmt.Errorf(
			"permission decision %s did not match expected %s",
			decision,
			expected,
		)
	}

	switch {
	case step.EmitDecision:
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: sessionID,
			Update:    acpsdk.UpdateAgentMessageText(decision),
		}); err != nil {
			return acpmock.DiagnosticsStep{}, err
		}
		if err := pauseForDelivery(ctx); err != nil {
			return acpmock.DiagnosticsStep{}, err
		}
	case strings.TrimSpace(step.EmitText) != "":
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: sessionID,
			Update:    acpsdk.UpdateAgentMessageText(strings.TrimSpace(step.EmitText)),
		}); err != nil {
			return acpmock.DiagnosticsStep{}, err
		}
		if err := pauseForDelivery(ctx); err != nil {
			return acpmock.DiagnosticsStep{}, err
		}
	}

	return acpmock.DiagnosticsStep{
		Kind:       acpmock.StepKindPermission,
		ToolCallID: strings.TrimSpace(step.ToolCallID),
		Decision:   decision,
	}, nil
}
