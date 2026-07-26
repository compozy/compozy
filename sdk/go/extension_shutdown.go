package aghsdk

import (
	"context"
	"encoding/json"
)

func (e *Extension) handleHealthCheck(
	ctx context.Context,
	request JSONRPCRequestEnvelope,
	params json.RawMessage,
) (HealthCheckResult, error) {
	e.mu.RLock()
	handler := e.handlers[healthCheckMethod]
	e.mu.RUnlock()
	if handler == nil {
		return HealthCheckResult{Healthy: true, Message: "", Details: map[string]json.RawMessage{}}, nil
	}
	result, err := handler(ctx, e.makeContext(request), params)
	if err != nil {
		return HealthCheckResult{}, err
	}
	typed, ok := result.(HealthCheckResult)
	if !ok {
		return HealthCheckResult{}, NewInternalError("health_check handler must return HealthCheckResult")
	}
	return typed, nil
}

func (e *Extension) handleShutdown(
	ctx context.Context,
	request JSONRPCRequestEnvelope,
	params json.RawMessage,
) (ShutdownResponse, error) {
	var shutdown ShutdownRequest
	if err := json.Unmarshal(params, &shutdown); err != nil {
		return ShutdownResponse{}, NewInvalidParamsError(
			"shutdown params must be an object",
			map[string]any{extensionErrorKey: err.Error()},
		)
	}
	if shutdown.DeadlineMS <= 0 {
		return ShutdownResponse{}, NewInvalidParamsError("deadline_ms must be greater than zero", nil)
	}
	e.mu.Lock()
	e.shutdownStarted = true
	e.shutdownDeadlineMS = shutdown.DeadlineMS
	handler := e.handlers[shutdownMethod]
	e.mu.Unlock()
	if handler == nil {
		return ShutdownResponse{Acknowledged: true}, nil
	}
	result, err := handler(ctx, e.makeContext(request), params)
	if err != nil {
		return ShutdownResponse{}, err
	}
	if result == nil {
		return ShutdownResponse{Acknowledged: true}, nil
	}
	typed, ok := result.(ShutdownResponse)
	if !ok {
		return ShutdownResponse{}, NewInternalError("shutdown handler must return ShutdownResponse")
	}
	return typed, nil
}
