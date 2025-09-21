package main

import (
	"reflect"

	"github.com/compozy/compozy/engine/core"
	"github.com/invopop/jsonschema"
)

func newJSONSchemaReflector() *jsonschema.Reflector {
	reflector := &jsonschema.Reflector{
		RequiredFromJSONSchemaTags: true,
		AllowAdditionalProperties:  false,
		DoNotReference:             false,
		BaseSchemaID:               "",
		ExpandedStruct:             true,
		FieldNameTag:               "json",
		IgnoredTypes:               []any{&core.PathCWD{}},
	}
	reflector.Mapper = func(t reflect.Type) *jsonschema.Schema {
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
			schema := jsonschema.ReflectFromType(t)
			if schema != nil && schema.Type == "" {
				schema.Type = "string"
			}
			return schema
		}
		return nil
	}
	return reflector
}
