package ref

import (
	"context"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

type WithRefMetadata struct {
	FilePath    string
	ProjectRoot string
}

// WithRef provides common reference resolution methods that can be embedded
// in other structs to give them reference capabilities.
type WithRef struct {
	refMetadata *WithRefMetadata
}

func (w *WithRef) SetRefMetadata(filePath, projectRoot string) {
	w.refMetadata = &WithRefMetadata{
		FilePath:    filePath,
		ProjectRoot: projectRoot,
	}
}

// ResolveRef resolves a reference node with the given context and returns the result.
func (w *WithRef) ResolveRef(
	ctx context.Context,
	node *Node,
	currentDoc any,
) (any, error) {
	if node.IsEmpty() {
		return nil, nil
	}
	filePath := w.refMetadata.FilePath
	projectRoot := w.refMetadata.ProjectRoot
	return node.Resolve(ctx, currentDoc, filePath, projectRoot)
}

// MergeRefValue merges a reference value with inline values using the specified node.
func (w *WithRef) MergeRefValue(node *Node, refValue, inlineValue any) (any, error) {
	if node.IsEmpty() {
		return inlineValue, nil
	}
	return node.ApplyMergeMode(refValue, inlineValue)
}

// ResolveAndMergeRef resolves a reference and merges it with inline values in one step.
func (w *WithRef) ResolveAndMergeRef(
	ctx context.Context,
	node *Node,
	inlineValue any,
	currentDoc any,
) (any, error) {
	if node.IsEmpty() {
		return inlineValue, nil
	}
	filePath := w.refMetadata.FilePath
	projectRoot := w.refMetadata.ProjectRoot
	resolvedValue, err := node.Resolve(ctx, currentDoc, filePath, projectRoot)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve reference")
	}
	return node.ApplyMergeMode(resolvedValue, inlineValue)
}

// ResolveAndMergeNode resolves a reference node and merges it with inline values, updating the target struct.
func (w *WithRef) ResolveAndMergeNode(
	ctx context.Context,
	node *Node,
	target any,
	currentDoc any,
	mergeMode Mode,
) error {
	if node.IsEmpty() {
		return nil
	}
	if err := w.validateTarget(target); err != nil {
		return err
	}
	resolvedValue, err := w.ResolveRef(ctx, node, currentDoc)
	if err != nil {
		return errors.Wrap(err, "failed to resolve reference")
	}
	if resolvedValue == nil {
		return nil
	}
	resolvedMap, ok := resolvedValue.(map[string]any)
	if !ok {
		return errors.New("resolved reference must be a map/object to merge into struct")
	}
	return w.mergeIntoStruct(target, resolvedMap, node, mergeMode)
}

// ResolveMapReference resolves $ref fields in a map recursively.
func (w *WithRef) ResolveMapReference(
	ctx context.Context,
	data map[string]any,
	currentDoc any,
) (map[string]any, error) {
	if refValue, hasRef := data["$ref"]; hasRef {
		return w.resolveMapWithRef(ctx, data, refValue, currentDoc)
	}
	return w.resolveMapWithoutRef(ctx, data, currentDoc)
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// validateTarget validates that the target is a pointer to struct
func (w *WithRef) validateTarget(target any) error {
	if target == nil {
		return errors.New("target must not be nil")
	}
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr || targetValue.Elem().Kind() != reflect.Struct {
		return errors.New("target must be a pointer to struct")
	}
	return nil
}

// getFieldName extracts the field name from JSON/YAML tags
func (w *WithRef) getFieldName(fieldType reflect.StructField) string {
	fieldName := fieldType.Name
	if jsonTag := fieldType.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
		if commaIdx := strings.Index(jsonTag, ","); commaIdx > 0 {
			fieldName = jsonTag[:commaIdx]
		} else {
			fieldName = jsonTag
		}
	} else if yamlTag := fieldType.Tag.Get("yaml"); yamlTag != "" && yamlTag != "-" {
		if commaIdx := strings.Index(yamlTag, ","); commaIdx > 0 {
			fieldName = yamlTag[:commaIdx]
		} else {
			fieldName = yamlTag
		}
	}
	return fieldName
}

// structToMap converts struct fields to a map, excluding WithRef and unexported fields
func (w *WithRef) structToMap(structValue reflect.Value) map[string]any {
	structType := structValue.Type()
	currentMap := make(map[string]any)
	for i := range structValue.NumField() {
		field := structValue.Field(i)
		fieldType := structType.Field(i)
		if !w.shouldIncludeField(field, fieldType) {
			continue
		}
		fieldName := w.getFieldName(fieldType)
		currentMap[fieldName] = field.Interface()
	}
	return currentMap
}

// shouldIncludeField determines if a field should be included in the map
func (w *WithRef) shouldIncludeField(field reflect.Value, fieldType reflect.StructField) bool {
	return field.CanSet() &&
		fieldType.Name != "WithRef" &&
		field.IsValid() &&
		!field.IsZero()
}

// setStructFields sets field values from merged map back to struct
func (w *WithRef) setStructFields(structValue reflect.Value, mergedMap map[string]any) {
	structType := structValue.Type()
	for i := range structValue.NumField() {
		field := structValue.Field(i)
		fieldType := structType.Field(i)
		if !field.CanSet() || fieldType.Name == "WithRef" {
			continue
		}
		fieldName := w.getFieldName(fieldType)
		if value, exists := mergedMap[fieldName]; exists && value != nil {
			w.setFieldValue(field, value)
		}
	}
}

// setFieldValue sets a single field value with type conversion if needed
func (w *WithRef) setFieldValue(field reflect.Value, value any) {
	valueReflect := reflect.ValueOf(value)
	if valueReflect.Type().AssignableTo(field.Type()) {
		field.Set(valueReflect)
	} else if valueReflect.Type().ConvertibleTo(field.Type()) {
		field.Set(valueReflect.Convert(field.Type()))
	}
}

// mergeIntoStruct handles the complete merge operation into a struct
func (w *WithRef) mergeIntoStruct(target any, resolvedMap map[string]any, node *Node, mergeMode Mode) error {
	targetValue := reflect.ValueOf(target).Elem()
	nodeCopy := w.createNodeWithMergeMode(node, mergeMode)
	currentMap := w.structToMap(targetValue)
	mergedValue, err := nodeCopy.ApplyMergeMode(resolvedMap, currentMap)
	if err != nil {
		return errors.Wrap(err, "failed to apply merge mode")
	}
	mergedMap, ok := mergedValue.(map[string]any)
	if !ok {
		return errors.New("merged value must be a map")
	}
	w.setStructFields(targetValue, mergedMap)
	return nil
}

// createNodeWithMergeMode creates a copy of the node with the specified merge mode
func (w *WithRef) createNodeWithMergeMode(node *Node, mergeMode Mode) *Node {
	nodeCopy := *node
	if nodeCopy.ref != nil {
		refCopy := *nodeCopy.ref
		refCopy.Mode = mergeMode
		nodeCopy.ref = &refCopy
	}
	return &nodeCopy
}

// resolveMapWithRef handles maps that contain $ref
func (w *WithRef) resolveMapWithRef(
	ctx context.Context,
	data map[string]any,
	refValue any,
	currentDoc any,
) (map[string]any, error) {
	refStr, ok := refValue.(string)
	if !ok {
		return nil, errors.New("$ref must be a string")
	}
	node, err := NewNodeFromString(refStr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse $ref")
	}
	inlineData := w.extractInlineData(data)
	mergedValue, err := w.ResolveAndMergeRef(ctx, node, inlineData, currentDoc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve $ref")
	}
	resolvedMap, ok := mergedValue.(map[string]any)
	if !ok {
		return nil, errors.New("$ref did not resolve to a map")
	}
	return resolvedMap, nil
}

// extractInlineData extracts all data except $ref
func (w *WithRef) extractInlineData(data map[string]any) map[string]any {
	inlineData := make(map[string]any, len(data)-1)
	for k, v := range data {
		if k != "$ref" {
			inlineData[k] = v
		}
	}
	return inlineData
}

// resolveMapWithoutRef handles regular maps without $ref
func (w *WithRef) resolveMapWithoutRef(
	ctx context.Context,
	data map[string]any,
	currentDoc any,
) (map[string]any, error) {
	result := make(map[string]any, len(data))
	for key, value := range data {
		resolved, err := w.resolveValue(ctx, value, currentDoc)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve value at key %s", key)
		}
		result[key] = resolved
	}
	return result, nil
}

// resolveValue resolves a single value that could be a map, slice, or primitive
func (w *WithRef) resolveValue(
	ctx context.Context,
	value any,
	currentDoc any,
) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		return w.ResolveMapReference(ctx, v, currentDoc)
	case []any:
		return w.resolveSlice(ctx, v, currentDoc)
	default:
		return value, nil
	}
}

// resolveSlice resolves references in a slice
func (w *WithRef) resolveSlice(
	ctx context.Context,
	slice []any,
	currentDoc any,
) ([]any, error) {
	result := make([]any, len(slice))
	for i, item := range slice {
		resolved, err := w.resolveValue(ctx, item, currentDoc)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve slice item at index %d", i)
		}
		result[i] = resolved
	}
	return result, nil
}
