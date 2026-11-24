package daemon

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type nativeComputerForwarder struct {
	computers core.ComputersService
	sessions  core.SessionManager
	access    workspaceaccess.Policy
}

var _ toolspkg.ComputerForwarder = (*nativeComputerForwarder)(nil)

func computerForwarderForBoot(state *bootState) toolspkg.ComputerForwarder {
	if state.deps.Computers == nil {
		return nil
	}
	return &nativeComputerForwarder{
		computers: state.deps.Computers,
		sessions:  state.sessions,
		access:    state.accessPolicy,
	}
}

func (f *nativeComputerForwarder) ResolveComputer(
	ctx context.Context, scope toolspkg.Scope, descriptor toolspkg.Descriptor, req toolspkg.CallRequest,
) (remoteproto.ComputerID, error) {
	if _, remote := remotehost.PrincipalFromContext(ctx); remote {
		return "", nativeComputerToolError(
			req.ToolID,
			"tool_not_forwardable",
			"This call is already on a remote computer.",
			http.StatusBadRequest,
			nil,
		)
	}
	if !f.sourceMayForward(ctx, scope, descriptor) {
		return "", nativeComputerToolError(
			req.ToolID,
			"tool_forward_denied",
			"This session can't use other computers.",
			http.StatusForbidden,
			nil,
		)
	}
	view, err := computers.ResolveReference(ctx, f.computers.List, req.ForwardComputer)
	return view.ID, asComputerToolError(req.ToolID, err)
}

func (f *nativeComputerForwarder) sourceMayForward(
	ctx context.Context,
	scope toolspkg.Scope,
	descriptor toolspkg.Descriptor,
) bool {
	if scope.SessionID == "" {
		return scope.Operator
	}
	if f.sessions == nil || f.access == nil {
		return false
	}
	info, err := f.sessions.Status(ctx, scope.SessionID)
	if err != nil || info == nil || info.State != session.StateActive || info.ID != scope.SessionID {
		return false
	}
	// A root session's generated workspace paths are its delegation budget for children.
	if lineage := info.Lineage; lineage != nil && len(lineage.PermissionPolicy.WorkspacePaths) > 0 &&
		(lineage.ParentSessionID != "" || info.InstanceID != "") {
		return false
	}
	decision, err := f.access.Authorize(ctx, workspaceaccess.Request{
		Actor: workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: info.ID,
			WorkspaceID: info.WorkspaceID, AgentName: info.AgentName},
		Target: workspaceaccess.TargetRef{
			Kind:        workspaceaccess.TargetAggregate,
			AllProfiles: true,
			Surface:     "remote_computers",
		},
		Seam:     workspaceaccess.SeamTool,
		ReadOnly: descriptor.ReadOnly,
	})
	return err == nil && decision.Allowed
}

func (f *nativeComputerForwarder) CallComputer(
	ctx context.Context, id remoteproto.ComputerID, req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	body, err := json.Marshal(
		contract.ToolInvokeRequest{Input: req.Input, SensitiveInputFields: req.SensitiveInputFields},
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	path := "/api/tools/" + url.PathEscape(req.ToolID.String()) + "/invoke"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := f.computers.RoundTrip(ctx, id, request)
	if err != nil {
		return toolspkg.ToolResult{}, asComputerToolError(req.ToolID, err)
	}
	defer core.DrainComputerResponse(response.Body)
	if response.StatusCode != http.StatusOK {
		return toolspkg.ToolResult{}, decodeForwardedToolError(req.ToolID, response.StatusCode, response.Body)
	}
	var result contract.ToolInvokeResponse
	if err := json.UnmarshalRead(response.Body, &result); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if result.ToolID != req.ToolID {
		return toolspkg.ToolResult{}, errors.New("remote tool response identifies a different tool")
	}
	return result.Result, nil
}

func decodeForwardedToolError(id toolspkg.ToolID, status int, body io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return err
	}
	var invokeError contract.ToolErrorResponse
	if err := json.Unmarshal(data, &invokeError); err == nil && invokeError.Error.Code != "" {
		payload := invokeError.Error
		failure := nativeComputerToolError(id, payload.Code, payload.Message, status, nil)
		failure.ReasonCodes, failure.Retryable = payload.ReasonCodes, payload.Retryable
		failure.PartialResult = payload.PartialResult
		failure.ClientMessageID, failure.ProviderError = payload.ClientMessageID, payload.ProviderError
		failure.RemoteDetails = payload.Details
		return failure
	}
	var remoteError contract.ErrorPayload
	if err := json.Unmarshal(data, &remoteError); err == nil && remoteError.Code != "" {
		failure := nativeComputerToolError(
			id,
			toolspkg.ErrorCode(remoteError.Code),
			remoteError.Error,
			status,
			&remotehost.Failure{
				Status: status, Code: remoteError.Code, Message: remoteError.Error, Details: remoteError.Details,
			},
		)
		if len(remoteError.Details) > 0 {
			failure.RemoteDetails = make(map[string]stdjson.RawMessage, len(remoteError.Details))
			for key, value := range remoteError.Details {
				encoded, _ := stdjson.Marshal(value)
				failure.RemoteDetails[key] = encoded
			}
		}
		return failure
	}
	return nativeComputerToolError(id, toolspkg.ErrorCodeBackendFailed, http.StatusText(status), status, nil)
}

func asComputerToolError(id toolspkg.ToolID, err error) error {
	if fault, ok := errors.AsType[*computers.Failure](err); ok {
		return nativeComputerToolError(id, toolspkg.ErrorCode(fault.Code), fault.Message, fault.Status, err)
	}
	if fault, ok := errors.AsType[*sshsetup.Failure](err); ok {
		return nativeComputerToolError(id, toolspkg.ErrorCode(fault.Code), fault.Message, fault.HTTPStatus, err)
	}
	if fault, ok := errors.AsType[*cloudclient.Failure](err); ok {
		return nativeComputerToolError(id, toolspkg.ErrorCode(fault.Code), fault.Message, fault.Status, err)
	}
	return err
}

func nativeComputerToolError(
	id toolspkg.ToolID,
	code toolspkg.ErrorCode,
	message string,
	status int,
	cause error,
) *toolspkg.ToolError {
	message = strings.TrimSpace(message)
	err := toolspkg.NewToolError(code, id, message, cause)
	err.PublicMessage, err.HTTPStatus = message, status
	return err
}
