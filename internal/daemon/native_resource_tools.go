package daemon

import (
	"context"
	"fmt"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/resources"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

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

func (n *daemonNativeTools) resourceToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDResourcesList:     {call: n.resourcesList, availability: availability},
		toolspkg.ToolIDResourcesInfo:     {call: n.resourcesInfo, availability: availability},
		toolspkg.ToolIDResourcesSnapshot: {call: n.resourcesSnapshot, availability: availability},
	}
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

func nativeResourceToolError(id toolspkg.ToolID, err error) error {
	return nativeHTTPStatusToolError(id, err, core.StatusForResourceError(err))
}
