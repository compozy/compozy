package sdkgo

import (
	"reflect"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

var generatedEnumValues = func() map[reflect.Type][]string {
	contracts := extensioncontract.SDKEnumContracts()
	values := make(map[reflect.Type][]string, len(contracts))
	for _, contract := range contracts {
		values[reflect.TypeOf(contract.Value)] = append([]string(nil), contract.Values...)
	}
	return values
}()
