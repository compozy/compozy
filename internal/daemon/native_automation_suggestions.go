package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	automationpkg "github.com/compozy/compozy/internal/automation"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type automationSuggestionsListInput struct {
	WorkspaceID string `json:"workspace"`
	Status      string `json:"status,omitempty"`
}

type automationSuggestionActionInput struct {
	WorkspaceID  string `json:"workspace"`
	SuggestionID string `json:"suggestion_id"`
}

func (n *daemonNativeTools) automationSuggestionsList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationSuggestionsListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := requiredNativeString(req.ToolID, nativeWorkspaceInputKey, input.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	status := automationpkg.SuggestionStatus(strings.TrimSpace(input.Status))
	if status == "" {
		status = automationpkg.SuggestionStatusPending
	}
	if err := status.Validate("status"); err != nil {
		return toolspkg.ToolResult{}, nativeAutomationValidationError(req.ToolID, err)
	}
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	suggestions, err := n.automationManager().ListSuggestions(ctx, readScope, workspaceID, status)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := core.AutomationSuggestionPayloadsFromSuggestions(suggestions)
	return structuredResult(
		contract.AutomationSuggestionsResponse{Suggestions: payload},
		fmt.Sprintf("%d automation suggestions", len(payload)),
	)
}

func (n *daemonNativeTools) automationSuggestionsAccept(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	input, err := decodeAutomationSuggestionActionInput(req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	accepted, err := n.automationManager().AcceptSuggestion(
		ctx,
		readScope.ProfileID,
		input.WorkspaceID,
		input.SuggestionID,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	response := contract.AutomationSuggestionAcceptanceResponse{
		Suggestion: core.AutomationSuggestionPayloadFromSuggestion(accepted.Suggestion),
		Job:        core.JobPayloadFromJob(accepted.Job, nil, nil),
	}
	return structuredResult(response, accepted.Job.ID)
}

func (n *daemonNativeTools) automationSuggestionsDismiss(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	input, err := decodeAutomationSuggestionActionInput(req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	dismissed, err := n.automationManager().DismissSuggestion(
		ctx,
		readScope.ProfileID,
		input.WorkspaceID,
		input.SuggestionID,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	response := contract.AutomationSuggestionResponse{
		Suggestion: core.AutomationSuggestionPayloadFromSuggestion(dismissed),
	}
	return structuredResult(response, dismissed.ID)
}

func decodeAutomationSuggestionActionInput(
	req toolspkg.CallRequest,
) (automationSuggestionActionInput, error) {
	var input automationSuggestionActionInput
	if err := decodeNativeInput(req, &input); err != nil {
		return automationSuggestionActionInput{}, err
	}
	workspaceID, err := requiredNativeString(req.ToolID, nativeWorkspaceInputKey, input.WorkspaceID)
	if err != nil {
		return automationSuggestionActionInput{}, err
	}
	suggestionID, err := requiredNativeString(req.ToolID, "suggestion_id", input.SuggestionID)
	if err != nil {
		return automationSuggestionActionInput{}, err
	}
	input.WorkspaceID = workspaceID
	input.SuggestionID = suggestionID
	return input, nil
}
