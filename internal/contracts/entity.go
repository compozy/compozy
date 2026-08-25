package contracts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidateEntities resolves every x-compozy-kind annotation against the injected catalog.
func ValidateEntities(
	ctx context.Context,
	contract Contract,
	payload json.RawMessage,
	catalog EntityCatalog,
) ([]ValidationIssue, error) {
	if catalog == nil {
		return nil, nil
	}
	var schema map[string]any
	if err := json.Unmarshal(contract.Schema, &schema); err != nil {
		return nil, fmt.Errorf("decode entity contract: %w", err)
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode entity payload: %w", err)
	}
	issues := make([]ValidationIssue, 0, 1)
	if err := walkEntities(ctx, schema, value, "$", catalog, &issues); err != nil {
		return nil, err
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Path < issues[j].Path })
	return issues, nil
}

func walkEntities(
	ctx context.Context,
	schema map[string]any,
	value any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
) error {
	if err := validateEntityAnnotation(ctx, schema, value, path, catalog, issues); err != nil {
		return err
	}
	if object, ok := value.(map[string]any); ok {
		if err := walkObjectEntities(ctx, schema, object, path, catalog, issues); err != nil {
			return err
		}
	}
	if items, ok := value.([]any); ok {
		if err := walkArrayEntities(ctx, schema, items, path, catalog, issues); err != nil {
			return err
		}
	}
	if err := walkCompositionEntities(ctx, schema, value, path, catalog, issues); err != nil {
		return err
	}
	return walkConditionalEntities(ctx, schema, value, path, catalog, issues)
}

func validateEntityAnnotation(
	ctx context.Context,
	schema map[string]any,
	value any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
) error {
	if _, enumerated := schema["enum"]; enumerated {
		return nil
	}
	rawKind, annotated := schema["x-compozy-kind"]
	if !annotated {
		return nil
	}
	kindText, ok := rawKind.(string)
	kind := EntityKind(strings.TrimSpace(kindText))
	if !ok || !validEntityKind(kind) {
		*issues = append(*issues, ValidationIssue{Path: path, Message: "response entity kind is invalid"})
		return nil
	}
	text, ok := value.(string)
	if !ok {
		*issues = append(*issues, ValidationIssue{Path: path, Message: "entity value must be a string"})
		return nil
	}
	exists, err := catalog.EntityExists(ctx, kind, text)
	if err != nil {
		return fmt.Errorf("resolve %s entity %q: %w", kind, text, err)
	}
	if !exists {
		*issues = append(*issues, ValidationIssue{
			Path: path, Message: fmt.Sprintf("%s %q does not exist", kind, text),
		})
	}
	return nil
}

func schemaApplies(schema map[string]any, value any) bool {
	rawSchema, err := json.Marshal(schema)
	if err != nil {
		return true
	}
	schemaValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(rawSchema))
	if err != nil {
		return true
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("entity-branch.json", schemaValue); err != nil {
		return true
	}
	compiled, err := compiler.Compile("entity-branch.json")
	if err != nil {
		return true
	}
	return compiled.Validate(value) == nil
}

func schemaObject(value any) (map[string]any, bool) {
	object, ok := value.(map[string]any)
	return object, ok
}

func schemaList(value any) ([]any, bool) {
	list, ok := value.([]any)
	return list, ok
}

func validEntityKind(kind EntityKind) bool {
	switch kind {
	case EntityAgent, EntitySkill, EntityLoop, EntityWorktree, EntitySession, EntityWorkspace, EntitySecret:
		return true
	default:
		return false
	}
}

func fieldPath(path, field string) string { return path + "." + field }
func indexedPath(path string, index int) string {
	return path + "[" + strconv.Itoa(index) + "]"
}
