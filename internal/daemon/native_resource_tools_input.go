package daemon

import (
	"strings"

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
	if parsedScope, ok, err := resources.ParseResourceScopePair(input.ScopeKind, input.ScopeID, "scope"); err != nil {
		return resources.ResourceFilter{}, nativeResourceToolError(req.ToolID, err)
	} else if ok {
		filter.Scope = &parsedScope
	}
	if owner, ok, err := resources.ParseResourceOwnerPair(input.OwnerKind, input.OwnerID, "owner"); err != nil {
		return resources.ResourceFilter{}, nativeResourceToolError(req.ToolID, err)
	} else if ok {
		filter.Owner = &owner
	}
	if source, ok, err := resources.ParseResourceSourcePair(input.SourceKind, input.SourceID, "source"); err != nil {
		return resources.ResourceFilter{}, nativeResourceToolError(req.ToolID, err)
	} else if ok {
		filter.Source = &source
	}
	if err := nativeApplyResourceScope(req.ToolID, &filter, scope); err != nil {
		return resources.ResourceFilter{}, err
	}
	return filter, nil
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
