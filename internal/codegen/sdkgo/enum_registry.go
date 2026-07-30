package sdkgo

import (
	"reflect"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

var generatedEnumValues = map[reflect.Type][]string{
	reflect.TypeFor[extensioncontract.IssueSeverity]():   extensioncontract.IssueSeverityValues(),
	reflect.TypeFor[extensioncontract.CommandFlagType](): extensioncontract.CommandFlagTypeValues(),
}
