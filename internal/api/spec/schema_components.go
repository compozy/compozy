package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"strings"
	"sync"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
)

var operationComponentNames = sync.OnceValues(func() (map[reflect.Type]string, error) {
	var roots []reflect.Type
	for _, operation := range Operations() {
		roots = append(roots, reflect.TypeOf(operation.RequestBody))
		for _, response := range operation.Responses {
			roots = append(roots, reflect.TypeOf(response.Body))
			roots = append(roots, response.Bodies.schemaTypes...)
			for _, body := range response.AdditionalContent {
				roots = append(roots, reflect.TypeOf(body))
			}
		}
	}
	for _, component := range extensionAuthoringComponentSchemas {
		roots = append(roots, reflect.TypeOf(component.value))
	}
	for _, component := range windowManagerComponentSchemas {
		roots = append(roots, reflect.TypeOf(component.value))
	}
	return componentNamesForTypes(roots...)
})

func componentNamesForTypes(roots ...reflect.Type) (map[reflect.Type]string, error) {
	types := make(map[reflect.Type]struct{})
	for _, root := range roots {
		collectSchemaTypes(root, types)
	}
	counts := make(map[string]int)
	for typeOf := range types {
		if isComponentType(typeOf) {
			counts[typeOf.Name()]++
		}
	}
	names := make(map[reflect.Type]string)
	owners := make(map[string]reflect.Type)
	for typeOf := range types {
		if !isComponentType(typeOf) {
			continue
		}
		name := typeOf.Name()
		if fixed := fixedSchemaComponentName(typeOf); fixed != "" {
			name = fixed
		} else if counts[name] > 1 {
			name = pascalPackageName(typeOf.PkgPath()) + name
		}
		if owner, exists := owners[name]; exists && owner != typeOf {
			return nil, fmt.Errorf("duplicate component name %q for %s and %s", name, owner, typeOf)
		}
		owners[name], names[typeOf] = typeOf, name
	}
	return names, nil
}

func collectSchemaTypes(typeOf reflect.Type, seen map[reflect.Type]struct{}) {
	if typeOf == nil {
		return
	}
	for typeOf.Kind() == reflect.Pointer {
		typeOf = typeOf.Elem()
	}
	if _, exists := seen[typeOf]; exists {
		return
	}
	seen[typeOf] = struct{}{}
	switch typeOf.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		collectSchemaTypes(typeOf.Elem(), seen)
	case reflect.Struct:
		if typeOf.PkgPath() == "time" {
			return
		}
		for field := range typeOf.Fields() {
			if !field.IsExported() && !field.Anonymous {
				continue
			}
			if field.Tag.Get("json") == "-" {
				continue
			}
			collectSchemaTypes(field.Type, seen)
		}
	}
}

func isComponentType(typeOf reflect.Type) bool {
	return typeOf.Kind() == reflect.Struct && typeOf.Name() != "" && typeOf.PkgPath() != "time"
}

func pascalPackageName(packagePath string) string {
	var result strings.Builder
	upper := true
	for _, character := range path.Base(packagePath) {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			upper = true
			continue
		}
		if upper {
			character, upper = unicode.ToUpper(character), false
		}
		result.WriteRune(character)
	}
	return result.String()
}

func componentizeSchema(
	schemaRef *openapi3.SchemaRef,
	sources map[*openapi3.Schema]reflect.Type,
	names map[reflect.Type]string,
	schemas openapi3.Schemas,
	seen map[*openapi3.Schema]struct{},
) error {
	if schemaRef == nil || schemaRef.Value == nil {
		return nil
	}
	schema := schemaRef.Value
	if _, visited := seen[schema]; !visited {
		seen[schema] = struct{}{}
		for _, property := range schema.Properties {
			if err := componentizeSchema(property, sources, names, schemas, seen); err != nil {
				return err
			}
		}
		if err := componentizeSchema(schema.Items, sources, names, schemas, seen); err != nil {
			return err
		}
		if err := componentizeSchema(schema.AdditionalProperties.Schema, sources, names, schemas, seen); err != nil {
			return err
		}
		for _, alternatives := range []openapi3.SchemaRefs{schema.OneOf, schema.AnyOf, schema.AllOf} {
			for _, nested := range alternatives {
				if err := componentizeSchema(nested, sources, names, schemas, seen); err != nil {
					return err
				}
			}
		}
		if err := componentizeSchema(schema.Not, sources, names, schemas, seen); err != nil {
			return err
		}
	}
	typeOf, known := sources[schema]
	if !known || !isComponentSchema(typeOf) {
		return nil
	}
	name := names[typeOf]
	if name == "" {
		name = typeOf.Name()
	}
	nullable := schema.Nullable
	canonical := *schema
	canonical.Nullable = false
	var err error
	name, err = contextualComponentName(name, typeOf, schema, &canonical, schemas)
	if err != nil {
		return err
	}

	if existing := schemas[name]; existing != nil && existing.Value != nil {
		schemaRef.Value = existing.Value
	} else {
		schemaRef.Value = &canonical
		schemas[name] = &openapi3.SchemaRef{Value: &canonical}
	}
	schemaRef.Ref = "#/components/schemas/" + name
	if nullable {
		*schemaRef = openapi3.SchemaRef{Value: &openapi3.Schema{
			OneOf: openapi3.SchemaRefs{
				{Ref: schemaRef.Ref, Value: schemaRef.Value},
				{Value: &openapi3.Schema{Nullable: true, Enum: []any{nil}}},
			},
		}}
	}
	return nil
}

func fixedSchemaComponentName(typeOf reflect.Type) string {
	for _, component := range extensionAuthoringComponentSchemas {
		if reflect.TypeOf(component.value) == typeOf {
			return component.name
		}
	}
	for _, component := range windowManagerComponentSchemas {
		if reflect.TypeOf(component.value) == typeOf {
			return component.name
		}
	}
	return ""
}

func isComponentSchema(typeOf reflect.Type) bool {
	return isComponentType(typeOf) && typeOf != reflect.TypeFor[binaryResponse]()
}

func equalComponentSchema(left, right *openapi3.Schema) (bool, error) {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false, err
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false, err
	}
	return bytes.Equal(leftJSON, rightJSON), nil
}

func contextualComponentName(
	name string,
	typeOf reflect.Type,
	schema, canonical *openapi3.Schema,
	schemas openapi3.Schemas,
) (string, error) {
	if existing := schemas[name]; existing != nil && existing.Value != nil && existing.Value != schema {
		same, err := equalComponentSchema(existing.Value, canonical)
		if err != nil {
			return "", err
		}
		if !same {
			name = pascalPackageName(typeOf.PkgPath()) + typeOf.Name()
			if qualified := schemas[name]; qualified != nil && qualified.Value != nil {
				equivalent, err := equalComponentSchema(qualified.Value, canonical)
				if err != nil {
					return "", err
				}
				if !equivalent {
					return "", fmt.Errorf("duplicate component name %q has incompatible contextual schemas", name)
				}
			}
		}
	}
	return name, nil
}
