package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	gatewayActionStatus          = "status"
	gatewayActionAudit           = "audit"
	gatewayActionDeviceList      = "device_list"
	gatewayActionSurfaceSet      = "surface_set"
	gatewayActionProviderEnable  = "provider_enable"
	gatewayActionProviderDisable = "provider_disable"
	gatewayActionDeviceRename    = "device_rename"
	gatewayActionDeviceRevoke    = "device_revoke"
	gatewayActionIngressBind     = "ingress_bind"
	gatewayActionIngressUnbind   = "ingress_unbind"
)

type nativeGatewayInput struct {
	Action             string `json:"action"`
	Surface            string `json:"surface"`
	Tier               string `json:"tier"`
	Desired            string `json:"desired"`
	ExpectedGeneration uint64 `json:"expected_generation"`
	Consent            bool   `json:"consent"`
	Provider           string `json:"provider"`
	InstallSource      string `json:"install_source"`
	DigestConfirmed    string `json:"digest_confirmed"`
	DeviceID           string `json:"device_id"`
	Name               string `json:"name"`
	SubjectKind        string `json:"subject_kind"`
	SubjectID          string `json:"subject_id"`
	Confirmed          bool   `json:"confirmed"`
}

type nativeGatewayResult struct {
	Action   string                                  `json:"action"`
	Redacted bool                                    `json:"redacted"`
	Status   *contract.GatewayStatusPayload          `json:"status,omitempty"`
	Audit    *contract.GatewayAuditPayload           `json:"audit,omitempty"`
	Devices  *[]contract.GatewayDevicePayload        `json:"devices,omitempty"`
	Device   *contract.GatewayDevicePayload          `json:"device,omitempty"`
	Revoke   *contract.GatewayRevokePayload          `json:"revoke,omitempty"`
	Binding  *contract.GatewayIngressBindingResponse `json:"binding,omitempty"`
	Unbind   *contract.GatewayIngressUnbindResponse  `json:"unbind,omitempty"`
}

type nativeGatewayActionRequest struct {
	scope  toolspkg.Scope
	toolID toolspkg.ToolID
	action string
	input  nativeGatewayInput
}

func (n *daemonNativeTools) gatewayToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDGateway: {
			call:         n.gatewayCall,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) gatewayCall(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeGatewayInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	action, err := requiredNativeString(req.ToolID, "action", input.Action)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if n == nil || n.deps == nil || n.deps.gatewayService() == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "gateway service is unavailable")
	}
	if !isGatewayReadAction(action) {
		if err := n.authorizeGatewayMutation(ctx, scope, req.ToolID); err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	workspaceID := strings.TrimSpace(firstNonEmpty(scope.WorkspaceID, req.WorkspaceID))
	if workspaceID != "" {
		ctx = gateway.WithIngressReadWorkspace(ctx, workspaceID)
	}

	result, err := n.executeGatewayAction(ctx, nativeGatewayActionRequest{
		scope:  scope,
		toolID: req.ToolID,
		action: action,
		input:  input,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(result, fmt.Sprintf("gateway %s", action))
}

func (n *daemonNativeTools) executeGatewayAction(
	ctx context.Context,
	request nativeGatewayActionRequest,
) (nativeGatewayResult, error) {
	switch request.action {
	case gatewayActionStatus, gatewayActionAudit, gatewayActionDeviceList:
		return n.executeGatewayReadAction(ctx, request)
	case gatewayActionIngressBind, gatewayActionIngressUnbind:
		return n.executeGatewayIngressAction(ctx, request)
	default:
		return n.executeGatewayMutationAction(ctx, request)
	}
}

func (n *daemonNativeTools) executeGatewayReadAction(
	ctx context.Context,
	request nativeGatewayActionRequest,
) (nativeGatewayResult, error) {
	result := nativeGatewayResult{Action: request.action, Redacted: true}
	switch request.action {
	case gatewayActionStatus:
		status, err := n.deps.gatewayService().Status(ctx)
		if err != nil {
			return nativeGatewayResult{}, err
		}
		payload := core.GatewayStatusPayload(status)
		result.Status = &payload
	case gatewayActionAudit:
		report, err := n.deps.gatewayService().Audit(ctx)
		if err != nil {
			return nativeGatewayResult{}, err
		}
		payload := core.GatewayAuditPayload(report)
		result.Audit = &payload
	case gatewayActionDeviceList:
		devices, err := n.deps.gatewayService().ListDevices(ctx)
		if err != nil {
			return nativeGatewayResult{}, err
		}
		payload := core.GatewayDevicePayloads(devices)
		result.Devices = &payload
	default:
		return nativeGatewayResult{}, nativeGatewayInputError(request.toolID, "action", "unsupported gateway action")
	}
	return result, nil
}

func (n *daemonNativeTools) executeGatewayMutationAction(
	ctx context.Context,
	request nativeGatewayActionRequest,
) (nativeGatewayResult, error) {
	result := nativeGatewayResult{Action: request.action, Redacted: true}
	switch request.action {
	case gatewayActionSurfaceSet:
		status, err := n.gatewaySurfaceSet(ctx, request.toolID, request.input)
		if err != nil {
			return nativeGatewayResult{}, err
		}
		payload := core.GatewayStatusPayload(status)
		result.Status = &payload
	case gatewayActionProviderEnable:
		status, err := n.gatewayProviderEnable(ctx, request.toolID, request.input)
		if err != nil {
			return nativeGatewayResult{}, err
		}
		payload := core.GatewayStatusPayload(status)
		result.Status = &payload
	case gatewayActionProviderDisable:
		status, err := n.gatewayProviderDisable(ctx, request.toolID, request.input)
		if err != nil {
			return nativeGatewayResult{}, err
		}
		payload := core.GatewayStatusPayload(status)
		result.Status = &payload
	case gatewayActionDeviceRename:
		device, err := n.gatewayDeviceRename(ctx, request.toolID, request.input)
		if err != nil {
			return nativeGatewayResult{}, err
		}
		payload := core.GatewayDevicePayload(device)
		result.Device = &payload
	case gatewayActionDeviceRevoke:
		revoked, err := n.gatewayDeviceRevoke(ctx, request.toolID, request.input)
		if err != nil {
			return nativeGatewayResult{}, err
		}
		result.Revoke = &contract.GatewayRevokePayload{
			Device: core.GatewayDevicePayload(revoked.Device), Changed: revoked.Changed, Canceled: revoked.Canceled,
		}
	default:
		return nativeGatewayResult{}, nativeGatewayInputError(request.toolID, "action", "unsupported gateway action")
	}
	return result, nil
}

func (n *daemonNativeTools) executeGatewayIngressAction(
	ctx context.Context,
	request nativeGatewayActionRequest,
) (nativeGatewayResult, error) {
	result := nativeGatewayResult{Action: request.action, Redacted: true}
	switch request.action {
	case gatewayActionIngressBind:
		binding, err := n.gatewayIngressBind(ctx, request.scope, request.toolID, request.input)
		if err != nil {
			return nativeGatewayResult{}, err
		}
		result.Binding = binding
	case gatewayActionIngressUnbind:
		unbound, err := n.gatewayIngressUnbind(ctx, request.scope, request.toolID, request.input)
		if err != nil {
			return nativeGatewayResult{}, err
		}
		result.Unbind = unbound
	default:
		return nativeGatewayResult{}, nativeGatewayInputError(request.toolID, "action", "unsupported gateway action")
	}
	return result, nil
}

func isGatewayReadAction(action string) bool {
	switch action {
	case gatewayActionStatus, gatewayActionAudit, gatewayActionDeviceList:
		return true
	default:
		return false
	}
}

func (n *daemonNativeTools) authorizeGatewayMutation(
	ctx context.Context,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
) error {
	if scope.Operator {
		return nil
	}
	sessionID := strings.TrimSpace(scope.SessionID)
	if sessionID == "" || n.deps.GatewayPermissionMode == nil {
		return nativeGatewayMutationDenied(toolID, nil)
	}
	mode, err := n.deps.GatewayPermissionMode(ctx, sessionID)
	if err != nil {
		return nativeGatewayMutationDenied(toolID, err)
	}
	if strings.TrimSpace(mode) != string(toolspkg.PermissionModeApproveAll) {
		return nativeGatewayMutationDenied(toolID, nil)
	}
	return nil
}

func nativeGatewayPermissionModeSource(
	sessions core.SessionManager,
) func(context.Context, string) (string, error) {
	if sessions == nil {
		return nil
	}
	return func(ctx context.Context, sessionID string) (string, error) {
		info, err := sessions.Status(ctx, strings.TrimSpace(sessionID))
		if err != nil {
			return "", fmt.Errorf("daemon: read gateway caller permission mode: %w", err)
		}
		if info == nil {
			return "", errors.New("daemon: gateway caller session status is nil")
		}
		return strings.TrimSpace(info.EffectivePermissions), nil
	}
}

func nativeGatewayMutationDenied(toolID toolspkg.ToolID, cause error) error {
	if cause == nil {
		cause = toolspkg.ErrToolDenied
	} else {
		cause = fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, cause)
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		toolID,
		"gateway management requires an operator or an approve-all session",
		cause,
		toolspkg.ReasonPolicyDenied,
	)
}

func nativeGatewayInputError(toolID toolspkg.ToolID, field string, detail string) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		toolID,
		fmt.Sprintf("%s is invalid: %s", field, detail),
		toolspkg.ErrToolInvalidInput,
		toolspkg.ReasonSchemaInvalid,
	)
}
