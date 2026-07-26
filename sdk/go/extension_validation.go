package aghsdk

import (
	"encoding/json"
	"errors"

	"strings"
)

func validateInitializeRequest(request InitializeRequest) error {
	if strings.TrimSpace(request.ProtocolVersion) == "" {
		return NewInvalidParamsError("protocol_version is required", nil)
	}
	if strings.TrimSpace(request.SessionNonce) == "" {
		return NewInvalidParamsError("session_nonce is required", nil)
	}
	if len(request.SupportedProtocolVersions) == 0 {
		return NewInvalidParamsError("supported_protocol_versions is required", nil)
	}
	if strings.TrimSpace(request.Extension.Name) == "" || strings.TrimSpace(request.Extension.Version) == "" {
		return NewInvalidParamsError("extension identity is required", nil)
	}
	if request.Runtime.HealthCheckIntervalMS <= 0 {
		return NewInvalidParamsError("health_check_interval_ms must be greater than zero", nil)
	}
	if request.Runtime.HealthCheckTimeoutMS <= 0 {
		return NewInvalidParamsError("health_check_timeout_ms must be greater than zero", nil)
	}
	if request.Runtime.ShutdownTimeoutMS <= 0 {
		return NewInvalidParamsError("shutdown_timeout_ms must be greater than zero", nil)
	}
	if request.Runtime.DefaultHookTimeoutMS <= 0 {
		return NewInvalidParamsError("default_hook_timeout_ms must be greater than zero", nil)
	}
	return nil
}

func normalizeHostMethods(values []HostAPIMethod) []HostAPIMethod {
	normalized := normalizeStrings(hostMethodsToStrings(values))
	out := make([]HostAPIMethod, 0, len(normalized))
	for _, value := range normalized {
		out = append(out, HostAPIMethod(value))
	}
	return out
}

func hostMethodsToStrings(values []HostAPIMethod) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return out
}

func canonicalExtensionToolID(extensionName string, handler string) (ToolID, error) {
	extensionSegment, err := canonicalIDSegment(extensionName)
	if err != nil {
		return "", err
	}
	handlerSegment, err := canonicalIDSegment(handler)
	if err != nil {
		return "", err
	}
	id := ToolID("ext__" + extensionSegment + "__" + handlerSegment)
	if err := id.Validate(); err != nil {
		return "", err
	}
	return id, nil
}

func canonicalIDSegment(raw string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	var builder strings.Builder
	lastUnderscore := false
	for _, char := range normalized {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
			lastUnderscore = false
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastUnderscore = false
		default:
			if builder.Len() > 0 && !lastUnderscore {
				builder.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	segment := strings.Trim(builder.String(), "_")
	if segment == "" || strings.Contains(segment, "__") || !regexpSegment(segment) {
		return "", NewInvalidParamsError("invalid tool id segment", map[string]any{"segment": raw})
	}
	return segment, nil
}

func regexpSegment(segment string) bool {
	return segmentedIDPattern.MatchString(segment)
}

func isToolProviderMethod(method string) bool {
	return method == ExtensionServiceMethodProvideTools || method == ExtensionServiceMethodToolsCall
}

func ensureRPCErrorIfTyped(err error) *RPCError {
	if rpcErr, ok := errors.AsType[*RPCError](err); ok {
		return rpcErr
	}
	return nil
}

func toolExecutionError(err error, call ExtensionToolCallRequest, sensitiveFields []string) *RPCError {
	message := redactSensitiveText(err.Error(), call.Input, sensitiveFields)
	data := map[string]any{
		extensionToolIDKey:  string(call.ToolID),
		extensionHandlerKey: call.Handler,
		extensionErrorKey:   message,
	}
	if len(sensitiveFields) > 0 {
		data["input"] = redactSensitiveInput(call.Input, sensitiveFields)
		data["sensitive_input_fields"] = sensitiveFields
	}
	return NewToolExecutionError(data)
}

func redactSensitiveText(text string, input json.RawMessage, sensitiveFields []string) string {
	redacted := text
	for _, field := range sensitiveFields {
		value := readJSONPath(input, field)
		if value != "" {
			redacted = strings.ReplaceAll(redacted, value, "[REDACTED]")
		}
	}
	return redacted
}

func redactSensitiveInput(input json.RawMessage, sensitiveFields []string) any {
	if len(input) == 0 {
		return map[string]any{}
	}
	var value any
	if err := json.Unmarshal(input, &value); err != nil {
		return map[string]any{"redacted": true}
	}
	for _, field := range sensitiveFields {
		redactPath(value, strings.Split(field, "."))
	}
	return value
}

func readJSONPath(input json.RawMessage, field string) string {
	var value any
	if err := json.Unmarshal(input, &value); err != nil {
		return ""
	}
	for part := range strings.SplitSeq(field, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		object, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		value = object[part]
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func redactPath(value any, path []string) {
	if len(path) == 0 {
		return
	}
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	head := strings.TrimSpace(path[0])
	if head == "" {
		redactPath(value, path[1:])
		return
	}
	if len(path) == 1 {
		if _, ok := object[head]; ok {
			object[head] = "[REDACTED]"
		}
		return
	}
	redactPath(object[head], path[1:])
}
