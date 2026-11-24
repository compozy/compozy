package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

const computerArgument = "computer"

type computerInputProjection struct {
	schema json.RawMessage
	digest string
}

type ComputerForwarder interface {
	ResolveComputer(context.Context, Scope, Descriptor, CallRequest) (remoteproto.ComputerID, error)
	CallComputer(context.Context, remoteproto.ComputerID, CallRequest) (ToolResult, error)
}

func WithComputerForwarder(forwarder ComputerForwarder) RegistryOption {
	return func(r *RuntimeRegistry) { r.computerForwarder = forwarder }
}

type noComputerForwardingKey struct{}

func WithoutComputerForwarding(ctx context.Context) context.Context {
	return context.WithValue(ctx, noComputerForwardingKey{}, true)
}

func computerForwardingDisabled(ctx context.Context) bool {
	return ctx.Value(noComputerForwardingKey{}) != nil
}

func prepareComputerInputSchema(descriptor *Descriptor) error {
	metadata := descriptor.ExecutionMetadata()
	if descriptor.Source.Kind != SourceBuiltin || metadata.RemoteScope == "" {
		return nil
	}
	var schema map[string]jsontext.Value
	if err := jsonv2.Unmarshal(descriptor.InputSchema, &schema); err != nil {
		return err
	}
	var properties map[string]jsontext.Value
	if raw, ok := schema["properties"]; ok {
		if err := jsonv2.Unmarshal(raw, &properties); err != nil {
			return err
		}
	}
	if properties == nil {
		properties = make(map[string]jsontext.Value)
	}
	properties[computerArgument] = jsontext.Value(`{"type":"string","minLength":1,"description":"Computer name or ID"}`)
	encoded, err := jsonv2.Marshal(properties, jsonv2.Deterministic(true))
	if err != nil {
		return err
	}
	schema["properties"] = encoded
	projected, err := jsonv2.Marshal(schema, jsonv2.Deterministic(true))
	if err != nil {
		return err
	}
	digest, err := SchemaDigest(projected)
	if err != nil {
		return err
	}
	metadata.computerInput = &computerInputProjection{schema: projected, digest: digest}
	descriptor.ToolExecutionMetadata = &metadata
	return nil
}

func splitComputerInput(req CallRequest) (CallRequest, error) {
	req.ForwardComputer = ""
	if !strings.HasPrefix(req.ToolID.String(), "compozy__") {
		return req, nil
	}
	input := bytes.TrimSpace(req.Input)
	if len(input) == 0 || input[0] != '{' {
		return req, nil
	}
	var args map[string]jsontext.Value
	if err := jsonv2.Unmarshal(input, &args); err != nil {
		return req, invalidInputError(req.ToolID, "expected an unambiguous JSON object", err)
	}
	if req.ToolID == ToolIDComputerSetupAnswer && hasSSHSecretAnswer(args) {
		return req, computerCallError(
			req.ToolID,
			ErrorCodeSecretAnswerOperatorOnly,
			"Answer SSH secrets in the app or CLI.",
			http.StatusForbidden,
		)
	}
	raw, ok := args[computerArgument]
	if !ok {
		return req, nil
	}
	if err := jsonv2.Unmarshal(raw, &req.ForwardComputer); err != nil || strings.TrimSpace(req.ForwardComputer) == "" {
		return req, invalidInputError(req.ToolID, "computer must be a name or ID", err)
	}
	req.ForwardComputer = strings.TrimSpace(req.ForwardComputer)
	delete(args, computerArgument)
	stripped, err := jsonv2.Marshal(args, jsonv2.Deterministic(true))
	req.Input = stripped
	return req, err
}

func hasSSHSecretAnswer(args map[string]jsontext.Value) bool {
	if _, found := args["value"]; found {
		return true
	}
	var kind string
	return jsonv2.Unmarshal(args["kind"], &kind) == nil && kind == "secret"
}

func computerCallError(id ToolID, code ErrorCode, message string, status int) *ToolError {
	err := NewToolError(code, id, message, nil)
	err.PublicMessage, err.HTTPStatus = message, status
	return err
}

func (r *RuntimeRegistry) prepareComputerDispatch(
	ctx context.Context,
	scope Scope,
	target *dispatchTarget,
	req *CallRequest,
	started time.Time,
) error {
	if req.ForwardComputer == "" {
		return nil
	}
	if err := checkComputerForwardable(ctx, target.descriptor, *req); err != nil {
		return r.failDispatch(ctx, target, *req, started, err, ToolCallDenied)
	}
	if err := r.ensureDispatchTargetCallable(ctx, target, *req, started); err != nil {
		return err
	}
	if err := r.resolveComputerCall(ctx, scope, target.descriptor, req); err != nil {
		return r.failDispatch(ctx, target, *req, started, err, ToolCallDenied)
	}
	return nil
}

func checkComputerForwardable(ctx context.Context, desc Descriptor, req CallRequest) error {
	if computerForwardingDisabled(ctx) || desc.Source.Kind != SourceBuiltin ||
		desc.ExecutionMetadata().RemoteScope == "" {
		return computerCallError(
			req.ToolID,
			ErrorCodeNotForwardable,
			fmt.Sprintf("%s can't run on another computer.", req.ToolID),
			http.StatusBadRequest,
		)
	}
	return nil
}

func (r *RuntimeRegistry) resolveComputerCall(
	ctx context.Context,
	scope Scope,
	desc Descriptor,
	req *CallRequest,
) error {
	if req.ForwardComputer == "local" {
		req.ForwardComputer = ""
		return nil
	}
	if r.computerForwarder == nil {
		return computerCallError(
			req.ToolID,
			ErrorCodeUnavailable,
			"Computer forwarding is unavailable.",
			http.StatusServiceUnavailable,
		)
	}
	target, err := r.computerForwarder.ResolveComputer(ctx, scope, desc, *req)
	if err == nil {
		req.ForwardComputer = string(target)
	}
	return err
}

// BindComputerApprovalInput binds approval to the same destination used by dispatch.
func (r *RuntimeRegistry) BindComputerApprovalInput(
	ctx context.Context, scope Scope, desc Descriptor, input json.RawMessage,
) (json.RawMessage, bool, error) {
	req, err := splitComputerInput(CallRequest{ToolID: desc.ID, Input: input})
	if err != nil || req.ForwardComputer == "" {
		return input, false, err
	}
	if err := checkComputerForwardable(ctx, desc, req); err != nil {
		return nil, false, err
	}
	if err := r.resolveComputerCall(ctx, scope, desc, &req); err != nil {
		return nil, false, err
	}
	if req.ForwardComputer == "" {
		return req.Input, false, nil
	}
	bound, err := InputWithComputer(req.Input, req.ForwardComputer)
	return bound, true, err
}

func (r *RuntimeRegistry) authorizeDispatchInput(
	ctx context.Context,
	scope Scope,
	desc Descriptor,
	req CallRequest,
) error {
	if req.ForwardComputer == "" {
		return r.authorizeBoundCallInput(ctx, scope, desc, req)
	}
	inner, err := splitComputerInput(req)
	if err != nil {
		return err
	}
	if inner.ForwardComputer != "" {
		return computerCallError(
			req.ToolID,
			ErrorCodeDenied,
			"A pre-call hook cannot change the destination computer.",
			http.StatusForbidden,
		)
	}
	return nil
}

func (r *RuntimeRegistry) invokeComputerCall(
	ctx context.Context, scope Scope, target *dispatchTarget, req CallRequest, started time.Time,
) (ToolResult, error) {
	id := remoteproto.ComputerID(req.ForwardComputer)
	result, err := r.computerForwarder.CallComputer(ctx, id, req)
	if contextError := contextErr(ctx, req.ToolID); contextError != nil {
		err = contextError
	}
	if target.audit != nil {
		target.audit.semanticError = bytes.Equal(result.Metadata["is_error"], []byte("true"))
	}
	if err != nil {
		return ToolResult{}, r.failDispatch(
			ctx,
			target,
			req,
			started,
			r.runPostErrorHook(ctx, scope, target, req, err),
			ToolCallFailed,
		)
	}
	result.ComputerID = id
	if err := r.emit(
		ctx,
		target,
		req,
		ToolCallCompleted,
		ToolEventData{StartedAt: started, Result: result},
	); err != nil {
		return ToolResult{}, err
	}
	return result, nil
}

func InputWithComputer(input json.RawMessage, computer string) (json.RawMessage, error) {
	var args map[string]jsontext.Value
	if err := jsonv2.Unmarshal(input, &args); err != nil {
		return nil, fmt.Errorf("computer tool input must be an object: %w", err)
	}
	if args == nil {
		return nil, fmt.Errorf("computer tool input must be an object")
	}
	encoded, err := jsonv2.Marshal(computer)
	if err != nil {
		return nil, err
	}
	args[computerArgument] = encoded
	return jsonv2.Marshal(args, jsonv2.Deterministic(true))
}
