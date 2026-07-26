package spec

import memorypkg "github.com/compozy/agh/internal/memory"

func memoryListQueryParams() []ParameterSpec {
	return append(
		memorySelectorQueryParams(),
		enumQueryParam("type", "Filter by memory type", memoryTypeValues()),
		enumQueryParam(
			"sort",
			"Order memory headers by recent modification or normalized name",
			[]string{memorypkg.HeaderListSortRecent, memorypkg.HeaderListSortName},
		),
		queryParam("cursor", "Continue after this query-bound memory cursor", false),
		intQueryParam("limit", "Maximum number of memories to return; defaults to 50 and is capped at 200"),
		boolQueryParam("include_system", "Include system-managed memory entries"),
	)
}
