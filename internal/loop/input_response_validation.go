package loop

import (
	"context"
	"encoding/json"
	"fmt"
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
			if !ok {
				continue
			}
			if err := s.walkResponseEntities(
				ctx, workspaceID, loopName, path, branchSchema, value,
			); err != nil {
				return err
			}
		}
	}
	return nil
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
