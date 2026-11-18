package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultGateway = map[toolspkg.ToolID]SingleResultReason{
	"compozy__gateway": SingleResultMinimal,
}
