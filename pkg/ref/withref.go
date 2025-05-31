package ref

import (
	"context"
	"maps"
	"path/filepath"
	"reflect"

	"github.com/jinzhu/copier" // Keep for setFieldValue if it remains, or remove if setFieldValue is removed
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

// Deprecated: Use NewResolver(...).Resolve(...) or similar methods on the new Resolver type instead.
// ResolveReferences resolves all fields with is_ref tag in the target struct.
func (w *WithRef) ResolveReferences(ctx context.Context, target any, currentDoc any) error {
	return w.resolveFields(ctx, target, currentDoc, false, ModeMerge)
}

// Deprecated: Use NewResolver(...).Resolve(...) or similar methods on the new Resolver type instead.
// ResolveAndMergeReferences resolves all reference fields and merges them into the struct.
func (w *WithRef) ResolveAndMergeReferences(ctx context.Context, target any, currentDoc any, mergeMode Mode) error {
	return w.resolveFields(ctx, target, currentDoc, true, mergeMode)
}

// Deprecated: Use NewResolver(...).Resolve(...) or similar methods on the new Resolver type instead.
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

// Deprecated: Use NewResolver(...).Resolve(...) or similar methods on the new Resolver type instead.
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

// resolveFields handles field resolution and optional merging using cached field plans.
func (w *WithRef) resolveFields(ctx context.Context, target any, currentDoc any, merge bool, mergeMode Mode) error {
	if err := w.validateTarget(target); err != nil {
		return err
	}

	targetValue := reflect.ValueOf(target).Elem()
	targetType := targetValue.Type()

	// Iterate over fields directly, field plan cache is removed.
	for i := 0; i < targetType.NumField(); i++ {
		fieldStructInfo := targetType.Field(i)
		if !fieldStructInfo.IsExported() || fieldStructInfo.Name == "WithRef" {
			continue
		}

		// Check for `is_ref:"true"` tag
		if fieldStructInfo.Tag.Get("is_ref") != "true" {
			continue
		}

		field := targetValue.Field(i)
		if !field.CanSet() {
			continue
		}

		refFieldValue := field.Interface()
		if refFieldValue == nil {
			continue
		}

		parsedRef, err := w.ParseRefFromValue(refFieldValue)
		if err != nil {
			return errors.Wrapf(err, "failed to parse reference for field %s", fieldStructInfo.Name)
		}
		if parsedRef == nil {
			continue
		}

		resolvedRefValue, err := parsedRef.Resolve(ctx, currentDoc, w.refMetadata.FilePath, w.refMetadata.ProjectRoot)
		if err != nil {
			return errors.Wrapf(err, "failed to resolve reference for field %s (%s)", fieldStructInfo.Name, parsedRef.String())
		}
		if resolvedRefValue == nil {
			// If a ref resolves to nil, we might want to clear the field or leave it as is.
			// For now, if merge is false, we'd attempt to set nil. If merge is true, mergeResolvedValue handles nil resolvedValue.
			if !merge {
				if field.CanSet() { // Ensure it's possible to set (e.g. not an unexported field if logic changes)
					field.Set(reflect.Zero(field.Type())) // Set to zero value if ref resolves to nil
				}
			}
			// Continue to mergeResolvedValue if merge is true, as it might clear fields in target if resolvedMap is empty.
		}

		if err := w.setRefPathWithRef(parsedRef); err != nil { // Assuming parsedRef is the *Ref object
			return errors.Wrapf(err, "failed to set ref path for field %s", fieldStructInfo.Name)
		}

		if merge {
			if err := w.mergeResolvedValue(target, resolvedRefValue, mergeMode); err != nil {
				return errors.Wrapf(err, "failed to merge reference for field %s", fieldStructInfo.Name)
			}
		} else if field.CanSet() && resolvedRefValue != nil {
			// Direct set if not merging, and resolved value is not nil
			// Type compatibility needs to be handled here.
			// reflect.ValueOf(resolvedRefValue)
			newVal := reflect.ValueOf(resolvedRefValue)
			if newVal.IsValid() && newVal.Type().AssignableTo(field.Type()) {
				field.Set(newVal)
			} else if newVal.IsValid() && newVal.Type().ConvertibleTo(field.Type()) {
				field.Set(newVal.Convert(field.Type()))
			} else if newVal.IsValid() {
				// TODO: Consider more sophisticated type conversion here if needed, e.g. map to struct.
				// For now, if not assignable or convertible, it might lead to errors or skipped fields.
				// This was previously handled by setFieldValue, which used mapstructure.
				// If resolvedValue is a map and field is a struct, copier might handle this in merge path,
				// but for direct set, we might need mapstructure or similar.
				// For now, this simplified version might not set fields if types are incompatible beyond assign/convert.
			}
		}
	}
	return nil
}

// processRefFieldWithInfo is removed as its logic is integrated into resolveFields.

// mergeResolvedValue merges the resolved reference value into the target struct.
func (w *WithRef) mergeResolvedValue(target any, resolvedValue any, mergeMode Mode) error {
	resolvedMap, ok := resolvedValue.(map[string]any)
	if !ok {
		// If resolvedValue is not a map, and we are in merge mode,
		// this indicates an issue as merging into a struct typically expects a map.
		// If mergeMode was Replace, it would have been handled by the non-merge path in resolveFields.
		// Thus, for merge, resolvedValue must be a map.
		if resolvedValue == nil { // If the ref resolved to nil, there's nothing to merge.
			return nil
		}
		return errors.New("resolved reference must be a map/object to merge into struct")
	}

	targetValue := reflect.ValueOf(target) // Assume target is Ptr to Struct
	if targetValue.Kind() != reflect.Ptr || targetValue.IsNil() || targetValue.Elem().Kind() != reflect.Struct {
		return errors.New("target for mergeResolvedValue must be a non-nil pointer to a struct")
	}

	// Step 1: Convert target struct to a temporary map (currentMap).
	// copier.Copy copies from source to destination. So, target (struct) is source.
	currentMap := make(map[string]any)
	// By default, copier matches fields by name. This might differ from old logic if json/yaml tags were used.
	if err := copier.Copy(&currentMap, target); err != nil {
		return errors.Wrap(err, "failed to copy target struct to map")
	}

	// Step 2: Merge resolvedMap into currentMap
	tempRef := &Ref{Mode: mergeMode}
	mergedValue, err := tempRef.ApplyMergeMode(resolvedMap, currentMap)
	if err != nil {
		return errors.Wrap(err, "failed to apply merge mode")
	}

	mergedMap, ok := mergedValue.(map[string]any)
	if !ok {
		// This should ideally not happen if ApplyMergeMode returns a map when inputs are maps.
		return errors.New("merged value must be a map to apply back to struct")
	}

	// Step 3: Copy the final merged map back to the target struct.
	// Here, mergedMap is source, target (struct) is destination.
	if err := copier.Copy(target, mergedMap); err != nil {
		return errors.Wrap(err, "failed to copy merged map back to target struct")
	}
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

// structToMapWithPlan is removed.
// setStructFieldsWithPlan is removed.
// setFieldValue is removed as copier.Copy is expected to handle type conversions.
// If specific complex conversions from map values to struct fields are needed beyond what copier offers by default,
// this might need revisiting, potentially by configuring copier options or using mapstructure for the map->struct step.

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
