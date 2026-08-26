package daemon

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeCallTask struct {
	Agent           string              `json:"agent"`
	SessionID       string              `json:"session_id"`
	Prompt          string              `json:"prompt"`
	Expect          json.RawMessage     `json:"expect"`
	IdleTTLSeconds  int64               `json:"idle_ttl_seconds"`
	DeadlineSeconds json.RawMessage     `json:"deadline_seconds"`
	Strict          bool                `json:"strict"`
	ResultBudget    string              `json:"result_budget"`
	ResultOverflow  string              `json:"result_overflow"`
	IdempotencyKey  string              `json:"idempotency_key"`
	Runtime         *nativeCallRuntime  `json:"runtime"`
	Narrow          nativeCallNarrowing `json:"narrow"`
}

type nativeCallInput struct {
	nativeCallTask
	Tasks *[]nativeCallTask `json:"tasks"`
}

type nativeCallRuntime struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	Speed           string `json:"speed"`
}

type nativeCallNarrowing struct {
	Tools           []string `json:"tools"`
	Skills          []string `json:"skills"`
	MCPServers      []string `json:"mcp_servers"`
	WorkspacePaths  []string `json:"workspace_paths"`
	NetworkChannels []string `json:"network_channels"`
	SandboxProfiles []string `json:"sandbox_profiles"`
}

type nativeCallReturnInput struct {
	CallID    string          `json:"call_id"`
	Result    json.RawMessage `json:"result"`
	FinalText string          `json:"final_text"`
}

type nativeCallAwaitInput struct {
	CallIDs   []string `json:"call_ids"`
	TimeoutMS int64    `json:"timeout_ms"`
	Resume    string   `json:"resume"`
}

type nativeCallCancelInput struct {
	CallID string `json:"call_id"`
	Reason string `json:"reason"`
}

type nativeCallIDInput struct {
	CallID string `json:"call_id"`
}

type nativeCallPublishInput struct {
	CallID   string `json:"call_id"`
	Channel  string `json:"channel"`
	ThreadID string `json:"thread_id"`
}

type nativeAgentMessageInput struct {
	To     string `json:"to"`
	Text   string `json:"text"`
	CallID string `json:"call_id"`
}

func (n *daemonNativeTools) agentCall(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	operationCtx := context.WithoutCancel(ctx)
	service, err := n.requireCallsService(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	var input nativeCallInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.Tasks != nil {
		if hasNativeInlineCall(input.nativeCallTask) {
			return toolspkg.ToolResult{}, &callspkg.Error{
				Code: callspkg.CodeValidation, Message: "tasks cannot be combined with inline call fields",
			}
		}
		creates := make([]callspkg.CreateInput, 0, len(*input.Tasks))
		for _, task := range *input.Tasks {
			create, createErr := n.nativeCreateCallInput(scope, task)
			if createErr != nil {
				return toolspkg.ToolResult{}, createErr
			}
			creates = append(creates, create)
		}
		outcomes, batchErr := service.CreateBatch(operationCtx, creates)
		if batchErr != nil {
			return toolspkg.ToolResult{}, batchErr
		}
		items := make([]map[string]any, 0, len(outcomes))
		for _, outcome := range outcomes {
			item := make(map[string]any)
			if outcome.Call != nil {
				item["call"] = nativeCallRecord(*outcome.Call)
			} else if outcome.Error != nil {
				item["error"] = outcome.Error
			}
			items = append(items, item)
		}
		return structuredNetworkResult(map[string]any{"items": items}, "accepted call batch")
	}
	create, err := n.nativeCreateCallInput(scope, input.nativeCallTask)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	record, err := service.Create(operationCtx, create)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredNetworkResult(nativeCallRecord(record), record.CallID+" "+string(record.State))
}

func (n *daemonNativeTools) nativeCreateCallInput(
	scope toolspkg.Scope,
	input nativeCallTask,
) (callspkg.CreateInput, error) {
	callScope := nativeCallsScope(scope)
	create := callspkg.CreateInput{
		ProfileID: callScope.ProfileID, Scope: callScope.Scope, WorkspaceID: callScope.WorkspaceID,
		Caller: participation.OwnerRef{
			Kind: participation.OwnerKindSession, ID: strings.TrimSpace(scope.SessionID),
			WorkspaceID: callScope.WorkspaceID,
		},
		Target: callspkg.Target{Agent: input.Agent, SessionID: input.SessionID},
		Prompt: input.Prompt, Expect: append(json.RawMessage(nil), input.Expect...), Strict: input.Strict,
		IdempotencyKey: input.IdempotencyKey,
		Actor:          callspkg.Actor{Kind: "agent_session", ID: strings.TrimSpace(scope.SessionID)},
		Narrow: callspkg.PermissionAtoms{
			Tools: input.Narrow.Tools, Skills: input.Narrow.Skills, MCPServers: input.Narrow.MCPServers,
			WorkspacePaths: input.Narrow.WorkspacePaths, NetworkChannels: input.Narrow.NetworkChannels,
			SandboxProfiles: input.Narrow.SandboxProfiles,
		},
	}
	if input.IdleTTLSeconds > 0 {
		create.IdleTTL = time.Duration(input.IdleTTLSeconds) * time.Second
	}
	if len(input.DeadlineSeconds) > 0 {
		var seconds int64
		if err := json.Unmarshal(input.DeadlineSeconds, &seconds); err != nil || seconds <= 0 {
			return callspkg.CreateInput{}, &callspkg.Error{
				Code: callspkg.CodeDeadlineInvalid, Message: "deadline_seconds must be a positive integer",
			}
		}
		deadline := time.Now().UTC().Add(time.Duration(seconds) * time.Second)
		create.Deadline = &deadline
	}
	if input.ResultBudget != "" || input.ResultOverflow != "" {
		budgetRaw := strings.TrimSpace(input.ResultBudget)
		if budgetRaw == "" {
			budgetRaw = n.deps.Config.Calls.Results.DefaultBudget
		}
		bytes, err := config.ParseByteSize(budgetRaw)
		if err != nil {
			return callspkg.CreateInput{}, &callspkg.Error{Code: callspkg.CodeValidation, Message: err.Error()}
		}
		overflow := contracts.OverflowMode(strings.TrimSpace(input.ResultOverflow))
		if overflow == "" {
			overflow = contracts.OverflowMode(n.deps.Config.Calls.Results.Overflow)
		}
		create.ResultBudget = &contracts.ByteBudget{MaxBytes: bytes, Overflow: overflow}
	}
	if input.Runtime != nil {
		runtime := callspkg.RuntimeSpec{
			Provider: input.Runtime.Provider, Model: input.Runtime.Model,
			ReasoningEffort: input.Runtime.ReasoningEffort,
		}
		if raw := strings.TrimSpace(input.Runtime.Speed); raw != "" {
			parsed, err := speed.Parse(raw)
			if err != nil {
				return callspkg.CreateInput{}, &callspkg.Error{Code: callspkg.CodeValidation, Message: err.Error()}
			}
			runtime.Speed = parsed
		}
		create.Runtime = &runtime
	}
	return create, nil
}

func hasNativeInlineCall(input nativeCallTask) bool {
	return strings.TrimSpace(input.Agent) != "" || strings.TrimSpace(input.SessionID) != "" ||
		strings.TrimSpace(input.Prompt) != "" || len(input.Expect) > 0 || input.IdleTTLSeconds != 0 ||
		len(input.DeadlineSeconds) > 0 || input.Strict || strings.TrimSpace(input.ResultBudget) != "" ||
		strings.TrimSpace(input.ResultOverflow) != "" || strings.TrimSpace(input.IdempotencyKey) != "" ||
		input.Runtime != nil || len(input.Narrow.Tools) > 0 || len(input.Narrow.Skills) > 0 ||
		len(input.Narrow.MCPServers) > 0 || len(input.Narrow.WorkspacePaths) > 0 ||
		len(input.Narrow.NetworkChannels) > 0 || len(input.Narrow.SandboxProfiles) > 0
}

func (n *daemonNativeTools) callReturn(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	operationCtx := context.WithoutCancel(ctx)
	service, err := n.requireCallsService(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	var input nativeCallReturnInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	settlement, err := service.Return(operationCtx, callspkg.ReturnInput{
		CallID: input.CallID, ChildSessionID: scope.SessionID, Result: input.Result,
		FinalText: input.FinalText, ChildLive: true,
		Actor: callspkg.SettlementActor{Kind: "agent_session", ID: scope.SessionID},
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := map[string]any{
		"call_id": settlement.Call.CallID,
		"state":   settlement.Call.State,
	}
	if settlement.RepairPrompt != "" {
		payload["repair_prompt"] = settlement.RepairPrompt
		payload["issues"] = settlement.Issues
	}
	return structuredNetworkResult(payload, settlement.Call.CallID+" "+string(settlement.Call.State))
}

func (n *daemonNativeTools) callAwait(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	service, err := n.requireCallsService(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	var input nativeCallAwaitInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	callScope := nativeCallsScope(scope)
	outcome, err := service.Await(ctx, callspkg.AwaitInput{
		ProfileID: callScope.ProfileID, Scope: callScope.Scope, WorkspaceID: callScope.WorkspaceID,
		CallIDs: input.CallIDs, Timeout: time.Duration(input.TimeoutMS) * time.Millisecond, Resume: input.Resume,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	settled := make([]map[string]any, 0, len(outcome.Settled))
	for _, record := range outcome.Settled {
		item := nativeCallRecord(record)
		if record.State == callspkg.StateCompleted && strings.TrimSpace(record.ResultRef) != "" {
			result, resultErr := service.Result(ctx, callspkg.CallReadQuery{
				ReadScope: store.ReadScope{ProfileID: callScope.ProfileID}, Scope: callScope.Scope,
				WorkspaceID: callScope.WorkspaceID,
			}, record.CallID)
			if resultErr != nil {
				return toolspkg.ToolResult{}, resultErr
			}
			item["result_preview"] = nativeBoundedResultPreview(result.Bytes, record.ResultBudget.MaxBytes)
		}
		settled = append(settled, item)
	}
	return structuredNetworkResult(map[string]any{
		"settled": settled, "pending": outcome.Pending, "outcome": outcome.Outcome,
		"resume": outcome.Resume, "clamped_timeout_ms": outcome.ClampedTimeout.Milliseconds(),
	}, outcome.Outcome)
}

func (n *daemonNativeTools) callCancel(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	operationCtx := context.WithoutCancel(ctx)
	service, err := n.requireCallsService(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	var input nativeCallCancelInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	callScope := nativeCallsScope(scope)
	if _, err := service.GetRead(operationCtx, callspkg.CallReadQuery{
		ReadScope: store.ReadScope{ProfileID: callScope.ProfileID}, Scope: callScope.Scope,
		WorkspaceID: callScope.WorkspaceID,
	}, input.CallID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	record, err := service.Cancel(operationCtx, input.CallID, input.Reason, callspkg.Actor{
		Kind: "agent_session", ID: scope.SessionID,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(nativeCallRecord(record), string(record.State))
}

func (n *daemonNativeTools) callResult(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	service, err := n.requireCallsService(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	var input nativeCallIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	callScope := nativeCallsScope(scope)
	result, err := service.Result(ctx, callspkg.CallReadQuery{
		ReadScope: store.ReadScope{ProfileID: callScope.ProfileID}, Scope: callScope.Scope,
		WorkspaceID: callScope.WorkspaceID,
	}, input.CallID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredNetworkResult(json.RawMessage(result.Bytes), result.CallID)
}

func nativeBoundedResultPreview(payload []byte, limit int) json.RawMessage {
	if limit <= 0 || limit > 64<<10 {
		limit = 64 << 10
	}
	if len(payload) > limit {
		return nil
	}
	return append(json.RawMessage(nil), payload...)
}

func (n *daemonNativeTools) agentMessage(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	operationCtx := context.WithoutCancel(ctx)
	service, err := n.requireCallsService(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	var input nativeAgentMessageInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	callScope := nativeCallsScope(scope)
	record, err := service.SendMessage(operationCtx, callspkg.SendMessageInput{
		ProfileID: callScope.ProfileID, Scope: callScope.Scope, WorkspaceID: callScope.WorkspaceID,
		From: callspkg.MessageSender{Kind: "session", ID: scope.SessionID}, To: input.To,
		CallID: input.CallID, Body: input.Text,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredNetworkResult(map[string]any{
		"message_id": record.MessageID, "delivery": record.Delivery,
	}, record.MessageID+" "+record.Delivery)
}

func (n *daemonNativeTools) callPublish(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	operationCtx := context.WithoutCancel(ctx)
	service, err := n.requireCallsService(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	var input nativeCallPublishInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	callScope := nativeCallsScope(scope)
	receipt, err := service.Publish(operationCtx, callspkg.PublishInput{
		ProfileID: callScope.ProfileID, Scope: callScope.Scope, WorkspaceID: callScope.WorkspaceID,
		CallID: input.CallID, Actor: callspkg.Actor{Kind: "agent_session", ID: scope.SessionID},
		Channel: input.Channel, ThreadID: input.ThreadID,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredNetworkResult(map[string]any{
		"network_message_id": receipt.NetworkMessageID,
		"published":          receipt.Published,
	}, receipt.NetworkMessageID)
}

func (n *daemonNativeTools) requireCallsService(toolID toolspkg.ToolID) (coreCallsService, error) {
	service := n.callsService()
	if service == nil {
		return nil, nativeUnavailableError(toolID, "calls service is unavailable")
	}
	return service, nil
}

type coreCallsService interface {
	Create(context.Context, callspkg.CreateInput) (callspkg.CallRecord, error)
	CreateBatch(context.Context, []callspkg.CreateInput) ([]callspkg.BatchOutcome, error)
	Return(context.Context, callspkg.ReturnInput) (callspkg.Settlement, error)
	GetRead(context.Context, callspkg.CallReadQuery, string) (callspkg.CallRecord, error)
	Result(context.Context, callspkg.CallReadQuery, string) (callspkg.ResultPayload, error)
	Await(context.Context, callspkg.AwaitInput) (callspkg.AwaitOutcome, error)
	Cancel(context.Context, string, string, callspkg.Actor) (callspkg.CallRecord, error)
	SendMessage(context.Context, callspkg.SendMessageInput) (callspkg.MessageRecord, error)
	Publish(context.Context, callspkg.PublishInput) (callspkg.PublishReceipt, error)
}

func nativeCallsScope(scope toolspkg.Scope) callspkg.CallScope {
	result := callspkg.CallScope{ProfileID: strings.TrimSpace(scope.ProfileID), WorkspaceID: strings.TrimSpace(scope.WorkspaceID)}
	if result.WorkspaceID == "" {
		result.Scope = callspkg.ScopeGlobal
	} else {
		result.Scope = callspkg.ScopeWorkspace
	}
	return result
}

func nativeCallRecord(record callspkg.CallRecord) map[string]any {
	return map[string]any{
		"call_id": record.CallID, "profile_id": record.ProfileID, "scope": record.Scope,
		"workspace_id": record.WorkspaceID, "caller": record.Caller, "actor": record.Actor,
		"agent": record.AgentName, "child_session_id": record.ChildSessionID,
		"parent_session_id": record.ParentSessionID, "root_session_id": record.GovernedRootID,
		"depth": record.Depth, "state": record.State, "verdict": record.Verdict,
		"expect_digest": record.ExpectDigest, "result_bytes": record.ResultBytes,
		"result_budget_bytes": record.ResultBudget.MaxBytes, "result_overflow": record.ResultBudget.Overflow,
		"strict": record.Strict, "failure_code": record.FailureCode,
		"repair_attempts": record.RepairAttempts, "replayed": record.Replayed,
		"deadline_at": record.DeadlineAt, "created_at": record.CreatedAt,
		"started_at": record.StartedAt, "settled_at": record.SettledAt, "updated_at": record.UpdatedAt,
	}
}
