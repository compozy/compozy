package contract

import (
	apicontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/hooks"
)

// SDKEnumContract binds one daemon enum type to its closed public wire vocabulary.
type SDKEnumContract struct {
	Value  any
	Values []string
}

// SDKEnumContracts returns the enums whose Go SDK declarations require generated constants.
func SDKEnumContracts() []SDKEnumContract {
	return []SDKEnumContract{
		{Value: IssueSeverity(""), Values: IssueSeverityValues()},
		{Value: CommandFlagType(""), Values: CommandFlagTypeValues()},
		{Value: hooks.LoopGenerationOrigin(""), Values: hooks.LoopGenerationOriginValues()},
		{Value: apicontract.LoopProvenanceRole(""), Values: apicontract.LoopProvenanceRoleValues()},
	}
}
