package spec

import bridgepkg "github.com/compozy/agh/internal/bridges"

func bridgeCatalogQueryParams() []ParameterSpec {
	return []ParameterSpec{
		enumQueryParam(specScopeKey, "Filter by bridge scope", []string{specAllKey, specGlobalKey, specWorkspaceKey}),
		queryParam("workspace_id", "Filter by active workspace id", false),
		queryParam(specWorkspaceKey, "Filter by workspace id, name, or path", false),
		queryParam("q", "Search display name, platform, extension, or effective status", false),
		queryParam("platform", "Filter by messaging platform", false),
		enumQueryParam("status", "Filter by effective runtime status", []string{
			string(bridgepkg.BridgeStatusDisabled),
			string(bridgepkg.BridgeStatusStarting),
			string(bridgepkg.BridgeStatusReady),
			string(bridgepkg.BridgeStatusDegraded),
			string(bridgepkg.BridgeStatusAuthRequired),
			string(bridgepkg.BridgeStatusError),
		}),
		enumQueryParam("sort", "Stable global-first display-name order", []string{bridgepkg.BridgeCatalogSortName}),
		queryParam("cursor", "Continue after this query-bound bridge cursor", false),
		intQueryParam("limit", "Maximum number of bridges to return; defaults to 50 and is capped at 200"),
	}
}
