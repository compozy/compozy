package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/resources"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type bundleInfoInput struct {
	ID string `json:"id"`
}

type bundleActivateToolInput struct {
	ExtensionName             string `json:"extension_name"`
	BundleName                string `json:"bundle_name"`
	ProfileName               string `json:"profile_name"`
	Scope                     string `json:"scope"`
	Workspace                 string `json:"workspace"`
	ConfirmNetworkRequirement bool   `json:"confirm_network_requirement"`
}

type resourceFilterInput struct {
	Kind       string `json:"kind"`
	Limit      int    `json:"limit"`
	ScopeKind  string `json:"scope_kind"`
	ScopeID    string `json:"scope_id"`
	OwnerKind  string `json:"owner_kind"`
	OwnerID    string `json:"owner_id"`
	SourceKind string `json:"source_kind"`
	SourceID   string `json:"source_id"`
}

type resourceInfoInput struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

func (n *daemonNativeTools) bundleToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDBundlesList:       {call: n.bundlesList, availability: availability},
		toolspkg.ToolIDBundlesInfo:       {call: n.bundlesInfo, availability: availability},
		toolspkg.ToolIDBundlesActivate:   {call: n.bundlesActivate, availability: availability},
		toolspkg.ToolIDBundlesDeactivate: {call: n.bundlesDeactivate, availability: availability},
		toolspkg.ToolIDBundlesStatus:     {call: n.bundlesStatus, availability: availability},
	}
}

func (n *daemonNativeTools) resourceToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDResourcesList:     {call: n.resourcesList, availability: availability},
		toolspkg.ToolIDResourcesInfo:     {call: n.resourcesInfo, availability: availability},
		toolspkg.ToolIDResourcesSnapshot: {call: n.resourcesSnapshot, availability: availability},
	}
}

func (n *daemonNativeTools) bundlesList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	service := n.bundleService()
	catalog, err := service.Catalog(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, nativeBundleToolError(req.ToolID, err)
	}
	activations, err := service.ListActivations(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, nativeBundleToolError(req.ToolID, err)
	}
	activations = nativeBundleActivationsForScope(scope, activations)
	payload := map[string]any{
		"bundles":     core.BundleCatalogPayloads(catalog),
		"activations": bundleActivationPayloads(activations),
	}
	return structuredResult(payload, fmt.Sprintf("%d bundles", len(catalog)))
}

func (n *daemonNativeTools) bundlesInfo(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input bundleInfoInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	id, err := requiredNativeString(req.ToolID, "id", input.ID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	activation, err := n.bundleService().GetActivation(ctx, id)
	if err != nil {
		return toolspkg.ToolResult{}, nativeBundleToolError(req.ToolID, err)
	}
	if !nativeBundleActivationAllowed(scope, activation.Activation) {
		return toolspkg.ToolResult{}, nativeScopeMismatchError(req.ToolID, "id")
	}
	return structuredResult(map[string]any{"activation": core.BundleActivationPayload(activation)}, id)
}

func (n *daemonNativeTools) bundlesActivate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input bundleActivateToolInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.ConfirmNetworkRequirement && !scope.Operator {
		return toolspkg.ToolResult{}, nativeBundleConfirmationDenied(req.ToolID)
	}
	activationScope, workspaceID, err := nativeBundleActivationScope(req.ToolID, input, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	activation, err := n.bundleService().Activate(ctx, bundlepkg.ActivateRequest{
		ExtensionName:             strings.TrimSpace(input.ExtensionName),
		BundleName:                strings.TrimSpace(input.BundleName),
		ProfileName:               strings.TrimSpace(input.ProfileName),
		Scope:                     activationScope,
		Workspace:                 workspaceID,
		ConfirmNetworkRequirement: input.ConfirmNetworkRequirement,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeBundleToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{"activation": core.BundleActivationPayload(activation)},
		activation.Activation.ID,
	)
}

func (n *daemonNativeTools) bundlesDeactivate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input bundleInfoInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	id, err := requiredNativeString(req.ToolID, "id", input.ID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	activation, err := n.bundleService().GetActivation(ctx, id)
	if err != nil {
		return toolspkg.ToolResult{}, nativeBundleToolError(req.ToolID, err)
	}
	if !nativeBundleActivationAllowed(scope, activation.Activation) {
		return toolspkg.ToolResult{}, nativeScopeMismatchError(req.ToolID, "id")
	}
	if err := n.bundleService().Deactivate(ctx, id); err != nil {
		return toolspkg.ToolResult{}, nativeBundleToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{"id": id, "deactivated": true}, id)
}

func (n *daemonNativeTools) bundlesStatus(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	service := n.bundleService()
	catalog, err := service.Catalog(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, nativeBundleToolError(req.ToolID, err)
	}
	activations, err := service.ListActivations(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, nativeBundleToolError(req.ToolID, err)
	}
	network, err := service.NetworkSettings(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, nativeBundleToolError(req.ToolID, err)
	}
	activations = nativeBundleActivationsForScope(scope, activations)
	network.DeclaredChannels = nativeBundleChannelsForScope(scope, network.DeclaredChannels)
	payload := map[string]any{
		"bundle_count":     len(catalog),
		"activation_count": len(activations),
		nativeToolsNetworkKey: contract.BundleNetworkSettingsPayload{
			DeclaredChannels: core.DeclaredNetworkChannelPayloads(network.DeclaredChannels),
		},
	}
	return structuredResult(payload, fmt.Sprintf("%d active bundles", len(activations)))
}

func (n *daemonNativeTools) resourcesList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	filter, err := decodeResourceFilterInput(req, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	records, err := n.deps.Resources.List(ctx, filter)
	if err != nil {
		return toolspkg.ToolResult{}, nativeResourceToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{"records": core.ResourceRecordPayloadsFromRaw(records)},
		fmt.Sprintf("%d resources", len(records)),
	)
}

func (n *daemonNativeTools) resourcesInfo(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	kind, id, err := decodeResourceInfoInput(req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	record, err := n.deps.Resources.Get(ctx, kind, id)
	if err != nil {
		return toolspkg.ToolResult{}, nativeResourceToolError(req.ToolID, err)
	}
	if !nativeResourceRecordAllowed(scope, record) {
		return toolspkg.ToolResult{}, nativeScopeMismatchError(req.ToolID, "id")
	}
	return structuredResult(map[string]any{"record": core.ResourceRecordPayloadFromRaw(record)}, id)
}

func (n *daemonNativeTools) resourcesSnapshot(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	filter, err := decodeResourceFilterInput(req, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	records, err := n.deps.Resources.List(ctx, filter)
	if err != nil {
		return toolspkg.ToolResult{}, nativeResourceToolError(req.ToolID, err)
	}
	payload := map[string]any{
		"count":   len(records),
		"records": core.ResourceRecordPayloadsFromRaw(records),
	}
	return structuredResult(payload, fmt.Sprintf("%d resources", len(records)))
}

func bundleActivationPayloads(items []bundlepkg.ActivationPreview) []contract.BundleActivationPayload {
	if len(items) == 0 {
		return []contract.BundleActivationPayload{}
	}
	payload := make([]contract.BundleActivationPayload, 0, len(items))
	for _, item := range items {
		payload = append(payload, core.BundleActivationPayload(item))
	}
	return payload
}

func nativeBundleActivationScope(
	id toolspkg.ToolID,
	input bundleActivateToolInput,
	scope toolspkg.Scope,
) (bundlepkg.Scope, string, error) {
	activationScope := bundlepkg.Scope(strings.TrimSpace(input.Scope)).Normalize()
	if activationScope == "" {
		activationScope = bundlepkg.ScopeGlobal
		if strings.TrimSpace(scope.WorkspaceID) != "" {
			activationScope = bundlepkg.ScopeWorkspace
		}
	}
	if !scope.Operator && strings.TrimSpace(scope.WorkspaceID) != "" && activationScope != bundlepkg.ScopeWorkspace {
		return "", "", nativeScopeMismatchError(id, "scope")
	}
	workspaceID := ""
	if activationScope == bundlepkg.ScopeWorkspace {
		resolved, err := nativeCallerWorkspaceInput(id, "workspace", input.Workspace, scope)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(resolved) == "" {
			return "", "", nativeRequiredInputError(id, "workspace")
		}
		workspaceID = resolved
	} else if strings.TrimSpace(input.Workspace) != "" {
		return "", "", nativeScopeMismatchError(id, "workspace")
	}
	return activationScope, workspaceID, nil
}

func nativeBundleActivationsForScope(
	scope toolspkg.Scope,
	items []bundlepkg.ActivationPreview,
) []bundlepkg.ActivationPreview {
	if scope.Operator || strings.TrimSpace(scope.WorkspaceID) == "" {
		return items
	}
	filtered := make([]bundlepkg.ActivationPreview, 0, len(items))
	for _, item := range items {
		if nativeBundleActivationAllowed(scope, item.Activation) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func nativeBundleChannelsForScope(
	scope toolspkg.Scope,
	items []bundlepkg.DeclaredChannel,
) []bundlepkg.DeclaredChannel {
	if scope.Operator || strings.TrimSpace(scope.WorkspaceID) == "" {
		return items
	}
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	filtered := make([]bundlepkg.DeclaredChannel, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.WorkspaceID) == workspaceID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func nativeBundleActivationAllowed(scope toolspkg.Scope, activation bundlepkg.Activation) bool {
	if scope.Operator || strings.TrimSpace(scope.WorkspaceID) == "" {
		return true
	}
	return activation.Scope.Normalize() == bundlepkg.ScopeWorkspace &&
		strings.TrimSpace(activation.WorkspaceID) == strings.TrimSpace(scope.WorkspaceID)
}

func decodeResourceFilterInput(
	req toolspkg.CallRequest,
	scope toolspkg.Scope,
) (resources.ResourceFilter, error) {
	var input resourceFilterInput
	if err := decodeNativeInput(req, &input); err != nil {
		return resources.ResourceFilter{}, err
	}
	filter := resources.ResourceFilter{Limit: input.Limit}
	if kind := strings.TrimSpace(input.Kind); kind != "" {
		filter.Kind = resources.ResourceKind(kind)
		if err := filter.Kind.Validate("kind"); err != nil {
			return resources.ResourceFilter{}, nativeResourceToolError(req.ToolID, err)
		}
	}
	if scope, ok, err := resourceScopeFromInput(input.ScopeKind, input.ScopeID, "scope"); err != nil {
		return resources.ResourceFilter{}, nativeResourceToolError(req.ToolID, err)
	} else if ok {
		filter.Scope = &scope
	}
	if owner, ok, err := resourceOwnerFromInput(input.OwnerKind, input.OwnerID, "owner"); err != nil {
		return resources.ResourceFilter{}, nativeResourceToolError(req.ToolID, err)
	} else if ok {
		filter.Owner = &owner
	}
	if source, ok, err := resourceSourceFromInput(input.SourceKind, input.SourceID, "source"); err != nil {
		return resources.ResourceFilter{}, nativeResourceToolError(req.ToolID, err)
	} else if ok {
		filter.Source = &source
	}
	if err := nativeApplyResourceScope(req.ToolID, &filter, scope); err != nil {
		return resources.ResourceFilter{}, err
	}
	return filter, nil
}

func nativeApplyResourceScope(
	id toolspkg.ToolID,
	filter *resources.ResourceFilter,
	scope toolspkg.Scope,
) error {
	if filter == nil || scope.Operator || strings.TrimSpace(scope.WorkspaceID) == "" {
		return nil
	}
	workspaceScope := resources.ResourceScope{
		Kind: resources.ResourceScopeKindWorkspace,
		ID:   strings.TrimSpace(scope.WorkspaceID),
	}
	if filter.Scope == nil {
		filter.Scope = &workspaceScope
		return nil
	}
	if filter.Scope.Kind.Normalize() != resources.ResourceScopeKindWorkspace ||
		strings.TrimSpace(filter.Scope.ID) != workspaceScope.ID {
		return nativeScopeMismatchError(id, "scope")
	}
	filter.Scope.ID = workspaceScope.ID
	return nil
}

func nativeResourceRecordAllowed(scope toolspkg.Scope, record resources.RawRecord) bool {
	if scope.Operator || strings.TrimSpace(scope.WorkspaceID) == "" {
		return true
	}
	recordScope := record.Scope.Normalize()
	return recordScope.Kind == resources.ResourceScopeKindWorkspace &&
		recordScope.ID == strings.TrimSpace(scope.WorkspaceID)
}

func decodeResourceInfoInput(req toolspkg.CallRequest) (resources.ResourceKind, string, error) {
	var input resourceInfoInput
	if err := decodeNativeInput(req, &input); err != nil {
		return "", "", err
	}
	kindRaw, err := requiredNativeString(req.ToolID, "kind", input.Kind)
	if err != nil {
		return "", "", err
	}
	id, err := requiredNativeString(req.ToolID, "id", input.ID)
	if err != nil {
		return "", "", err
	}
	kind := resources.ResourceKind(kindRaw)
	if err := kind.Validate("kind"); err != nil {
		return "", "", nativeResourceToolError(req.ToolID, err)
	}
	return kind, id, nil
}

func resourceScopeFromInput(rawKind string, rawID string, path string) (resources.ResourceScope, bool, error) {
	if strings.TrimSpace(rawKind) == "" && strings.TrimSpace(rawID) == "" {
		return resources.ResourceScope{}, false, nil
	}
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKind(rawKind), ID: rawID}.Normalize()
	if err := scope.Validate(path); err != nil {
		return resources.ResourceScope{}, false, err
	}
	return scope, true, nil
}

func resourceOwnerFromInput(rawKind string, rawID string, path string) (resources.ResourceOwner, bool, error) {
	if strings.TrimSpace(rawKind) == "" && strings.TrimSpace(rawID) == "" {
		return resources.ResourceOwner{}, false, nil
	}
	owner := resources.ResourceOwner{Kind: resources.ResourceOwnerKind(rawKind), ID: rawID}.Normalize()
	if err := owner.Validate(path); err != nil {
		return resources.ResourceOwner{}, false, err
	}
	return owner, true, nil
}

func resourceSourceFromInput(rawKind string, rawID string, path string) (resources.ResourceSource, bool, error) {
	if strings.TrimSpace(rawKind) == "" && strings.TrimSpace(rawID) == "" {
		return resources.ResourceSource{}, false, nil
	}
	source := resources.ResourceSource{Kind: resources.ResourceSourceKind(rawKind), ID: rawID}.Normalize()
	if err := source.Validate(path); err != nil {
		return resources.ResourceSource{}, false, err
	}
	return source, true, nil
}

func nativeBundleToolError(id toolspkg.ToolID, err error) error {
	if errors.Is(err, aghconfig.ErrAgentNameReserved) {
		return nativeReservedAgentNameToolError(id, err)
	}
	return nativeHTTPStatusToolError(id, err, core.StatusForBundleError(err))
}

func nativeResourceToolError(id toolspkg.ToolID, err error) error {
	return nativeHTTPStatusToolError(id, err, core.StatusForResourceError(err))
}
