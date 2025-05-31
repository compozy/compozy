package ref

import (
	"context"
	"maps"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

// fieldPlan contains pre-computed field information for a struct type.
type fieldPlan struct {
	refFields    []fieldInfo // Fields marked with is_ref:"true"
	normalFields []fieldInfo // Regular fields for map conversion
}

// fieldInfo contains cached information about a struct field.
type fieldInfo struct {
	index int    // Field index in the struct
	name  string // JSON/YAML field name
	isRef bool   // Whether field has is_ref:"true"
}

// fieldPlanCache caches field plans by reflect.Type.
var fieldPlanCache = &sync.Map{}

// getFieldPlan returns a cached field plan for the given struct type.
func getFieldPlan(structType reflect.Type) *fieldPlan {
	if cached, ok := fieldPlanCache.Load(structType); ok {
		plan, ok := cached.(*fieldPlan)
		if !ok {
			// This should never happen, but handle gracefully
			plan = buildFieldPlan(structType)
			fieldPlanCache.Store(structType, plan)
		}
		return plan
	}

	plan := buildFieldPlan(structType)
	fieldPlanCache.Store(structType, plan)
	return plan
}

// buildFieldPlan builds a field plan for the given struct type.
func buildFieldPlan(structType reflect.Type) *fieldPlan {
	plan := &fieldPlan{
		refFields:    make([]fieldInfo, 0),
		normalFields: make([]fieldInfo, 0),
	}

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if !field.IsExported() || field.Name == "WithRef" {
			continue
		}

		info := fieldInfo{
			index: i,
			name:  getFieldNameFromTags(&field),
			isRef: field.Tag.Get("is_ref") == "true",
		}

		if info.isRef {
			plan.refFields = append(plan.refFields, info)
		} else {
			plan.normalFields = append(plan.normalFields, info)
		}
	}

	return plan
}

// getFieldNameFromTags extracts the field name from JSON/YAML tags.
func getFieldNameFromTags(field *reflect.StructField) string {
	// Check JSON tag first
	if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
		if idx := strings.Index(jsonTag, ","); idx > 0 {
			return jsonTag[:idx]
		}
		return jsonTag
	}

	// Check YAML tag
	if yamlTag := field.Tag.Get("yaml"); yamlTag != "" && yamlTag != "-" {
		if idx := strings.Index(yamlTag, ","); idx > 0 {
			return yamlTag[:idx]
		}
		return yamlTag
	}

	return field.Name
}

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
// Deprecated: use Resolver.Resolve instead.
func (w *WithRef) ResolveReferences(ctx context.Context, target any, currentDoc any) error {
	return w.resolveFields(ctx, target, currentDoc, false, ModeMerge)
}

// ResolveAndMergeReferences resolves all reference fields and merges them into the struct.
// Deprecated: use Resolver.Resolve with Resolver.Mode to control merge behaviour.
func (w *WithRef) ResolveAndMergeReferences(ctx context.Context, target any, currentDoc any, mergeMode Mode) error {
	return w.resolveFields(ctx, target, currentDoc, true, mergeMode)
}

// ResolveMapReference resolves $ref fields in a map recursively.
// Deprecated: use Resolver.Resolve instead.
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
// Deprecated: use Resolver.Resolve instead.
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

// resolveFields handles field resolution and optional merging using cached field plans.
func (w *WithRef) resolveFields(ctx context.Context, target any, currentDoc any, merge bool, mergeMode Mode) error {
	if err := w.validateTarget(target); err != nil {
		return err
	}

	targetValue := reflect.ValueOf(target).Elem()
	targetType := targetValue.Type()

	// Get cached field plan
	plan := getFieldPlan(targetType)

	// Process only reference fields
	for _, fieldInfo := range plan.refFields {
		field := targetValue.Field(fieldInfo.index)
		if !field.CanSet() {
			continue
		}

		if err := w.processRefFieldWithInfo(ctx, field, target, currentDoc, merge, mergeMode); err != nil {
			return errors.Wrapf(err, "failed to process reference field %s", fieldInfo.name)
		}
	}

	return nil
}

// processRefFieldWithInfo resolves or merges a single reference field using cached field info.
func (w *WithRef) processRefFieldWithInfo(
	ctx context.Context,
	field reflect.Value,
	target any,
	currentDoc any,
	merge bool,
	mergeMode Mode,
) error {
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
	currentMap := w.structToMapWithPlan(targetValue)
	mergedValue, err := (&Ref{Mode: mergeMode}).ApplyMergeMode(resolvedMap, currentMap)
	if err != nil {
		return errors.Wrap(err, "failed to apply merge mode")
	}

	mergedMap, ok := mergedValue.(map[string]any)
	if !ok {
		return errors.New("merged value must be a map")
	}
	w.setStructFieldsWithPlan(targetValue, mergedMap)
	return nil
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

// structToMapWithPlan converts struct fields to a map using cached field plan.
func (w *WithRef) structToMapWithPlan(structValue reflect.Value) map[string]any {
	plan := getFieldPlan(structValue.Type())
	result := make(map[string]any, len(plan.normalFields))

	for _, fieldInfo := range plan.normalFields {
		field := structValue.Field(fieldInfo.index)
		if field.CanSet() && field.IsValid() {
			result[fieldInfo.name] = field.Interface()
		}
	}

	return result
}

// setStructFieldsWithPlan sets field values from a merged map back to the struct using cached field plan.
func (w *WithRef) setStructFieldsWithPlan(structValue reflect.Value, mergedMap map[string]any) {
	plan := getFieldPlan(structValue.Type())

	for _, fieldInfo := range plan.normalFields {
		field := structValue.Field(fieldInfo.index)
		if !field.CanSet() {
			continue
		}

		if value, exists := mergedMap[fieldInfo.name]; exists && value != nil {
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
