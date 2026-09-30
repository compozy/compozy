package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionContinueInput struct {
	Workspace           string                                  `json:"workspace,omitempty"`
	SessionID           string                                  `json:"session_id"`
	Agent               string                                  `json:"agent"`
	Runtime             *contract.PromptRuntimeSelectionPayload `json:"runtime,omitempty"`
	Route               int                                     `json:"route,omitempty"`
	Name                string                                  `json:"name,omitempty"`
	Message             string                                  `json:"message,omitempty"`
	IdempotencyKey      string                                  `json:"idempotency_key"`
	ExpectedEpoch       *int64                                  `json:"expected_epoch,omitempty"`
	ExpectedGeneration  *int64                                  `json:"expected_generation,omitempty"`
	ExpectedMaxSequence *int64                                  `json:"expected_max_sequence,omitempty"`
}

// sessionContinue binds compozy__session_continue to the manager's derive primitive;
// the invariants (snapshot, receipt, admission) are owned by session.Manager.
func (n *daemonNativeTools) sessionContinue(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionContinueInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	agent, err := requiredNativeString(req.ToolID, "agent", input.Agent)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	idempotencyKey, err := requiredNativeString(req.ToolID, "idempotency_key", input.IdempotencyKey)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.Runtime != nil && input.Route > 0 {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID,
			errors.New("runtime and route are mutually exclusive"))
	}
	if err := nativeDeriveFencesComplete(input.ExpectedEpoch, input.ExpectedGeneration,
		input.ExpectedMaxSequence); err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	deriver, workspaceID, err := n.nativeDeriveTarget(ctx, scope, req.ToolID, input.Workspace)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := deriver.ContinueSession(ctx, session.ContinueSessionOpts{
		SourceSessionID: sessionID,
		WorkspaceID:     workspaceID,
		AgentName:       agent,
		Runtime:         nativeDeriveRuntime(input.Runtime),
		Route:           input.Route,
		Name:            strings.TrimSpace(input.Name),
		Message:         input.Message,
		IdempotencyKey:  idempotencyKey,
		Fences: session.DeriveFences{
			ExpectedEpoch: input.ExpectedEpoch, ExpectedGeneration: input.ExpectedGeneration,
			ExpectedMaxSequence: input.ExpectedMaxSequence,
		},
	})
	return nativeDeriveResult(req.ToolID, result, err)
}

type sessionForkInput struct {
	Workspace           string `json:"workspace,omitempty"`
	SessionID           string `json:"session_id"`
	MessageID           string `json:"message_id,omitempty"`
	Name                string `json:"name,omitempty"`
	IdempotencyKey      string `json:"idempotency_key"`
	ExpectedEpoch       *int64 `json:"expected_epoch,omitempty"`
	ExpectedGeneration  *int64 `json:"expected_generation,omitempty"`
	ExpectedMaxSequence *int64 `json:"expected_max_sequence,omitempty"`
}

// sessionFork binds compozy__session_fork to the manager's derive primitive; the
// invariants (snapshot, cut, native gate, receipt) are owned by session.Manager.
func (n *daemonNativeTools) sessionFork(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionForkInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	idempotencyKey, err := requiredNativeString(req.ToolID, "idempotency_key", input.IdempotencyKey)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := nativeDeriveFencesComplete(input.ExpectedEpoch, input.ExpectedGeneration,
		input.ExpectedMaxSequence); err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	deriver, workspaceID, err := n.nativeDeriveTarget(ctx, scope, req.ToolID, input.Workspace)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := deriver.ForkSession(ctx, session.ForkSessionOpts{
		SourceSessionID: sessionID,
		WorkspaceID:     workspaceID,
		MessageID:       strings.TrimSpace(input.MessageID),
		Name:            strings.TrimSpace(input.Name),
		IdempotencyKey:  idempotencyKey,
		Fences: session.DeriveFences{
			ExpectedEpoch: input.ExpectedEpoch, ExpectedGeneration: input.ExpectedGeneration,
			ExpectedMaxSequence: input.ExpectedMaxSequence,
		},
	})
	return nativeDeriveResult(req.ToolID, result, err)
}

// nativeDeriveTarget resolves the derive manager and the caller-visible workspace. It
// does not read the source: the repairing Status read would rewrite an inactive
// source, and the manager validates ownership with its non-repairing snapshot after
// replaying a matching receipt, so a retry survives the source's deletion (ADR-007).
func (n *daemonNativeTools) nativeDeriveTarget(
	ctx context.Context,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
	workspace string,
) (core.SessionDeriveManager, string, error) {
	deriver, ok := n.deps.Sessions.(core.SessionDeriveManager)
	if !ok {
		return nil, "", errors.New("daemon: session continue and fork are unavailable")
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, toolID, workspace, scope)
	if err != nil {
		return nil, "", err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return nil, "", nativeInputError(toolID, err)
	}
	return deriver, workspaceID, nil
}

// nativeDeriveResult projects a derive outcome. A failure keeps the HTTP/UDS error
// payload (code and, after the child was committed, child_session_id) as the tool
// error's partial result, so the calling agent can still reach the committed child.
func nativeDeriveResult(
	toolID toolspkg.ToolID,
	result session.DeriveResult,
	err error,
) (toolspkg.ToolResult, error) {
	if err != nil {
		return toolspkg.ToolResult{}, nativeDeriveToolError(toolID, core.DeriveFailure(err, result))
	}
	payload := contract.SessionDeriveResponse{Derived: core.SessionDerivedPayloadFromResult(result)}
	if result.Child != nil {
		child := core.SessionPayloadFromInfo(result.Child)
		payload.Session = &child
	}
	return structuredResult(payload, result.ChildSessionID)
}

func nativeDeriveToolError(toolID toolspkg.ToolID, err error) error {
	toolErr, ok := errors.AsType[*toolspkg.ToolError](
		nativeHTTPStatusToolError(toolID, err, core.StatusForSessionError(err)),
	)
	if !ok {
		return err
	}
	payload := core.DeriveErrorPayload(err)
	// The calling agent reads the payload's user message, never the wrapped chain (which can
	// carry the child agent's stderr); the chain stays in the tool error's cause.
	if message := strings.TrimSpace(payload.Error); message != "" {
		cleaned := *toolErr
		cleaned.Message = message
		toolErr = &cleaned
	}
	structured, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return toolErr
	}
	preview := payload.Code
	if payload.ChildSessionID != "" {
		preview = "session " + payload.ChildSessionID +
			" was created before the failure; open it or retry the same idempotency_key"
	}
	return toolErr.WithPartialResult(toolspkg.ToolResult{Structured: structured, Preview: preview})
}

func nativeDeriveRuntime(payload *contract.PromptRuntimeSelectionPayload) *session.DeriveRuntime {
	selection := core.PromptRuntimeSelectionFromPayload(payload)
	if selection == nil {
		return nil
	}
	return &session.DeriveRuntime{
		Provider: selection.Provider, Model: selection.Model, ReasoningEffort: selection.ReasoningEffort,
		Speed: selection.Speed, ACPOptions: selection.ACPOptions,
	}
}

func nativeDeriveFencesComplete(epoch, generation, maxSequence *int64) error {
	set := 0
	for _, fence := range []*int64{epoch, generation, maxSequence} {
		if fence == nil {
			continue
		}
		if *fence < 0 {
			return errors.New("transcript fences must not be negative")
		}
		set++
	}
	if set != 0 && set != 3 {
		return errors.New("set all transcript fences together or omit all three")
	}
	return nil
}

func (n *daemonNativeTools) sessionContinueAvailability() toolspkg.NativeAvailabilityFunc {
	return n.dependencyAvailability(func() bool {
		if n == nil || n.deps == nil {
			return false
		}
		_, ok := n.deps.Sessions.(core.SessionDeriveManager)
		return ok
	})
}
