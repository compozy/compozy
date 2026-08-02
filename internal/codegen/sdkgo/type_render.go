package sdkgo

import (
	"fmt"
	"reflect"

	"github.com/dave/jennifer/jen"
)

type goField struct {
	name     string
	typeCode *jen.Statement
	jsonTag  string
	comment  string
}

var generatedFieldComments = map[string]map[string]string{
	"LoopGenerationPrePayload": {
		"OriginKind": "OriginKind identifies the actor or task source kind that started the loop.",
		"OriginRef":  "OriginRef identifies the actor or task source reference that started the loop.",
		"Origin":     "Origin is the closed loop-generation provenance value that explains why this generation exists.",
	},
	"LoopGenerationPostPayload": {
		"OriginKind": "OriginKind identifies the actor or task source kind that started the loop.",
		"OriginRef":  "OriginRef identifies the actor or task source reference that started the loop.",
		"Origin":     "Origin is the closed loop-generation provenance value that explains why this generation exists.",
	},
	"LoopGatePrePayload": {
		"Outcome":        "Outcome is the machine result already computed when the hook observes the gate.",
		"Score":          "Score is the computed metric score, when the observed gate has a metric criterion.",
		"BestGeneration": "BestGeneration is the durable best generation known when the hook observes the result.",
	},
	"LoopGatePostPayload": {
		"Outcome":        "Outcome is the machine result already computed when the hook observes the gate.",
		"Score":          "Score is the computed metric score, when the observed gate has a metric criterion.",
		"BestGeneration": "BestGeneration is the durable best generation known when the hook observes the result.",
	},
}

func (g *typeGenerator) renderDeclaration(name string, value reflect.Type) (*jen.Statement, error) {
	var underlying *jen.Statement
	var err error
	if value.Kind() == reflect.Struct {
		underlying, err = g.renderNamedStruct(name, value)
	} else {
		underlying, err = g.goUnderlyingType(value)
	}
	if err != nil {
		return nil, fmt.Errorf("render generated Go type %s: %w", name, err)
	}
	declaration := jen.Type().Id(name).Add(underlying)
	values, isEnum := generatedEnumValues[value]
	if !isEnum {
		return declaration, nil
	}
	if err := assertUniqueIdentifiers(values); err != nil {
		return nil, fmt.Errorf("render generated Go enum %s: %w", name, err)
	}
	constants := make([]jen.Code, 0, len(values))
	for _, enumValue := range values {
		constants = append(constants,
			jen.Id(name+exportIdentifier(enumValue)).Id(name).Op("=").Lit(enumValue),
		)
	}
	return declaration.Line().Const().Defs(constants...), nil
}

func (g *typeGenerator) goUnderlyingType(value reflect.Type) (*jen.Statement, error) {
	switch value.Kind() {
	case reflect.Struct:
		return g.renderStruct(value)
	case reflect.Slice:
		element, err := g.goType(value.Elem())
		return jen.Index().Add(element), err
	case reflect.Array:
		element, err := g.goType(value.Elem())
		return jen.Index(jen.Lit(value.Len())).Add(element), err
	case reflect.Map:
		key, err := g.goType(value.Key())
		if err != nil {
			return nil, err
		}
		element, err := g.goType(value.Elem())
		return jen.Map(key).Add(element), err
	default:
		return primitiveGoType(value.Kind()), nil
	}
}

func (g *typeGenerator) goType(value reflect.Type) (*jen.Statement, error) {
	if value == rawMessageType {
		return jen.Qual("encoding/json", "RawMessage"), nil
	}
	if value == timeType {
		return jen.Qual("time", "Time"), nil
	}
	if value == durationType {
		return jen.Qual("time", "Duration"), nil
	}
	if value.Kind() == reflect.Pointer {
		element, err := g.goType(value.Elem())
		return jen.Op("*").Add(element), err
	}
	switch value.Kind() {
	case reflect.Slice:
		element, err := g.goType(value.Elem())
		return jen.Index().Add(element), err
	case reflect.Array:
		element, err := g.goType(value.Elem())
		return jen.Index(jen.Lit(value.Len())).Add(element), err
	case reflect.Map:
		key, err := g.goType(value.Key())
		if err != nil {
			return nil, err
		}
		element, err := g.goType(value.Elem())
		return jen.Map(key).Add(element), err
	}
	if name, ok := g.preferred[value]; ok {
		return jen.Id(name), nil
	}
	if name := g.ensureInternalNamed(value); name != "" {
		return jen.Id(name), nil
	}

	switch value.Kind() {
	case reflect.Interface:
		return jen.Any(), nil
	case reflect.Struct:
		return g.renderStruct(value)
	default:
		return primitiveGoType(value.Kind()), nil
	}
}

func primitiveGoType(kind reflect.Kind) *jen.Statement {
	switch kind {
	case reflect.Bool:
		return jen.Bool()
	case reflect.Int:
		return jen.Int()
	case reflect.Int8:
		return jen.Int8()
	case reflect.Int16:
		return jen.Int16()
	case reflect.Int32:
		return jen.Int32()
	case reflect.Int64:
		return jen.Int64()
	case reflect.Uint:
		return jen.Uint()
	case reflect.Uint8:
		return jen.Uint8()
	case reflect.Uint16:
		return jen.Uint16()
	case reflect.Uint32:
		return jen.Uint32()
	case reflect.Uint64:
		return jen.Uint64()
	case reflect.Float32:
		return jen.Float32()
	case reflect.Float64:
		return jen.Float64()
	case reflect.String:
		return jen.String()
	default:
		return jen.Any()
	}
}

func (g *typeGenerator) renderStruct(value reflect.Type) (*jen.Statement, error) {
	return g.renderNamedStruct("", value)
}

func (g *typeGenerator) renderNamedStruct(name string, value reflect.Type) (*jen.Statement, error) {
	fields, err := g.structFields(value, name)
	if err != nil {
		return nil, err
	}
	statements := make([]jen.Code, 0, len(fields))
	for _, field := range fields {
		statement := jen.Id(field.name).Add(field.typeCode)
		if field.comment != "" {
			statement = jen.Comment(field.comment).Line().Add(statement)
		}
		if field.jsonTag != "" {
			statement.Tag(map[string]string{"json": field.jsonTag})
		}
		statements = append(statements, statement)
	}
	return jen.Struct(statements...), nil
}

func (g *typeGenerator) structFields(value reflect.Type, rootName string) ([]goField, error) {
	fields := make([]goField, 0, value.NumField())
	for field := range value.Fields() {
		if !field.IsExported() {
			continue
		}
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		if field.Anonymous && jsonTag == "" {
			embedded := declarationType(field.Type)
			if embedded.Kind() == reflect.Struct {
				embeddedFields, err := g.structFields(embedded, rootName)
				if err != nil {
					return nil, err
				}
				fields = append(fields, embeddedFields...)
				continue
			}
		}
		typeCode, err := g.goType(field.Type)
		if err != nil {
			return nil, fmt.Errorf("resolve field %s.%s: %w", value.Name(), field.Name, err)
		}
		comment := ""
		if comments := generatedFieldComments[rootName]; comments != nil {
			comment = comments[field.Name]
		}
		fields = append(fields, goField{
			name: field.Name, typeCode: typeCode, jsonTag: jsonTag, comment: comment,
		})
	}
	return fields, nil
}
