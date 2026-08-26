package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

func callReadQuery(c *gin.Context, readScope store.ReadScope) (callspkg.CallReadQuery, error) {
	scope, workspaceID, err := callSurfaceScope(c, "", "")
	if err != nil {
		return callspkg.CallReadQuery{}, err
	}
	return callspkg.CallReadQuery{ReadScope: readScope, Scope: scope, WorkspaceID: workspaceID}, nil
}

func callSurfaceScope(c *gin.Context, requestedScope, requestedWorkspace string) (callspkg.Scope, string, error) {
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	if bodyWorkspace := strings.TrimSpace(requestedWorkspace); bodyWorkspace != "" {
		if workspaceID != "" && workspaceID != bodyWorkspace {
			return "", "", callRequestError(callspkg.CodeWorkspaceDenied, "route and body workspace_id differ")
		}
		workspaceID = bodyWorkspace
	}
	scope := callspkg.Scope(strings.TrimSpace(requestedScope))
	if scope == "" {
		if workspaceID == "" {
			scope = callspkg.ScopeGlobal
		} else {
			scope = callspkg.ScopeWorkspace
		}
	}
	if scope == callspkg.ScopeGlobal && workspaceID != "" || scope == callspkg.ScopeWorkspace && workspaceID == "" {
		return "", "", callRequestError(callspkg.CodeValidation, "scope and workspace_id do not match")
	}
	return scope, workspaceID, nil
}

func (h *BaseHandlers) createCallInputs(
	c *gin.Context,
	req contract.CreateCallRequest,
	profileID string,
	caller participation.OwnerRef,
	actor callspkg.Actor,
) ([]callspkg.CreateInput, bool, error) {
	scope, workspaceID, err := callSurfaceScope(c, req.Scope, req.WorkspaceID)
	if err != nil {
		return nil, false, err
	}
	items, batch := []contract.CreateCallItemRequest{req.CreateCallItemRequest}, false
	if req.TasksPresent || len(req.Tasks) > 0 {
		items, batch = req.Tasks, true
		if hasInlineCallItem(req.CreateCallItemRequest) {
			return nil, false, callRequestError(callspkg.CodeValidation, "tasks cannot be combined with inline call fields")
		}
	}
	inputs := make([]callspkg.CreateInput, 0, len(items))
	for _, item := range items {
		input, mapErr := h.createCallInput(item, profileID, scope, workspaceID, caller, actor)
		if mapErr != nil {
			return nil, batch, mapErr
		}
		inputs = append(inputs, input)
	}
	return inputs, batch, nil
}

func (h *BaseHandlers) createCallInput(
	item contract.CreateCallItemRequest,
	profileID string,
	scope callspkg.Scope,
	workspaceID string,
	caller participation.OwnerRef,
	actor callspkg.Actor,
) (callspkg.CreateInput, error) {
	input := callspkg.CreateInput{
		ProfileID: profileID, Scope: scope, WorkspaceID: workspaceID, Caller: caller,
		Target: callspkg.Target{Agent: item.Target.Agent, SessionID: item.Target.SessionID},
		Prompt: item.Prompt, Expect: cloneCallJSON(item.Expect), Strict: item.Strict,
		IdempotencyKey: item.IdempotencyKey, Actor: actor,
		Narrow: callspkg.PermissionAtoms{
			Tools: item.Narrow.Tools, Skills: item.Narrow.Skills, MCPServers: item.Narrow.MCPServers,
			WorkspacePaths: item.Narrow.WorkspacePaths, NetworkChannels: item.Narrow.NetworkChannels,
			SandboxProfiles: item.Narrow.SandboxProfiles,
		},
	}
	if item.IdleTTLSeconds != nil {
		if *item.IdleTTLSeconds <= 0 {
			return callspkg.CreateInput{}, callRequestError(callspkg.CodeValidation, "idle_ttl_seconds must be positive")
		}
		input.IdleTTL = time.Duration(*item.IdleTTLSeconds) * time.Second
	}
	if item.DeadlineSeconds != nil {
		if *item.DeadlineSeconds <= 0 {
			return callspkg.CreateInput{}, callRequestError(callspkg.CodeDeadlineInvalid, "deadline_seconds must be positive")
		}
		deadline := h.nowUTC().Add(time.Duration(*item.DeadlineSeconds) * time.Second)
		input.Deadline = &deadline
	}
	if strings.TrimSpace(item.ResultBudget) != "" || strings.TrimSpace(item.ResultOverflow) != "" {
		budgetRaw := strings.TrimSpace(item.ResultBudget)
		if budgetRaw == "" {
			budgetRaw = h.Config.Calls.Results.DefaultBudget
		}
		budget, err := config.ParseByteSize(budgetRaw)
		if err != nil {
			return callspkg.CreateInput{}, callRequestError(callspkg.CodeValidation, fmt.Sprintf("invalid result_budget: %v", err))
		}
		overflow := contracts.OverflowMode(strings.TrimSpace(item.ResultOverflow))
		if overflow == "" {
			overflow = contracts.OverflowMode(h.Config.Calls.Results.Overflow)
		}
		input.ResultBudget = &contracts.ByteBudget{MaxBytes: budget, Overflow: overflow}
	}
	if item.Runtime != nil {
		runtime := callspkg.RuntimeSpec{
			Provider: item.Runtime.Provider, Model: item.Runtime.Model,
			ReasoningEffort: item.Runtime.ReasoningEffort,
		}
		if raw := strings.TrimSpace(item.Runtime.Speed); raw != "" {
			parsed, err := speed.Parse(raw)
			if err != nil {
				return callspkg.CreateInput{}, callRequestError(callspkg.CodeValidation, err.Error())
			}
			runtime.Speed = parsed
		}
		input.Runtime = &runtime
	}
	return input, nil
}

func hasInlineCallItem(item contract.CreateCallItemRequest) bool {
	return item.Target.Agent != "" || item.Target.SessionID != "" || item.Prompt != "" || len(item.Expect) > 0 ||
		item.IdleTTLSeconds != nil || item.DeadlineSeconds != nil || item.Strict || item.ResultBudget != "" ||
		item.ResultOverflow != "" || item.IdempotencyKey != "" || item.Runtime != nil ||
		len(item.Narrow.Tools) > 0 || len(item.Narrow.Skills) > 0 || len(item.Narrow.MCPServers) > 0 ||
		len(item.Narrow.WorkspacePaths) > 0 || len(item.Narrow.NetworkChannels) > 0 ||
		len(item.Narrow.SandboxProfiles) > 0
}

func callRequestError(code callspkg.ErrorCode, message string) error {
	return &callspkg.Error{Code: code, Message: strings.TrimSpace(message)}
}

func parseCallStates(values []string) ([]callspkg.State, error) {
	states := make([]callspkg.State, 0, len(values))
	for _, raw := range values {
		for _, value := range strings.Split(raw, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			states = append(states, callspkg.State(value))
		}
	}
	return states, nil
}

func decodeCallBody(c *gin.Context, target any) error {
	if err := decodeStrictJSONBody(c, target); err != nil {
		if strings.Contains(err.Error(), "deadline_seconds") {
			return callRequestError(callspkg.CodeDeadlineInvalid, "deadline_seconds must be a positive integer")
		}
		return callRequestError(callspkg.CodeValidation, err.Error())
	}
	return nil
}
