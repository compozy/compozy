package contract

import (
	apicontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// SDKEnumContract binds one daemon enum type to its closed public wire vocabulary.
type SDKEnumContract struct {
	Value  any
	Values []string
}

// SDKEnumContracts returns the enums whose Go SDK declarations require generated constants.
func SDKEnumContracts() []SDKEnumContract {
	return []SDKEnumContract{
		{Value: session.Disposition(""), Values: session.DispositionValues()},
		{Value: store.SteerDeliveryMode(""), Values: store.SteerDeliveryModeValues()},
		{Value: IssueSeverity(""), Values: IssueSeverityValues()},
		{Value: CommandFlagType(""), Values: CommandFlagTypeValues()},
		{Value: ModelSourceOptionKind(""), Values: ModelSourceOptionKindValues()},
		{Value: hooks.LoopGenerationOrigin(""), Values: hooks.LoopGenerationOriginValues()},
		{Value: apicontract.LoopProvenanceRole(""), Values: apicontract.LoopProvenanceRoleValues()},
	}
}
