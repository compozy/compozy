package spec

import (
	"reflect"
	"slices"
)

type responseBodies struct {
	schemaTypes []reflect.Type
	overlapping bool
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

// overlappingResponseBodiesOf permits payloads that satisfy multiple alternatives.
func overlappingResponseBodiesOf(bodyTypes ...responseBodyType) responseBodies {
	bodies := responseBodiesOf(bodyTypes...)
	bodies.overlapping = true
	return bodies
}

func (bodies responseBodies) empty() bool {
	return len(bodies.schemaTypes) == 0
}

func (bodies responseBodies) clone() responseBodies {
	return responseBodies{schemaTypes: slices.Clone(bodies.schemaTypes), overlapping: bodies.overlapping}
}

// BodyTypes returns independent type descriptors for the primary response media type.
func (response ResponseSpec) BodyTypes() []reflect.Type {
	if response.Body != nil {
		return []reflect.Type{reflect.TypeOf(response.Body)}
	}
	return slices.Clone(response.Bodies.schemaTypes)
}
