package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func (s *service) validateResponseEntities(
	ctx context.Context,
	workspaceID WorkspaceID,
	loopName string,
	request Request,
	decision string,
	payload json.RawMessage,
) error {
	if s.inputEntities == nil {
		return nil
	}
	schema := responseEntitySchema(request, decision)
	if len(schema) == 0 || len(payload) == 0 {
		return nil
	}
	var schemaValue map[string]any
	if err := json.Unmarshal(schema, &schemaValue); err != nil {
		return fmt.Errorf("decode persisted Loop response schema: %w", err)
	}
	var payloadValue any
	if err := json.Unmarshal(payload, &payloadValue); err != nil {
		return nil
	}
	return s.walkResponseEntities(ctx, workspaceID, loopName, "", schemaValue, payloadValue)
}

func responseEntitySchema(request Request, decision string) json.RawMessage {
	if request.Kind == RequestKindAsk {
		return request.Expect
	}
	switch decision {
	case RequestDecisionEdit:
		return request.EditSchema
	case RequestDecisionRespond:
		return request.RespondSchema
	default:
		return nil
	}
}

func (s *service) walkResponseEntities(
	ctx context.Context,
	workspaceID WorkspaceID,
	loopName string,
	path string,
	schema map[string]any,
	value any,
) error {
	if _, hasEnum := schema[jsonSchemaEnumKey]; !hasEnum {
		if rawKind, exists := schema[jsonSchemaEntityKindKey]; exists {
			kind, ok := rawKind.(string)
			entityKind := dsl.EntityKind(strings.TrimSpace(kind))
			if !ok || !entityKind.Valid() {
				return &InputValidationError{
					Loop: loopName, Field: path, Origin: InputOriginResponse,
					Reason: InputValidationReasonInvalidKindPayload,
					Err:    fmt.Errorf("response entity kind is invalid"),
				}
			}
			input := entityKindInput(entityKind)
			return validateEntityInput(
				ctx, workspaceID, entityKind, path, input, value,
				InputOriginResponse, loopName, s.inputEntities,
			)
		}
	}
	properties, propertiesOK := entitySchemaObject(schema[jsonSchemaPropertiesKey])
	object, objectOK := value.(map[string]any)
	if propertiesOK && objectOK {
		for name, child := range properties {
			childSchema, ok := entitySchemaObject(child)
			childValue, present := object[name]
			if !ok || !present {
				continue
			}
			if err := s.walkResponseEntities(
				ctx, workspaceID, loopName, appendResponsePath(path, name), childSchema, childValue,
			); err != nil {
				return err
			}
		}
		if err := s.walkResponseEntitySchemaMaps(
			ctx, workspaceID, loopName, path, schema, object,
		); err != nil {
			return err
		}
	}
	itemSchema, itemsOK := entitySchemaObject(schema[jsonSchemaItemsKey])
	items, valueIsList := value.([]any)
	if itemsOK && valueIsList {
		for index, item := range items {
			if err := s.walkResponseEntities(
				ctx, workspaceID, loopName,
				appendResponsePath(path, strconv.Itoa(index)), itemSchema, item,
			); err != nil {
				return err
			}
		}
	}
	for _, keyword := range []string{jsonSchemaAllOfKey, jsonSchemaAnyOfKey, jsonSchemaOneOfKey} {
		branches, ok := entitySchemaList(schema[keyword])
		if !ok {
			continue
		}
		for _, branch := range branches {
			branchSchema, ok := entitySchemaObject(branch)
			if !ok || !responseSchemaApplies(branchSchema, value) {
				continue
			}
			if err := s.walkResponseEntities(
				ctx, workspaceID, loopName, path, branchSchema, value,
			); err != nil {
				return err
			}
		}
	}
	if condition, ok := entitySchemaObject(schema["if"]); ok {
		keyword := "else"
		if responseSchemaApplies(condition, value) {
			keyword = "then"
		}
		if branch, branchOK := entitySchemaObject(schema[keyword]); branchOK {
			if err := s.walkResponseEntities(ctx, workspaceID, loopName, path, branch, value); err != nil {
				return err
			}
		}
	}
	if contains, ok := entitySchemaObject(schema["contains"]); ok && valueIsList {
		for index, item := range items {
			if !responseSchemaApplies(contains, item) {
				continue
			}
			if err := s.walkResponseEntities(
				ctx, workspaceID, loopName, appendResponsePath(path, strconv.Itoa(index)), contains, item,
			); err != nil {
				return err
			}
		}
	}
	if names, ok := entitySchemaObject(schema["propertyNames"]); ok && objectOK {
		for name := range object {
			if err := s.walkResponseEntities(
				ctx, workspaceID, loopName, appendResponsePath(path, name), names, name,
			); err != nil {
				return err
			}
		}
	}
	if prefixSchemas, ok := entitySchemaList(schema["prefixItems"]); ok && valueIsList {
		for index, child := range prefixSchemas {
			if index >= len(items) {
				break
			}
			childSchema, schemaOK := entitySchemaObject(child)
			if !schemaOK {
				continue
			}
			if err := s.walkResponseEntities(
				ctx, workspaceID, loopName, appendResponsePath(path, strconv.Itoa(index)), childSchema, items[index],
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *service) walkResponseEntitySchemaMaps(
	ctx context.Context,
	workspaceID WorkspaceID,
	loopName string,
	path string,
	schema map[string]any,
	value map[string]any,
) error {
	for _, keyword := range []string{"dependentSchemas"} {
		children, ok := entitySchemaObject(schema[keyword])
		if !ok {
			continue
		}
		for trigger, child := range children {
			if _, present := value[trigger]; !present {
				continue
			}
			childSchema, schemaOK := entitySchemaObject(child)
			if schemaOK {
				if err := s.walkResponseEntities(ctx, workspaceID, loopName, path, childSchema, value); err != nil {
					return err
				}
			}
		}
	}
	patterns, _ := entitySchemaObject(schema["patternProperties"])
	matched := make(map[string]bool, len(value))
	for pattern, child := range patterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		childSchema, schemaOK := entitySchemaObject(child)
		if !schemaOK {
			continue
		}
		for name, childValue := range value {
			if !compiled.MatchString(name) {
				continue
			}
			matched[name] = true
			if err := s.walkResponseEntities(
				ctx, workspaceID, loopName, appendResponsePath(path, name), childSchema, childValue,
			); err != nil {
				return err
			}
		}
	}
	properties, _ := entitySchemaObject(schema[jsonSchemaPropertiesKey])
	for name, childValue := range value {
		if _, declared := properties[name]; declared || matched[name] {
			continue
		}
		childSchema, ok := entitySchemaObject(schema["additionalProperties"])
		if !ok {
			childSchema, ok = entitySchemaObject(schema["unevaluatedProperties"])
		}
		if !ok {
			continue
		}
		if err := s.walkResponseEntities(
			ctx, workspaceID, loopName, appendResponsePath(path, name), childSchema, childValue,
		); err != nil {
			return err
		}
	}
	return nil
}

func responseSchemaApplies(schema map[string]any, value any) bool {
	raw, err := json.Marshal(value)
	if err != nil {
		return true
	}
	return validateJSONSchema(dsl.Schema(schema), raw) == nil
}

func entityKindInput(kind dsl.EntityKind) dsl.Input {
	if kind == dsl.EntityKindAgent {
		return dsl.Input{Type: dsl.InputTypeAgent}
	}
	return dsl.Input{
		Type: dsl.InputTypeRef,
		Ref:  &dsl.InputRef{Kind: dsl.InputRefKind(kind)},
	}
}

func appendResponsePath(path string, segment string) string {
	if path == "" {
		return segment
	}
	return path + "." + segment
}
