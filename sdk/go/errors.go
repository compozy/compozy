package compozysdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"strings"
)

const (
	errorsErrorKey            = "error"
	claimTokenRedactionMarker = "[REDACTED]"
)

var claimTokenPattern = regexp.MustCompile(`(?i)compozy_claim_[A-Za-z0-9_-]+`)

// JSONRPCErrorObject is the wire error object used by JSON-RPC 2.0.
type JSONRPCErrorObject struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// RPCError is a typed JSON-RPC error.
type RPCError struct {
	Code    int
	Message string
	Data    json.RawMessage
}

// Error returns the stable error message.
func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	return redactClaimTokens(e.Message)
}

func (e *RPCError) object() JSONRPCErrorObject {
	if e == nil {
		return JSONRPCErrorObject{}
	}
	return JSONRPCErrorObject{
		Code:    e.Code,
		Message: redactClaimTokens(e.Message),
		Data:    redactClaimTokensJSON(e.Data),
	}
}

// NewRPCError creates a typed JSON-RPC error with optional data.
func NewRPCError(code int, message string, data any) *RPCError {
	var raw json.RawMessage
	if data != nil {
		encoded, err := json.Marshal(data)
		if err == nil && string(encoded) != "null" {
			raw = redactClaimTokensJSON(encoded)
		}
	}
	return &RPCError{Code: code, Message: redactClaimTokens(message), Data: raw}
}

// NewInvalidRequestError creates an Invalid request error.
func NewInvalidRequestError(reason string) *RPCError {
	return NewRPCError(-32600, "Invalid request", map[string]any{"reason": reason})
}

// NewMethodNotFoundError creates a Method not found error.
func NewMethodNotFoundError(method string) *RPCError {
	return NewRPCError(-32601, "Method not found", map[string]any{"method": method})
}

// NewInvalidParamsError creates an Invalid params error.
func NewInvalidParamsError(reason string, data map[string]any) *RPCError {
	payload := map[string]any{errorsErrorKey: reason}
	maps.Copy(payload, data)
	return NewRPCError(-32602, "Invalid params", payload)
}

// NewInternalError creates an Internal error.
func NewInternalError(reason string) *RPCError {
	return NewRPCError(-32603, "Internal error", map[string]any{errorsErrorKey: reason})
}

// NewCapabilityDeniedError creates a capability denied error.
func NewCapabilityDeniedError(data map[string]any) *RPCError {
	return NewRPCError(-32001, "Capability denied", data)
}

// NewNotInitializedError creates a not-initialized error.
func NewNotInitializedError() *RPCError {
	return NewRPCError(-32003, "Not initialized", map[string]any{"allowed_methods": []string{initializeMethod}})
}

// NewShutdownInProgressError creates a shutdown-in-progress error.
func NewShutdownInProgressError(deadlineMS int64) *RPCError {
	data := map[string]any{}
	if deadlineMS > 0 {
		data["deadline_ms"] = deadlineMS
	}
	return NewRPCError(-32004, "Shutdown in progress", data)
}

// NewToolExecutionError creates a tool execution error.
func NewToolExecutionError(data map[string]any) *RPCError {
	return NewRPCError(-32010, "Tool execution failed", data)
}

func rpcErrorFromObject(obj JSONRPCErrorObject) *RPCError {
	return NewRPCError(obj.Code, obj.Message, obj.Data)
}

func ensureRPCError(err error) *RPCError {
	if rpcErr, ok := errors.AsType[*RPCError](err); ok {
		return rpcErr
	}
	if err == nil {
		return nil
	}
	return NewInternalError(err.Error())
}

func wrapTransportError(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

func redactClaimTokens(value string) string {
	return claimTokenPattern.ReplaceAllString(value, claimTokenRedactionMarker)
}

func redactClaimTokensJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}

	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return redactClaimTokensJSONFallback(raw)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return redactClaimTokensJSONFallback(raw)
	}

	redacted, changed := redactClaimTokensJSONValue(decoded)
	if !changed {
		return cloneRawMessage(raw)
	}
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return redactClaimTokensJSONFallback(raw)
	}
	return encoded
}

func redactClaimTokensJSONFallback(raw json.RawMessage) json.RawMessage {
	encoded, err := json.Marshal(redactClaimTokens(string(raw)))
	if err != nil {
		return json.RawMessage(`""`)
	}
	return encoded
}

func redactClaimTokensJSONValue(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		changed := false
		redacted := make(map[string]any, len(typed))
		for key, nested := range typed {
			if strings.EqualFold(strings.TrimSpace(key), "claim_token") || redactClaimTokens(key) != key {
				changed = true
				continue
			}
			next, nestedChanged := redactClaimTokensJSONValue(nested)
			redacted[key] = next
			changed = changed || nestedChanged
		}
		return redacted, changed
	case []any:
		changed := false
		redacted := make([]any, len(typed))
		for index, nested := range typed {
			next, nestedChanged := redactClaimTokensJSONValue(nested)
			redacted[index] = next
			changed = changed || nestedChanged
		}
		return redacted, changed
	case string:
		redacted := redactClaimTokens(typed)
		return redacted, redacted != typed
	default:
		return value, false
	}
}
