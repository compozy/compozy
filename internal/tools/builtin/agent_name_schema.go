package builtin

import (
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

var agentNameInputSchema = fmt.Sprintf(
	`{"type":"string","pattern":%q,"maxLength":%d}`,
	compozyconfig.AgentNamePattern,
	compozyconfig.AgentNameMaxLength,
)

var optionalAgentNameInputSchema = fmt.Sprintf(
	`{"type":"string","pattern":%q,"maxLength":%d}`,
	compozyconfig.OptionalAgentNamePattern,
	compozyconfig.AgentNameMaxLength,
)
