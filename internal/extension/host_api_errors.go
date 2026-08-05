package extensionpkg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"strings"

	"github.com/compozy/compozy/internal/acp"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"

	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/resources"

	"github.com/compozy/compozy/internal/subprocess"
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
	denied, ok := errors.AsType[*ErrCapabilityDenied](err)
	if !ok {
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

	normalized := err
	if isResourceHostAPIMethod(method) {
		if rpcErr, ok := errors.AsType[*subprocess.RPCError](err); ok {
			if rpcErr.Code == HostAPIRateLimitedCode {
				normalized = hostAPIStatusRPCError(429, "Rate limited", rpcErr.Data)
			}
		} else {
			switch {
			case errors.Is(err, resources.ErrPermissionDenied), errors.Is(err, resources.ErrDirectMutationNotAllowed):
				normalized = hostAPIStatusRPCError(403, "Forbidden", map[string]any{extensionStateError: err.Error()})
			case errors.Is(err, resources.ErrConflict), errors.Is(err, resources.ErrSessionNotActive),
				errors.Is(err, resources.ErrStaleSourceVersion):
				normalized = hostAPIStatusRPCError(409, "Conflict", map[string]any{extensionStateError: err.Error()})
			case errors.Is(err, resources.ErrPayloadTooLarge):
				normalized = hostAPIStatusRPCError(
					413,
					"Payload too large",
					map[string]any{extensionStateError: err.Error()},
				)
			case errors.Is(err, resources.ErrNotFound):
				normalized = notFoundRPCError(hostAPIResourceKey, "", err)
			case errors.Is(err, resources.ErrValidation), errors.Is(err, resources.ErrInvalidScopeBinding):
				normalized = invalidParamsRPCError(err)
			}
		}
	}
	return sanitizeHostAPIError(normalized)
}

func sanitizeHostAPIError(err error) error {
	if err == nil {
		return nil
	}
	if rpcErr, ok := errors.AsType[*subprocess.RPCError](err); ok {
		sanitized := sanitizeHostAPIRPCError(rpcErr)
		if sanitized == rpcErr && !hostAPIErrorContainsRawClaimToken(err, 0) {
			return err
		}
		return sanitized
	}
	if !hostAPIErrorContainsRawClaimToken(err, 0) {
		return err
	}
	return errors.New(redact.ClaimTokens(err.Error()))
}

func sanitizeHostAPIRPCError(err *subprocess.RPCError) *subprocess.RPCError {
	if err == nil {
		return nil
	}
	message := redact.ClaimTokens(err.Message)
	data := redact.ClaimTokensJSON(err.Data)
	if message == err.Message && bytes.Equal(data, err.Data) {
		return err
	}
	return &subprocess.RPCError{Code: err.Code, Message: message, Data: data}
}

func hostAPIErrorContainsRawClaimToken(err error, depth int) bool {
	if err == nil || depth >= 64 {
		return false
	}
	if redact.ClaimTokens(err.Error()) != err.Error() {
		return true
	}
	if rpcErr, ok := errors.AsType[*subprocess.RPCError](err); ok {
		if sanitizeHostAPIRPCError(rpcErr) != rpcErr {
			return true
		}
	}
	if multiple, ok := err.(interface{ Unwrap() []error }); ok {
		for _, nested := range multiple.Unwrap() {
			if hostAPIErrorContainsRawClaimToken(nested, depth+1) {
				return true
			}
		}
		return false
	}
	if single, ok := err.(interface{ Unwrap() error }); ok {
		return hostAPIErrorContainsRawClaimToken(single.Unwrap(), depth+1)
	}
	return false
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

func withHostAPICapabilityGrantID(ctx context.Context, grantID string) context.Context {
	return context.WithValue(ctx, hostAPICapabilityGrantIDContextKey, strings.TrimSpace(grantID))
}

func hostAPICapabilityGrantIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, ok := ctx.Value(hostAPICapabilityGrantIDContextKey).(string)
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
