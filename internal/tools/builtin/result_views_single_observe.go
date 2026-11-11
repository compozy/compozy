package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultObserve = map[toolspkg.ToolID]SingleResultReason{
	"compozy__observe_search": SingleResultContent,
}
