package extensionpkg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/session"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/google/uuid"
)

const hostAPIClarifyAskPath = "clarify/ask"

type hostAPIClarifyRuntime struct {
	broker      toolspkg.ClarifyBroker
	mu          sync.Mutex
	invocations map[string]hostAPIClarifyInvocation
	newID       func() string
}

type hostAPIClarifyInvocation struct {
	extensionName string
	sessionID     string
	callContext   context.Context
}

// WithHostAPIClarify injects the shared broker and active-call correlation runtime.
func WithHostAPIClarify(broker toolspkg.ClarifyBroker) HostAPIOption {
	return func(handler *HostAPIHandler) {
		if broker == nil {
			return
		}
		handler.clarify = &hostAPIClarifyRuntime{
			broker:      broker,
			invocations: make(map[string]hostAPIClarifyInvocation),
			newID:       uuid.NewString,
		}
	}
}

// BeginExtensionToolCall binds one opaque invocation ID to the active extension session.
func (h *HostAPIHandler) BeginExtensionToolCall(
	ctx context.Context,
	extensionName string,
	sessionID string,
) (string, func(), error) {
	if h == nil || h.clarify == nil {
		return "", func() {}, nil
	}
	if ctx == nil {
		return "", nil, errors.New("extension: tool call context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	extensionName = strings.TrimSpace(extensionName)
	sessionID = strings.TrimSpace(sessionID)
	if extensionName == "" || sessionID == "" {
		return "", func() {}, nil
	}

	runtime := h.clarify
	invocationID := strings.TrimSpace(runtime.newID())
	if invocationID == "" {
		return "", nil, errors.New("extension: clarification invocation ID is required")
	}
	callContext, cancelCall := context.WithCancel(ctx)
	runtime.mu.Lock()
	if _, exists := runtime.invocations[invocationID]; exists {
		runtime.mu.Unlock()
		cancelCall()
		return "", nil, fmt.Errorf("extension: duplicate clarification invocation ID %q", invocationID)
	}
	runtime.invocations[invocationID] = hostAPIClarifyInvocation{
		extensionName: extensionName,
		sessionID:     sessionID,
		callContext:   callContext,
	}
	runtime.mu.Unlock()

	var once sync.Once
	return invocationID, func() {
		once.Do(func() {
			runtime.mu.Lock()
			delete(runtime.invocations, invocationID)
			runtime.mu.Unlock()
			cancelCall()
		})
	}, nil
}

func registerHostAPIClarifyMethodHandler(
	handler *HostAPIHandler,
	handlers map[string]hostAPIMethodFunc,
) {
	handlers[hostAPIClarifyAskPath] = handler.handleClarifyAsk
}

func (h *HostAPIHandler) handleClarifyAsk(
	ctx context.Context,
	raw json.RawMessage,
) (any, error) {
	if h.clarify == nil || h.clarify.broker == nil {
		return nil, unavailableRPCError(errors.New("extension: clarification broker is not configured"))
	}
	if h.sessions == nil {
		return nil, unavailableRPCError(errors.New("extension: session manager is not configured"))
	}

	var params extensioncontract.ClarifyAskParams
	if err := decodeStrictClarifyParams(raw, &params); err != nil {
		return nil, err
	}
	invocation, ok := h.clarifyInvocation(ctx, params.InvocationID)
	if !ok {
		return nil, notFoundRPCError(
			"tool invocation",
			params.InvocationID,
			errors.New("extension: active tool invocation not found"),
		)
	}
	info, err := h.sessions.Status(ctx, invocation.sessionID)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return nil, notFoundRPCError("session", invocation.sessionID, err)
		}
		return nil, unavailableRPCError(err)
	}
	if info == nil || info.State != session.StateActive {
		return nil, unavailableRPCError(fmt.Errorf(
			"extension: session %q is not active",
			invocation.sessionID,
		))
	}
	question, err := (toolspkg.ClarifyQuestion{
		Question: params.Question,
		Choices:  params.Choices,
	}).Normalize()
	if err != nil {
		return nil, invalidParamsRPCError(err)
	}
	askCtx, cancelAsk := context.WithCancel(ctx)
	stopCallCancellation := context.AfterFunc(invocation.callContext, cancelAsk)
	defer func() {
		stopCallCancellation()
		cancelAsk()
	}()
	answer, err := h.clarify.broker.Ask(askCtx, toolspkg.Scope{
		WorkspaceID: strings.TrimSpace(info.WorkspaceID),
		SessionID:   strings.TrimSpace(info.ID),
		AgentName:   strings.TrimSpace(info.AgentName),
	}, question)
	if err != nil {
		return nil, clarifyHostAPIRPCError(err)
	}
	return answer, nil
}

func (h *HostAPIHandler) clarifyInvocation(
	ctx context.Context,
	invocationID string,
) (hostAPIClarifyInvocation, bool) {
	if h == nil || h.clarify == nil || ctx == nil {
		return hostAPIClarifyInvocation{}, false
	}
	invocationID = strings.TrimSpace(invocationID)
	if invocationID == "" {
		return hostAPIClarifyInvocation{}, false
	}
	extensionName := hostAPIExtensionNameFromContext(ctx)
	h.clarify.mu.Lock()
	invocation, ok := h.clarify.invocations[invocationID]
	h.clarify.mu.Unlock()
	if !ok || invocation.extensionName != extensionName {
		return hostAPIClarifyInvocation{}, false
	}
	return invocation, true
}

func decodeStrictClarifyParams(raw json.RawMessage, target *extensioncontract.ClarifyAskParams) error {
	if target == nil {
		return invalidParamsRPCError(errors.New("clarification params target is required"))
	}
	payload := bytes.TrimSpace(raw)
	if len(payload) == 0 || bytes.Equal(payload, []byte("null")) {
		payload = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return invalidParamsRPCError(fmt.Errorf("decode params: %w", err))
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return invalidParamsRPCError(fmt.Errorf("decode params: %w", err))
	}
	if strings.TrimSpace(target.InvocationID) == "" {
		return invalidParamsRPCError(errors.New("invocation_id is required"))
	}
	return nil
}

func clarifyHostAPIRPCError(err error) error {
	switch {
	case errors.Is(err, toolspkg.ErrClarifyInvalid):
		return invalidParamsRPCError(err)
	case errors.Is(err, toolspkg.ErrClarifyPending):
		return hostAPIStatusRPCError(409, "Conflict", map[string]string{extensionStateError: err.Error()})
	case errors.Is(err, toolspkg.ErrClarifyNotFound):
		return notFoundRPCError("clarification", "", err)
	case errors.Is(err, toolspkg.ErrClarifyCanceled), errors.Is(err, toolspkg.ErrClarifyClosed):
		return unavailableRPCError(err)
	default:
		return unavailableRPCError(err)
	}
}
