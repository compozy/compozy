package spec

import (
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
)

func schemaRefForValue(value any, schemas openapi3.Schemas) (*openapi3.SchemaRef, error) {
	var rootType reflect.Type
	if value != nil {
		rootType = reflect.TypeOf(value)
		switch rootType.Kind() {
		case reflect.Struct, reflect.Slice, reflect.Array:
			value = reflect.New(rootType).Interface()
		}
	}
	schemaRef, err := openapi3gen.NewSchemaRefForValue(
		value,
		schemas,
		openapi3gen.SchemaCustomizer(schemaCustomizer),
	)
	if err != nil {
		return nil, err
	}
	applySchemaRequirements(schemaRef, rootType)
	resolveSchemaReferences(schemaRef, schemas, make(map[*openapi3.Schema]struct{}))
	return schemaRef, nil
}

func resolveComponentSchemaReferences(schemas openapi3.Schemas) {
	seen := make(map[*openapi3.Schema]struct{})
	for _, schemaRef := range schemas {
		resolveSchemaReferences(schemaRef, schemas, seen)
	}
}

func resolveSchemaReferences(
	schemaRef *openapi3.SchemaRef,
	schemas openapi3.Schemas,
	seen map[*openapi3.Schema]struct{},
) {
	if schemaRef == nil {
		return
	}
	if schemaRef.Value == nil {
		const componentPrefix = "#/components/schemas/"
		name := strings.TrimPrefix(schemaRef.Ref, componentPrefix)
		if name != schemaRef.Ref {
			if target := schemas[name]; target != nil {
				schemaRef.Value = target.Value
			}
		}
	}
	if schemaRef.Value == nil {
		return
	}
	if _, visited := seen[schemaRef.Value]; visited {
		return
	}
	seen[schemaRef.Value] = struct{}{}
	for _, property := range schemaRef.Value.Properties {
		resolveSchemaReferences(property, schemas, seen)
	}
	resolveSchemaReferences(schemaRef.Value.Items, schemas, seen)
	resolveSchemaReferences(schemaRef.Value.AdditionalProperties.Schema, schemas, seen)
	for _, nested := range schemaRef.Value.OneOf {
		resolveSchemaReferences(nested, schemas, seen)
	}
	for _, nested := range schemaRef.Value.AnyOf {
		resolveSchemaReferences(nested, schemas, seen)
	}
	for _, nested := range schemaRef.Value.AllOf {
		resolveSchemaReferences(nested, schemas, seen)
	}
	resolveSchemaReferences(schemaRef.Value.Not, schemas, seen)
}

func buildParameter(spec ParameterSpec) *openapi3.Parameter {
	var param *openapi3.Parameter
	switch spec.In {
	case openapi3.ParameterInPath:
		param = openapi3.NewPathParameter(spec.Name)
	case openapi3.ParameterInHeader:
		param = &openapi3.Parameter{Name: spec.Name, In: openapi3.ParameterInHeader}
	default:
		param = openapi3.NewQueryParameter(spec.Name)
	}
	param.WithRequired(spec.Required)
	if spec.Description != "" {
		param.WithDescription(spec.Description)
	}
	param.Schema = schemaRefForParameter(spec)
	return param
}

func schemaRefForParameter(spec ParameterSpec) *openapi3.SchemaRef {
	var schema *openapi3.Schema
	switch spec.Kind {
	case "boolean":
		schema = openapi3.NewBoolSchema()
	case specIntegerKey:
		schema = openapi3.NewIntegerSchema()
		if spec.Format != "" {
			schema.Format = spec.Format
		}
	default:
		schema = openapi3.NewStringSchema()
		if spec.Format != "" {
			schema.Format = spec.Format
		}
	}
	if len(spec.Enum) > 0 {
		schema.Enum = make([]any, 0, len(spec.Enum))
		for _, value := range spec.Enum {
			schema.Enum = append(schema.Enum, value)
		}
	}
	if spec.Maximum != nil {
		schema.WithMax(*spec.Maximum)
	}
	return openapi3.NewSchemaRef("", schema)
}

func schemaCustomizer(_ string, t reflect.Type, _ reflect.StructTag, schema *openapi3.Schema) error {
	if customizer, ok := schemaCustomizers[t]; ok {
		customizer(schema)
		return nil
	}
	if values, ok := schemaEnumValues[t]; ok {
		setStringEnum(schema, values)
	}
	return nil
}

func applySchemaRequirements(schemaRef *openapi3.SchemaRef, t reflect.Type) {
	applySchemaRequirementsSeen(schemaRef, t, make(map[schemaRequirementVisit]struct{}))
}

type schemaRequirementVisit struct {
	schema *openapi3.Schema
	typeOf reflect.Type
}

func applySchemaRequirementsSeen(
	schemaRef *openapi3.SchemaRef,
	t reflect.Type,
	seen map[schemaRequirementVisit]struct{},
) {
	if schemaRef == nil || schemaRef.Value == nil || t == nil {
		return
	}

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	visit := schemaRequirementVisit{schema: schemaRef.Value, typeOf: t}
	if _, visited := seen[visit]; visited {
		return
	}
	seen[visit] = struct{}{}

	switch t.Kind() {
	case reflect.Array, reflect.Slice:
		applySchemaRequirementsSeen(schemaRef.Value.Items, t.Elem(), seen)
	case reflect.Map:
		if schemaRef.Value.AdditionalProperties.Schema != nil {
			applySchemaRequirementsSeen(schemaRef.Value.AdditionalProperties.Schema, t.Elem(), seen)
		}
	case reflect.Struct:
		applyStructRequirements(schemaRef.Value, t, seen)
	}
}

func applyStructRequirements(
	schema *openapi3.Schema,
	t reflect.Type,
	seen map[schemaRequirementVisit]struct{},
) {
	if schema == nil || t.Kind() != reflect.Struct {
		return
	}
	if t.PkgPath() == "time" && t.Name() == "Time" {
		return
	}
	if schema.Properties == nil {
		return
	}

	required := make(map[string]struct{}, len(schema.Properties))
	collectStructRequirements(schema, t, required, seen)
	if len(required) == 0 {
		schema.Required = nil
		return
	}

	schema.Required = schema.Required[:0]
	for name := range required {
		schema.Required = append(schema.Required, name)
	}
	sort.Strings(schema.Required)
}

func collectStructRequirements(
	schema *openapi3.Schema,
	t reflect.Type,
	required map[string]struct{},
	seen map[schemaRequirementVisit]struct{},
) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}

	for field := range t.Fields() {
		if !field.IsExported() && !field.Anonymous {
			continue
		}

		jsonName, omitEmpty, skip := parseJSONField(field)
		if skip {
			continue
		}

		if field.Anonymous && field.Tag.Get("json") == "" {
			collectStructRequirements(schema, field.Type, required, seen)
			continue
		}

		propertyRef, ok := schema.Properties[jsonName]
		if !ok {
			continue
		}

		if !omitEmpty {
			required[jsonName] = struct{}{}
		}
		applySchemaRequirementsSeen(propertyRef, field.Type, seen)
	}
}

func parseJSONField(field reflect.StructField) (name string, omitEmpty bool, skip bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}

	if tag == "" {
		return field.Name, false, false
	}

	parts := strings.Split(tag, ",")
	if len(parts) > 0 && parts[0] != "" {
		name = parts[0]
	} else {
		name = field.Name
	}
	if slices.Contains(parts[1:], "omitempty") {
		omitEmpty = true
	}
	return name, omitEmpty, false
}

func setStringEnum(schema *openapi3.Schema, values []string) {
	nullable := schema.Nullable
	*schema = *openapi3.NewStringSchema()
	schema.Nullable = nullable
	schema.Enum = make([]any, 0, len(values))
	for _, value := range values {
		schema.Enum = append(schema.Enum, value)
	}
}

func enumAsAny(values []string) []any {
	converted := make([]any, 0, len(values))
	for _, value := range values {
		converted = append(converted, value)
	}
	return converted
}
