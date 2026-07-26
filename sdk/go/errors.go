package aghsdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
)

const (
	errorsErrorKey = "error"
)

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
	return e.Message
}

func (e *RPCError) object() JSONRPCErrorObject {
	if e == nil {
		return JSONRPCErrorObject{}
	}
	return JSONRPCErrorObject{
		Code:    e.Code,
		Message: e.Message,
		Data:    cloneRawMessage(e.Data),
	}
}

// NewRPCError creates a typed JSON-RPC error with optional data.
func NewRPCError(code int, message string, data any) *RPCError {
	var raw json.RawMessage
	if data != nil {
		encoded, err := json.Marshal(data)
		if err == nil && string(encoded) != "null" {
			raw = encoded
		}
	}
	return &RPCError{Code: code, Message: message, Data: raw}
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
	return &RPCError{
		Code:    obj.Code,
		Message: obj.Message,
		Data:    cloneRawMessage(obj.Data),
	}
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
