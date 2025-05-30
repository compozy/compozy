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

func (w *WithRef) SetRefMetadata(filePath, projectRoot string) {
	w.refMetadata = &WithRefMetadata{
		FilePath:    filePath,
		ProjectRoot: projectRoot,
	}
}

// GetRefMetadata returns the reference metadata
func (w *WithRef) GetRefMetadata() *WithRefMetadata {
	return w.refMetadata
}

// ResolveReferences resolves all fields with is_ref tag in the target struct
func (w *WithRef) ResolveReferences(ctx context.Context, target any, currentDoc any) error {
	if err := w.validateTarget(target); err != nil {
		return err
	}
	targetValue := reflect.ValueOf(target).Elem()
	targetType := targetValue.Type()
	for i := range targetValue.NumField() {
		field := targetValue.Field(i)
		fieldType := targetType.Field(i)
		if !w.isRefField(fieldType) {
			continue
		}
		if err := w.resolveRefField(ctx, field, currentDoc); err != nil {
			return errors.Wrapf(err, "failed to resolve reference field %s", fieldType.Name)
		}
	}
	return nil
}

// ResolveAndMergeReferences resolves all reference fields and merges them into the struct
func (w *WithRef) ResolveAndMergeReferences(ctx context.Context, target any, currentDoc any, mergeMode Mode) error {
	if err := w.validateTarget(target); err != nil {
		return err
	}
	targetValue := reflect.ValueOf(target).Elem()
	targetType := targetValue.Type()
	for i := range targetValue.NumField() {
		field := targetValue.Field(i)
		fieldType := targetType.Field(i)
		if !w.isRefField(fieldType) {
			continue
		}
		if err := w.resolveAndMergeRefField(ctx, field, target, currentDoc); err != nil {
			return errors.Wrapf(err, "failed to resolve and merge reference field %s", fieldType.Name)
		}
	}
	return nil
}

// isRefField checks if a field has the is_ref tag
func (w *WithRef) isRefField(fieldType reflect.StructField) bool {
	return fieldType.Tag.Get("is_ref") == "true"
}

// resolveRefField resolves a single reference field
func (w *WithRef) resolveRefField(ctx context.Context, field reflect.Value, currentDoc any) error {
	if !field.CanSet() {
		return nil
	}
	refValue := field.Interface()
	if refValue == nil {
		return nil
	}
	// Parse the reference from the field value
	ref, err := w.ParseRefFromValue(refValue)
	if err != nil {
		return errors.Wrap(err, "failed to parse reference")
	}
	if ref == nil {
		return nil
	}
	// Resolve the reference
	filePath := w.refMetadata.FilePath
	projectRoot := w.refMetadata.ProjectRoot
	resolvedValue, err := ref.Resolve(ctx, currentDoc, filePath, projectRoot)
	if err != nil {
		return errors.Wrap(err, "failed to resolve reference")
	}
	if err := w.setRefPathWithRef(ref); err != nil {
		return errors.Wrap(err, "failed to set ref path")
	}
	// Set the resolved value back to the field
	if resolvedValue != nil {
		field.Set(reflect.ValueOf(resolvedValue))
	}
	return nil
}

// resolveAndMergeRefField resolves a reference field and merges it into the parent struct
func (w *WithRef) resolveAndMergeRefField(ctx context.Context, field reflect.Value, target any, currentDoc any) error {
	if !field.CanSet() {
		return nil
	}
	refValue := field.Interface()
	if refValue == nil {
		return nil
	}
	// Parse the reference from the field value
	ref, err := w.ParseRefFromValue(refValue)
	if err != nil {
		return errors.Wrap(err, "failed to parse reference")
	}
	if ref == nil {
		return nil
	}
	// Resolve the reference
	filePath := w.refMetadata.FilePath
	projectRoot := w.refMetadata.ProjectRoot
	resolvedValue, err := ref.Resolve(ctx, currentDoc, filePath, projectRoot)
	if err != nil {
		return errors.Wrap(err, "failed to resolve reference")
	}
	if resolvedValue == nil {
		return nil
	}
	// Merge the resolved value into the target struct
	if err := w.setRefPathWithRef(ref); err != nil {
		return errors.Wrap(err, "failed to set ref path")
	}
	resolvedMap, ok := resolvedValue.(map[string]any)
	if !ok {
		return errors.New("resolved reference must be a map/object to merge into struct")
	}
	// Apply the merge with the specified mode
	targetValue := reflect.ValueOf(target).Elem()
	currentMap := w.structToMap(targetValue)
	mergedValue, err := ref.ApplyMergeMode(resolvedMap, currentMap)
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

// ParseRefFromValue parses a reference from any value (string or object)
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

// parseRefFromMap parses a reference from a map representation
func (w *WithRef) parseRefFromMap(m map[string]any) (*Ref, error) {
	ref := &Ref{}
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
	} else {
		ref.Type = TypeProperty // default
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
	} else {
		ref.Mode = ModeMerge // default
	}
	return ref, nil
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

// ResolveRefWithInlineData resolves a ref field and merges it with existing inline data
func (w *WithRef) ResolveRefWithInlineData(ctx context.Context, refField any, inlineData map[string]any, currentDoc any) (map[string]any, error) {
	if refField == nil {
		return inlineData, nil
	}
	// Create a map that contains both the $ref and the existing inline properties
	dataWithRef := make(map[string]any, len(inlineData)+1)
	maps.Copy(dataWithRef, inlineData)
	dataWithRef["$ref"] = refField
	// Use ResolveMapReference to resolve the $ref and merge with inline properties
	return w.ResolveMapReference(ctx, dataWithRef, currentDoc)
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
		!w.isRefField(fieldType) // exclude is_ref fields from map conversion
}

// setStructFields sets field values from merged map back to struct
func (w *WithRef) setStructFields(structValue reflect.Value, mergedMap map[string]any) {
	structType := structValue.Type()
	for i := range structValue.NumField() {
		field := structValue.Field(i)
		fieldType := structType.Field(i)
		if !field.CanSet() || fieldType.Name == "WithRef" || w.isRefField(fieldType) {
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

	// Direct assignment if types match
	if valueReflect.Type().AssignableTo(field.Type()) {
		field.Set(valueReflect)
		return
	}

	// Type conversion if possible
	if valueReflect.Type().ConvertibleTo(field.Type()) {
		field.Set(valueReflect.Convert(field.Type()))
		return
	}

	// Handle map to struct conversion using mapstructure
	if valueMap, ok := value.(map[string]any); ok && field.Kind() == reflect.Struct {
		config := &mapstructure.DecoderConfig{
			Result:           field.Addr().Interface(),
			WeaklyTypedInput: true,
			TagName:          "json",
		}

		decoder, err := mapstructure.NewDecoder(config)
		if err == nil {
			decoder.Decode(valueMap) // Ignore error for graceful fallback
		}
	}
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
	ref, err := parseStringRef(refStr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse $ref")
	}
	inlineData := w.extractInlineData(data)
	filePath := w.refMetadata.FilePath
	projectRoot := w.refMetadata.ProjectRoot
	resolvedValue, err := ref.Resolve(ctx, currentDoc, filePath, projectRoot)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve $ref")
	}
	if err := w.setRefPathWithRef(ref); err != nil {
		return nil, errors.Wrap(err, "failed to set ref path")
	}
	mergedValue, err := ref.ApplyMergeMode(resolvedValue, inlineData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to apply merge mode")
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

func (w *WithRef) setRefPathWithRef(ref *Ref) error {
	if ref.Type == TypeFile {
		// For relative file references, resolve relative to the current file's directory
		// For absolute file references, use as-is
		var refPath string
		var err error
		if filepath.IsAbs(ref.File) {
			refPath, err = filepath.Abs(ref.File)
		} else {
			// Resolve relative to the current file's directory
			currentDir := filepath.Dir(w.refMetadata.FilePath)
			refPath, err = filepath.Abs(filepath.Join(currentDir, ref.File))
		}
		if err != nil {
			return errors.Wrap(err, "failed to get absolute path for ref file")
		}
		w.refMetadata.RefPath = refPath
	}
	return nil
}
