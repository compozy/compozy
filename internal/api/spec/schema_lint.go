package spec

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func lintDocumentSchemas(doc *openapi3.T) error {
	named := make(map[*openapi3.Schema]string)
	for name, ref := range doc.Components.Schemas {
		if ref != nil && ref.Value != nil {
			named[ref.Value] = name
		}
	}
	for name, ref := range doc.Components.Schemas {
		if err := lintSchemaRef(ref, "#/components/schemas/"+schemaPointerEscape(name), named, true); err != nil {
			return err
		}
	}
	for path, item := range doc.Paths.Map() {
		for method, operation := range item.Operations() {
			pointer := "#/paths/" + schemaPointerEscape(path) + "/" + strings.ToLower(method)
			for i, parameter := range operation.Parameters {
				if parameter.Value != nil {
					at := pointer + "/parameters/" + strconv.Itoa(i) + "/schema"
					if err := lintSchemaRef(parameter.Value.Schema, at, named, false); err != nil {
						return err
					}
				}
			}
			if operation.RequestBody != nil && operation.RequestBody.Value != nil {
				content := operation.RequestBody.Value.Content
				if err := lintContentSchemas(content, pointer+"/requestBody/content", named); err != nil {
					return err
				}
			}
			for status, response := range operation.Responses.Map() {
				if response.Value != nil {
					if err := lintContentSchemas(response.Value.Content, pointer+"/responses/"+status+"/content", named); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func lintContentSchemas(content openapi3.Content, pointer string, named map[*openapi3.Schema]string) error {
	for mediaType, media := range content {
		at := pointer + "/" + schemaPointerEscape(mediaType) + "/schema"
		if err := lintSchemaRef(media.Schema, at, named, false); err != nil {
			return err
		}
	}
	return nil
}

func lintSchemaRef(ref *openapi3.SchemaRef, pointer string, named map[*openapi3.Schema]string, component bool) error {
	if ref == nil {
		return nil
	}
	if ref.Ref != "" {
		if ref.Value == nil {
			return fmt.Errorf("%s: unresolved schema reference %q", pointer, ref.Ref)
		}
		return nil
	}
	schema := ref.Value
	if schema == nil {
		return nil
	}
	if err := lintSchemaValue(schema, pointer, named[schema], component); err != nil {
		return err
	}

	for name, child := range schema.Properties {
		if err := lintSchemaRef(child, pointer+"/properties/"+schemaPointerEscape(name), named, false); err != nil {
			return err
		}
	}
	for name, child := range map[string]*openapi3.SchemaRef{
		"items": schema.Items, "additionalProperties": schema.AdditionalProperties.Schema, "not": schema.Not,
	} {
		if err := lintSchemaRef(child, pointer+"/"+name, named, false); err != nil {
			return err
		}
	}
	for name, alternatives := range map[string]openapi3.SchemaRefs{
		"oneOf": schema.OneOf, "anyOf": schema.AnyOf, "allOf": schema.AllOf,
	} {
		for i, child := range alternatives {
			if err := lintSchemaRef(child, pointer+"/"+name+"/"+strconv.Itoa(i), named, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func schemaPointerEscape(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func lintSchemaValue(schema *openapi3.Schema, pointer, name string, component bool) error {
	if schema.Origin != nil && schema.Origin.Key != nil && schema.Origin.Key.Name != "" {
		name = schema.Origin.Key.Name
	}
	if name != "" && !component {
		return fmt.Errorf("%s: named struct %s must use a component reference", pointer, name)
	}
	if schema.Type != nil && len(*schema.Type) > 1 {
		return fmt.Errorf("%s/type: type arrays are invalid in OpenAPI 3.0.3", pointer)
	}
	if schema.Type.Is("integer") {
		for name, bound := range map[string]*float64{"minimum": schema.Min, "maximum": schema.Max} {
			if bound != nil && (*bound < math.MinInt64 || *bound >= math.Exp2(63)) {
				return fmt.Errorf("%s/%s: integer bound is outside int64", pointer, name)
			}
		}
	}
	return nil
}
