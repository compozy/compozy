package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultResources = map[toolspkg.ToolID]SingleResultReason{
	"compozy__resources_info": SingleResultContent,
}
