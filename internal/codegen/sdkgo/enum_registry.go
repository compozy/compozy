package sdkgo

import (
	"reflect"

	apicontract "github.com/compozy/compozy/internal/api/contract"
)

var generatedEnumValues = map[reflect.Type][]string{
	reflect.TypeFor[apicontract.IssueSeverity](): apicontract.IssueSeverityValues(),
}
