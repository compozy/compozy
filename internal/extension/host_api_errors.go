package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"

	"strings"

	"github.com/compozy/agh/internal/acp"

	extensioncontract "github.com/compozy/agh/internal/extension/contract"

	"github.com/compozy/agh/internal/resources"

	"github.com/compozy/agh/internal/subprocess"
)

func decodeJSONValue(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
		return decoded
	}
	return trimmed
}

func invalidParamsRPCError(err error) error {
	if err == nil {
		return subprocess.NewRPCError(HostAPIInvalidParamsCode, "Invalid params", nil)
	}
	return subprocess.NewRPCError(
		HostAPIInvalidParamsCode,
		"Invalid params",
		map[string]string{extensionStateError: err.Error()},
	)
}

func unavailableRPCError(err error) error {
	if err == nil {
		return subprocess.NewRPCError(HostAPIUnavailableCode, "Unavailable", nil)
	}
	return subprocess.NewRPCError(
		HostAPIUnavailableCode,
		"Unavailable",
		map[string]string{extensionStateError: err.Error()},
	)
}

func notFoundRPCError(resource string, id string, err error) error {
	data := map[string]string{
		hostAPIResourceKey: strings.TrimSpace(resource),
		"id":               strings.TrimSpace(id),
	}
	if err != nil {
		data[extensionStateError] = err.Error()
	}
	return subprocess.NewRPCError(HostAPINotFoundCode, "Not found", data)
}

func methodNotFoundRPCError(method string) error {
	return subprocess.NewRPCError(
		HostAPIMethodNotFoundCode,
		"Method not found",
		map[string]string{hostAPIMethodKey: strings.TrimSpace(method)},
	)
}

func rpcCapabilityDenied(err error) error {
	var denied *ErrCapabilityDenied
	if !errors.As(err, &denied) {
		return err
	}
	if isResourceHostAPIMethod(denied.Data.Method) {
		return hostAPIStatusRPCError(403, "Forbidden", map[string]any{
			extensionStateError: denied.Error(),
			hostAPIMethodKey:    strings.TrimSpace(denied.Data.Method),
			"required":          append([]string(nil), denied.Data.Required...),
			"granted":           append([]string(nil), denied.Data.Granted...),
		})
	}
	return subprocess.NewRPCError(denied.Code(), "Capability denied", denied.Data)
}

func normalizeHostAPIRPCError(method string, err error) error {
	if err == nil {
		return nil
	}
	if !isResourceHostAPIMethod(method) {
		return err
	}

	if rpcErr, ok := errors.AsType[*subprocess.RPCError](err); ok {
		if rpcErr.Code == HostAPIRateLimitedCode {
			return hostAPIStatusRPCError(429, "Rate limited", rpcErr.Data)
		}
		return err
	}

	switch {
	case errors.Is(err, resources.ErrPermissionDenied), errors.Is(err, resources.ErrDirectMutationNotAllowed):
		return hostAPIStatusRPCError(403, "Forbidden", map[string]any{extensionStateError: err.Error()})
	case errors.Is(err, resources.ErrConflict), errors.Is(err, resources.ErrSessionNotActive),
		errors.Is(err, resources.ErrStaleSourceVersion):
		return hostAPIStatusRPCError(409, "Conflict", map[string]any{extensionStateError: err.Error()})
	case errors.Is(err, resources.ErrPayloadTooLarge):
		return hostAPIStatusRPCError(413, "Payload too large", map[string]any{extensionStateError: err.Error()})
	case errors.Is(err, resources.ErrNotFound):
		return notFoundRPCError(hostAPIResourceKey, "", err)
	case errors.Is(err, resources.ErrValidation), errors.Is(err, resources.ErrInvalidScopeBinding):
		return invalidParamsRPCError(err)
	default:
		return err
	}
}

func hostAPIStatusRPCError(code int, message string, data any) error {
	return subprocess.NewRPCError(code, strings.TrimSpace(message), data)
}

func isResourceHostAPIMethod(method string) bool {
	switch strings.TrimSpace(method) {
	case string(extensioncontract.HostAPIMethodResourcesList),
		string(extensioncontract.HostAPIMethodResourcesGet),
		string(extensioncontract.HostAPIMethodResourcesSnapshot):
		return true
	default:
		return false
	}
}

func withHostAPIExtensionName(ctx context.Context, extName string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, hostAPIExtensionNameContextKey, strings.TrimSpace(extName))
}

func hostAPIExtensionNameFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, ok := ctx.Value(hostAPIExtensionNameContextKey).(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func drainAgentEvents(events <-chan acp.AgentEvent) {
	for range events {
		continue
	}
}
