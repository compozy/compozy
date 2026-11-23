package daemon

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/ingress"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) gatewaySurfaceSet(
	ctx context.Context,
	toolID toolspkg.ToolID,
	input nativeGatewayInput,
) (gateway.Status, error) {
	surface, err := parseNativeGatewaySurface(toolID, input.Surface)
	if err != nil {
		return gateway.Status{}, err
	}
	tier, err := parseNativeGatewayTier(toolID, input.Tier)
	if err != nil {
		return gateway.Status{}, err
	}
	desired, err := parseNativeGatewayDesired(toolID, input.Desired)
	if err != nil {
		return gateway.Status{}, err
	}
	return n.deps.gatewayService().SetSurfaceExposure(ctx, gateway.SurfaceExposureRequest{
		Surface: surface, Tier: tier, Desired: desired,
		ExpectedGeneration: input.ExpectedGeneration, Consent: input.Consent,
	})
}

func (n *daemonNativeTools) gatewayProviderEnable(
	ctx context.Context,
	toolID toolspkg.ToolID,
	input nativeGatewayInput,
) (gateway.Status, error) {
	provider, err := requiredNativeString(toolID, "provider", input.Provider)
	if err != nil {
		return gateway.Status{}, err
	}
	installSource, err := requiredNativeString(toolID, "install_source", input.InstallSource)
	if err != nil {
		return gateway.Status{}, err
	}
	tier, err := parseNativeGatewayTier(toolID, input.Tier)
	if err != nil {
		return gateway.Status{}, err
	}
	return n.deps.gatewayService().EnableProvider(ctx, gateway.ProviderEnableRequest{
		Provider: gateway.ProviderIdentity{
			Name: provider, InstallSource: installSource, DigestConfirmed: strings.TrimSpace(input.DigestConfirmed),
		},
		Tier: tier, ExpectedGeneration: input.ExpectedGeneration,
	})
}

func (n *daemonNativeTools) gatewayProviderDisable(
	ctx context.Context,
	toolID toolspkg.ToolID,
	input nativeGatewayInput,
) (gateway.Status, error) {
	provider, err := requiredNativeString(toolID, "provider", input.Provider)
	if err != nil {
		return gateway.Status{}, err
	}
	tier, err := parseNativeGatewayTier(toolID, input.Tier)
	if err != nil {
		return gateway.Status{}, err
	}
	return n.deps.gatewayService().DisableProvider(ctx, tier, provider)
}

func parseNativeGatewayTier(toolID toolspkg.ToolID, value string) (gateway.Tier, error) {
	tier := gateway.Tier(strings.TrimSpace(value))
	if err := tier.Validate(); err != nil {
		return "", nativeGatewayInputError(toolID, "tier", err.Error())
	}
	return tier, nil
}

func parseNativeGatewaySurface(toolID toolspkg.ToolID, value string) (gateway.Surface, error) {
	surface := gateway.Surface(strings.TrimSpace(value))
	if err := surface.Validate(); err != nil {
		return "", nativeGatewayInputError(toolID, "surface", err.Error())
	}
	return surface, nil
}

func parseNativeGatewayDesired(toolID toolspkg.ToolID, value string) (gateway.DesiredState, error) {
	desired := gateway.DesiredState(strings.TrimSpace(value))
	if err := desired.Validate(); err != nil {
		return "", nativeGatewayInputError(toolID, "desired", err.Error())
	}
	return desired, nil
}

func (n *daemonNativeTools) gatewayDeviceRename(
	ctx context.Context,
	toolID toolspkg.ToolID,
	input nativeGatewayInput,
) (gateway.DeviceSession, error) {
	deviceID, err := requiredNativeString(toolID, "device_id", input.DeviceID)
	if err != nil {
		return gateway.DeviceSession{}, err
	}
	name, err := requiredNativeString(toolID, "name", input.Name)
	if err != nil {
		return gateway.DeviceSession{}, err
	}
	return n.deps.gatewayService().RenameDevice(ctx, deviceID, name)
}

func (n *daemonNativeTools) gatewayDeviceRevoke(
	ctx context.Context,
	toolID toolspkg.ToolID,
	input nativeGatewayInput,
) (gateway.RevokeResult, error) {
	deviceID, err := requiredNativeString(toolID, "device_id", input.DeviceID)
	if err != nil {
		return gateway.RevokeResult{}, err
	}
	return n.deps.gatewayService().RevokeDevice(ctx, deviceID)
}

func (n *daemonNativeTools) gatewayDeviceRemove(
	ctx context.Context,
	toolID toolspkg.ToolID,
	input nativeGatewayInput,
) (gateway.RemoveResult, error) {
	deviceID, err := requiredNativeString(toolID, "device_id", input.DeviceID)
	if err != nil {
		return gateway.RemoveResult{}, err
	}
	return n.deps.gatewayService().RemoveDevice(ctx, deviceID)
}

func (n *daemonNativeTools) gatewayIngressBind(
	ctx context.Context,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
	input nativeGatewayInput,
) (*contract.GatewayIngressBindingResponse, error) {
	ref, err := nativeGatewayIngressRef(toolID, input)
	if err != nil {
		return nil, err
	}
	service := n.deps.ingressService()
	if service == nil {
		return nil, ingress.ErrSubjectNotFound
	}
	binding, err := service.Bind(ctx, ingress.BindRequest{
		Subject: ref, Confirmed: input.Confirmed, Caller: nativeGatewayIngressCaller(scope),
	})
	if err != nil {
		return nil, err
	}
	projection, err := service.Project(ctx, ref)
	if err != nil {
		return nil, err
	}
	return &contract.GatewayIngressBindingResponse{
		Binding: core.GatewayIngressPayload(projection), Changed: binding.Changed,
	}, nil
}

func (n *daemonNativeTools) gatewayIngressUnbind(
	ctx context.Context,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
	input nativeGatewayInput,
) (*contract.GatewayIngressUnbindResponse, error) {
	ref, err := nativeGatewayIngressRef(toolID, input)
	if err != nil {
		return nil, err
	}
	service := n.deps.ingressService()
	if service == nil {
		return nil, ingress.ErrSubjectNotFound
	}
	result, err := service.Unbind(ctx, ingress.UnbindRequest{
		Subject: ref, Caller: nativeGatewayIngressCaller(scope),
	})
	if err != nil {
		return nil, err
	}
	payload := core.GatewayIngressUnbindPayload(result)
	return &payload, nil
}

func nativeGatewayIngressRef(
	toolID toolspkg.ToolID,
	input nativeGatewayInput,
) (ingress.SubjectRef, error) {
	ref := ingress.SubjectRef{
		Kind: ingress.SubjectKind(strings.TrimSpace(input.SubjectKind)),
		ID:   strings.TrimSpace(input.SubjectID),
	}
	if err := ref.Validate(); err != nil {
		return ingress.SubjectRef{}, nativeGatewayInputError(toolID, "subject", err.Error())
	}
	return ref, nil
}

func nativeGatewayIngressCaller(scope toolspkg.Scope) *ingress.Caller {
	if strings.TrimSpace(scope.SessionID) == "" {
		return nil
	}
	return &ingress.Caller{
		SessionID: strings.TrimSpace(scope.SessionID), WorkspaceID: strings.TrimSpace(scope.WorkspaceID),
		AgentName: strings.TrimSpace(scope.AgentName),
	}
}
