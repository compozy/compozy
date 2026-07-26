package spec

import (
	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
)

func loopCatalogQueryParams() []ParameterSpec {
	return []ParameterSpec{
		queryParam("q", "Search Loop name or contract goal", false),
		enumQueryParam("kind", "Filter by effective Loop kind", []string{
			string(looppkg.CatalogKindReadOnly),
			string(looppkg.CatalogKindWorkspace),
		}),
		queryParam("category", "Filter by exact catalog category", false),
		enumQueryParam("status", "Filter by exact latest-run status", contract.LoopRunStatusValues()),
		enumQueryParam("sort", "Stable read-only-first name order", []string{looppkg.CatalogSortName}),
		queryParam("cursor", "Continue after this workspace- and query-bound Loop cursor", false),
		intQueryParam("limit", "Maximum number of Loops to return; defaults to 50 and is capped at 200"),
	}
}
