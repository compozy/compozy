package ref

import (
	"context"
	"maps"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

// WithRefMetadata holds metadata for reference resolution.
type WithRefMetadata struct {
	FilePath    string
	RefPath     string
	ProjectRoot string
}

// WithRef provides common reference resolution methods that can be embedded
// in other structs to give them reference capabilities.
type WithRef struct {
	refMetadata *WithRefMetadata
}

// SetRefMetadata sets the reference metadata.
func (w *WithRef) SetRefMetadata(filePath, projectRoot string) {
	w.refMetadata = &WithRefMetadata{
		FilePath:    filePath,
		ProjectRoot: projectRoot,
	}
}

// GetRefMetadata returns the reference metadata.
func (w *WithRef) GetRefMetadata() *WithRefMetadata {
	return w.refMetadata
}

// ResolveReferences resolves all fields with is_ref tag in the target struct.
func (w *WithRef) ResolveReferences(ctx context.Context, target any, currentDoc any) error {
	return w.resolveFields(ctx, target, currentDoc, false, ModeMerge)
}

// ResolveAndMergeReferences resolves all reference fields and merges them into the struct.
func (w *WithRef) ResolveAndMergeReferences(ctx context.Context, target any, currentDoc any, mergeMode Mode) error {
	return w.resolveFields(ctx, target, currentDoc, true, mergeMode)
}

// ResolveMapReference resolves $ref fields in a map recursively.
func (w *WithRef) ResolveMapReference(
	ctx context.Context,
	data map[string]any,
	currentDoc any,
) (map[string]any, error) {
	if refValue, hasRef := data[refKey]; hasRef {
		return w.resolveMapWithRef(ctx, data, refValue, currentDoc)
	}
	return w.resolveMapWithoutRef(ctx, data, currentDoc)
}

// ResolveRefWithInlineData resolves a ref field and merges it with existing inline data.
func (w *WithRef) ResolveRefWithInlineData(
	ctx context.Context,
	refField any,
	inlineData map[string]any,
	currentDoc any,
) (map[string]any, error) {
	if refField == nil {
		return inlineData, nil
	}
	dataWithRef := make(map[string]any, len(inlineData)+1)
	maps.Copy(dataWithRef, inlineData)
	dataWithRef[refKey] = refField
	return w.ResolveMapReference(ctx, dataWithRef, currentDoc)
}

// resolveFields handles field resolution and optional merging for both ResolveReferences and ResolveAndMergeReferences.
func (w *WithRef) resolveFields(ctx context.Context, target any, currentDoc any, merge bool, mergeMode Mode) error {
	if err := w.validateTarget(target); err != nil {
		return err
	}
	targetValue := reflect.ValueOf(target).Elem()
	targetType := targetValue.Type()

	for i := 0; i < targetValue.NumField(); i++ {
		field := targetValue.Field(i)
		fieldType := targetType.Field(i)
		if !w.isRefField(&fieldType) {
			continue
		}
		if err := w.processRefField(ctx, field, target, currentDoc, merge, mergeMode); err != nil {
			return errors.Wrapf(err, "failed to process reference field %s", fieldType.Name)
		}
	}
	return nil
}

// processRefField resolves or merges a single reference field.
func (w *WithRef) processRefField(
	ctx context.Context,
	field reflect.Value,
	target any,
	currentDoc any,
	merge bool,
	mergeMode Mode,
) error {
	if !field.CanSet() {
		return nil
	}
	refValue := field.Interface()
	if refValue == nil {
		return nil
	}

	ref, err := w.ParseRefFromValue(refValue)
	if err != nil {
		return errors.Wrap(err, "failed to parse reference")
	}
	if ref == nil {
		return nil
	}

	resolvedValue, err := ref.Resolve(ctx, currentDoc, w.refMetadata.FilePath, w.refMetadata.ProjectRoot)
	if err != nil {
		return errors.Wrap(err, "failed to resolve reference")
	}
	if resolvedValue == nil {
		return nil
	}

	if err := w.setRefPathWithRef(ref); err != nil {
		return errors.Wrap(err, "failed to set ref path")
	}

	if merge {
		return w.mergeResolvedValue(target, resolvedValue, mergeMode)
	}
	field.Set(reflect.ValueOf(resolvedValue))
	return nil
}

// mergeResolvedValue merges the resolved reference value into the target struct.
func (w *WithRef) mergeResolvedValue(target any, resolvedValue any, mergeMode Mode) error {
	resolvedMap, ok := resolvedValue.(map[string]any)
	if !ok {
		return errors.New("resolved reference must be a map/object to merge into struct")
	}

	targetValue := reflect.ValueOf(target).Elem()
	currentMap := w.structToMap(targetValue)
	mergedValue, err := (&Ref{Mode: mergeMode}).ApplyMergeMode(resolvedMap, currentMap)
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

// isRefField checks if a field has the is_ref tag.
func (w *WithRef) isRefField(fieldType *reflect.StructField) bool {
	return fieldType.Tag.Get("is_ref") == "true"
}

// ParseRefFromValue parses a reference from any value (string or object).
func (w *WithRef) ParseRefFromValue(value any) (*Ref, error) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil, nil
		}
		return parseStringRef(v)
	case map[string]any:
		return w.parseRefFromMap(v)
	default:
		return nil, nil
	}
}

// parseRefFromMap parses a reference from a map representation.
func (w *WithRef) parseRefFromMap(m map[string]any) (*Ref, error) {
	ref := &Ref{Type: TypeProperty, Mode: ModeMerge} // Defaults
	if typeVal, ok := m["type"].(string); ok {
		switch typeVal {
		case "property":
			ref.Type = TypeProperty
		case "file":
			ref.Type = TypeFile
		case "global":
			ref.Type = TypeGlobal
		default:
			return nil, errors.Errorf("unknown reference type: %s", typeVal)
		}
	}
	if path, ok := m["path"].(string); ok {
		ref.Path = path
	}
	if file, ok := m["file"].(string); ok {
		ref.File = file
	}
	if mode, ok := m["mode"].(string); ok {
		switch mode {
		case "merge":
			ref.Mode = ModeMerge
		case "replace":
			ref.Mode = ModeReplace
		case "append":
			ref.Mode = ModeAppend
		default:
			return nil, errors.Errorf("unknown merge mode: %s", mode)
		}
	}
	return ref, nil
}

// validateTarget validates that the target is a pointer to a struct.
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

// getFieldName extracts the field name from JSON/YAML tags.
func (w *WithRef) getFieldName(fieldType *reflect.StructField) string {
	if jsonTag := fieldType.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
		if commaIdx := strings.Index(jsonTag, ","); commaIdx > 0 {
			return jsonTag[:commaIdx]
		}
		return jsonTag
	}
	if yamlTag := fieldType.Tag.Get("yaml"); yamlTag != "" && yamlTag != "-" {
		if commaIdx := strings.Index(yamlTag, ","); commaIdx > 0 {
			return yamlTag[:commaIdx]
		}
		return yamlTag
	}
	return fieldType.Name
}

// structToMap converts struct fields to a map, excluding WithRef and unexported fields.
func (w *WithRef) structToMap(structValue reflect.Value) map[string]any {
	currentMap := make(map[string]any)
	for i := 0; i < structValue.NumField(); i++ {
		field := structValue.Field(i)
		fieldType := structValue.Type().Field(i)
		if !w.shouldIncludeField(field, &fieldType) {
			continue
		}
		currentMap[w.getFieldName(&fieldType)] = field.Interface()
	}
	return currentMap
}

// shouldIncludeField determines if a field should be included in the map.
func (w *WithRef) shouldIncludeField(field reflect.Value, fieldType *reflect.StructField) bool {
	return field.CanSet() && fieldType.Name != "WithRef" && field.IsValid() && !w.isRefField(fieldType)
}

// setStructFields sets field values from a merged map back to the struct.
func (w *WithRef) setStructFields(structValue reflect.Value, mergedMap map[string]any) {
	for i := 0; i < structValue.NumField(); i++ {
		field := structValue.Field(i)
		fieldType := structValue.Type().Field(i)
		if !field.CanSet() || fieldType.Name == "WithRef" || w.isRefField(&fieldType) {
			continue
		}
		if value, exists := mergedMap[w.getFieldName(&fieldType)]; exists && value != nil {
			w.setFieldValue(field, value)
		}
	}
}

// setFieldValue sets a single field value with type conversion if needed.
func (w *WithRef) setFieldValue(field reflect.Value, value any) {
	valueReflect := reflect.ValueOf(value)
	if valueReflect.Type().AssignableTo(field.Type()) {
		field.Set(valueReflect)
		return
	}
	if valueReflect.Type().ConvertibleTo(field.Type()) {
		field.Set(valueReflect.Convert(field.Type()))
		return
	}
	if valueMap, ok := value.(map[string]any); ok && field.Kind() == reflect.Struct {
		config := &mapstructure.DecoderConfig{
			Result:           field.Addr().Interface(),
			WeaklyTypedInput: true,
			TagName:          "json",
		}
		if decoder, err := mapstructure.NewDecoder(config); err == nil {
			if err := decoder.Decode(valueMap); err != nil {
				// Best effort - if we can't decode to struct, skip
				return
			}
		}
	}
}

// resolveMapWithRef handles maps that contain $ref.
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
	ref, err := parseStringRef(refStr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse $ref")
	}

	// Resolve the reference using the Ref's own resolution logic
	resolvedValue, err := ref.Resolve(ctx, currentDoc, w.refMetadata.FilePath, w.refMetadata.ProjectRoot)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve $ref")
	}

	// Apply merge mode with inline data
	// Note: The inline data from the map containing the $ref takes precedence
	mergedValue, err := ref.ApplyMergeMode(resolvedValue, w.extractInlineData(data))
	if err != nil {
		return nil, errors.Wrap(err, "failed to apply merge mode")
	}

	// The resolved value might not be a map (could be array, string, etc)
	// Only process as map if it's actually a map
	if resolvedMap, ok := mergedValue.(map[string]any); ok {
		// File references are already fully resolved by Ref.Resolve
		// Property and global references might still have nested refs to resolve
		if ref.Type == TypeFile {
			// File references are already fully resolved
			return resolvedMap, nil
		}
		// For property and global references, continue resolving nested refs
		return w.resolveMapWithoutRef(ctx, resolvedMap, currentDoc)
	}

	// If it's not a map, return an error as WithRef expects map results
	return nil, errors.New("$ref did not resolve to a map")
}

// extractInlineData extracts all data except $ref.
func (w *WithRef) extractInlineData(data map[string]any) map[string]any {
	inlineData := make(map[string]any, len(data)-1)
	for k, v := range data {
		if k != refKey {
			inlineData[k] = v
		}
	}
	return inlineData
}

// resolveMapWithoutRef handles regular maps without $ref.
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

// resolveValue resolves a single value that could be a map, slice, or primitive.
func (w *WithRef) resolveValue(ctx context.Context, value any, currentDoc any) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		// Create a new WithRef instance with the current metadata context
		// This ensures each recursive call maintains its own file path context
		subResolver := &WithRef{
			refMetadata: &WithRefMetadata{
				FilePath:    w.refMetadata.FilePath,
				RefPath:     w.refMetadata.RefPath,
				ProjectRoot: w.refMetadata.ProjectRoot,
			},
		}
		return subResolver.ResolveMapReference(ctx, v, currentDoc)
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			resolved, err := w.resolveValue(ctx, item, currentDoc)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to resolve slice item at index %d", i)
			}
			result[i] = resolved
		}
		return result, nil
	default:
		return value, nil
	}
}

// setRefPathWithRef sets the RefPath in metadata for file-type references.
func (w *WithRef) setRefPathWithRef(ref *Ref) error {
	if ref.Type != TypeFile {
		return nil
	}
	var refPath string
	if filepath.IsAbs(ref.File) {
		var err error
		refPath, err = filepath.Abs(ref.File)
		if err != nil {
			return errors.Wrap(err, "failed to get absolute path for ref file")
		}
	} else {
		currentDir := filepath.Dir(w.refMetadata.FilePath)
		var err error
		refPath, err = filepath.Abs(filepath.Join(currentDir, ref.File))
		if err != nil {
			return errors.Wrap(err, "failed to get absolute path for ref file")
		}
	}
	w.refMetadata.RefPath = refPath
	return nil
}
