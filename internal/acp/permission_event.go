package acp

import (
	"encoding/json"

	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

func selectPermissionOutcome(
	options []acpsdk.PermissionOption,
	decision permissionDecision,
) (acpsdk.RequestPermissionOutcome, permissionDecision) {
	var preferred []acpsdk.PermissionOptionKind
	switch decision {
	case decisionAllowAlways:
		preferred = []acpsdk.PermissionOptionKind{
			acpsdk.PermissionOptionKindAllowAlways,
			acpsdk.PermissionOptionKindAllowOnce,
		}
	case decisionAllowOnce:
		preferred = []acpsdk.PermissionOptionKind{
			acpsdk.PermissionOptionKindAllowOnce,
			acpsdk.PermissionOptionKindAllowAlways,
		}
	case decisionRejectAlways:
		preferred = []acpsdk.PermissionOptionKind{
			acpsdk.PermissionOptionKindRejectAlways,
			acpsdk.PermissionOptionKindRejectOnce,
		}
	case decisionRejectOnce:
		preferred = []acpsdk.PermissionOptionKind{
			acpsdk.PermissionOptionKindRejectOnce,
			acpsdk.PermissionOptionKindRejectAlways,
		}
	default:
		return acpsdk.NewRequestPermissionOutcomeCancelled(), ""
	}

	for _, kind := range preferred {
		for _, option := range options {
			if option.Kind == kind {
				return acpsdk.NewRequestPermissionOutcomeSelected(option.OptionId), permissionDecisionFromKind(kind)
			}
		}
	}

	return acpsdk.NewRequestPermissionOutcomeCancelled(), ""
}

func permissionDecisionFromKind(kind acpsdk.PermissionOptionKind) permissionDecision {
	switch kind {
	case acpsdk.PermissionOptionKindAllowOnce:
		return decisionAllowOnce
	case acpsdk.PermissionOptionKindAllowAlways:
		return decisionAllowAlways
	case acpsdk.PermissionOptionKindRejectOnce:
		return decisionRejectOnce
	case acpsdk.PermissionOptionKindRejectAlways:
		return decisionRejectAlways
	default:
		return ""
	}
}

func supportedPermissionDecisions(options []acpsdk.PermissionOption) map[permissionDecision]struct{} {
	supported := make(map[permissionDecision]struct{}, len(options))
	for _, option := range options {
		decision := permissionDecisionFromKind(option.Kind)
		if decision != "" {
			supported[decision] = struct{}{}
		}
	}
	return supported
}

func buildPermissionEventRaw(
	requestID string,
	decision permissionDecision,
	request acpsdk.RequestPermissionRequest,
) json.RawMessage {
	options := make([]permissionEventOption, 0, len(request.Options))
	for _, option := range request.Options {
		options = append(options, permissionEventOption{
			Decision: string(permissionDecisionFromKind(option.Kind)),
			OptionID: string(option.OptionId),
			Kind:     string(option.Kind),
			Label:    option.Name,
		})
	}

	var toolInput json.RawMessage
	if request.ToolCall.RawInput != nil {
		if marshaled, err := json.Marshal(request.ToolCall.RawInput); err == nil {
			toolInput = marshaled
		}
	}

	toolCall := permissionToolCallRaw{
		ID:        strings.TrimSpace(string(request.ToolCall.ToolCallId)),
		Locations: append([]acpsdk.ToolCallLocation(nil), request.ToolCall.Locations...),
	}
	if request.ToolCall.Kind != nil {
		toolCall.Kind = string(*request.ToolCall.Kind)
	}
	if request.ToolCall.Title != nil {
		toolCall.Title = strings.TrimSpace(*request.ToolCall.Title)
	}
	if request.ToolCall.Status != nil {
		toolCall.Status = string(*request.ToolCall.Status)
	}

	payload := permissionEventRaw{
		RequestID: requestID,
		Options:   options,
		ToolInput: toolInput,
		ToolCall:  toolCall,
	}
	if decision != "" && decision != decisionPending {
		payload.Decision = string(decision)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fallbackPermissionEventRaw(requestID, decision)
	}
	return data
}
