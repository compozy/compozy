package spec

import (
	"reflect"
	"slices"
)

type responseBodies struct {
	schemaTypes []reflect.Type
}

type responseBodyType struct {
	typeOf reflect.Type
}

// responseBodyOf retains immutable type metadata instead of mutable schema values.
func responseBodyOf[T any]() responseBodyType {
	return responseBodyType{typeOf: reflect.TypeFor[T]()}
}

func responseBodiesOf(bodyTypes ...responseBodyType) responseBodies {
	schemaTypes := make([]reflect.Type, len(bodyTypes))
	for index, bodyType := range bodyTypes {
		schemaTypes[index] = bodyType.typeOf
	}
	return responseBodies{schemaTypes: schemaTypes}
}

func (bodies responseBodies) empty() bool {
	return len(bodies.schemaTypes) == 0
}

func (bodies responseBodies) clone() responseBodies {
	return responseBodies{schemaTypes: slices.Clone(bodies.schemaTypes)}
}
