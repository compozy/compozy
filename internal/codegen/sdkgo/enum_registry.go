package sdkgo

import (
	"reflect"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

var generatedEnumValues = map[reflect.Type][]string{
	reflect.TypeFor[apicontract.IssueSeverity]():         apicontract.IssueSeverityValues(),
	reflect.TypeFor[extensioncontract.CommandFlagType](): extensioncontract.CommandFlagTypeValues(),
}
