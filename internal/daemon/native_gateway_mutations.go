package daemon

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) gatewaySurfaceSet(
	ctx context.Context,
	toolID toolspkg.ToolID,
	input nativeGatewayInput,
) (gateway.Status, error) {
	surface := gateway.Surface(strings.TrimSpace(input.Surface))
	if err := surface.Validate(); err != nil {
		return gateway.Status{}, nativeGatewayInputError(toolID, "surface", err.Error())
	}
	tier := gateway.Tier(strings.TrimSpace(input.Tier))
	if err := tier.Validate(); err != nil {
		return gateway.Status{}, nativeGatewayInputError(toolID, "tier", err.Error())
	}
	desired := gateway.DesiredState(strings.TrimSpace(input.Desired))
	if err := desired.Validate(); err != nil {
		return gateway.Status{}, nativeGatewayInputError(toolID, "desired", err.Error())
	}
	return n.deps.Gateway.SetSurfaceExposure(ctx, gateway.SurfaceExposureRequest{
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
	tier := gateway.Tier(strings.TrimSpace(input.Tier))
	if err := tier.Validate(); err != nil {
		return gateway.Status{}, nativeGatewayInputError(toolID, "tier", err.Error())
	}
	return n.deps.Gateway.EnableProvider(ctx, gateway.ProviderEnableRequest{
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
	tier := gateway.Tier(strings.TrimSpace(input.Tier))
	if err := tier.Validate(); err != nil {
		return gateway.Status{}, nativeGatewayInputError(toolID, "tier", err.Error())
	}
	return n.deps.Gateway.DisableProvider(ctx, tier, provider)
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
	return n.deps.Gateway.RenameDevice(ctx, deviceID, name)
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
	return n.deps.Gateway.RevokeDevice(ctx, deviceID)
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
	binding, err := n.deps.Gateway.BindIngress(ctx, gateway.IngressBindRequest{
		Subject: ref, Confirmed: input.Confirmed, Caller: nativeGatewayIngressCaller(scope),
	})
	if err != nil {
		return nil, err
	}
	projection, err := n.deps.Gateway.ProjectIngress(ctx, ref)
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
	result, err := n.deps.Gateway.UnbindIngress(ctx, gateway.IngressUnbindRequest{
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
) (gateway.IngressSubjectRef, error) {
	ref := gateway.IngressSubjectRef{
		Kind: gateway.IngressSubjectKind(strings.TrimSpace(input.SubjectKind)),
		ID:   strings.TrimSpace(input.SubjectID),
	}
	if err := ref.Validate(); err != nil {
		return gateway.IngressSubjectRef{}, nativeGatewayInputError(toolID, "subject", err.Error())
	}
	return ref, nil
}

func nativeGatewayIngressCaller(scope toolspkg.Scope) *gateway.IngressCaller {
	if strings.TrimSpace(scope.SessionID) == "" {
		return nil
	}
	return &gateway.IngressCaller{
		SessionID: strings.TrimSpace(scope.SessionID), WorkspaceID: strings.TrimSpace(scope.WorkspaceID),
		AgentName: strings.TrimSpace(scope.AgentName),
	}
}
