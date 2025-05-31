package ref

import (
	"context"
	"reflect"

	"github.com/dgraph-io/ristretto"
	"github.com/pkg/errors"
	// "encoding/json" // Will be needed for full resolveStruct CurrentDocJSON
)

// Meta contains metadata for reference resolution, typically embedded in structs
// or passed to the Resolve method.
type Meta struct {
	FilePath string // The absolute path to the file containing the struct or data.
	RefPath  string // The path of the reference itself, if applicable (for logging/debugging).
	// ProjectRoot is available from the Resolver.
}

// Resolver is the new central type for resolving references.
// It holds configuration for resolution, such as project root, cache, and merge mode.
type Resolver struct {
	ProjectRoot string
	Cache       *ristretto.Cache
	Mode        Mode // Default: ModeMerge, can be overridden by WithMode option.
}

// NewResolver creates a new Resolver instance.
// ProjectRoot is required as it defines the base for resolving global and sometimes file references.
// By default, it uses the global Ristretto cache (from GetGlobalCache()) and ModeMerge.
// Options can be provided to customize the Resolver's behavior (e.g., WithMode, WithCache).
func NewResolver(projectRoot string, opts ...Option) *Resolver {
	// Initialize with default values.
	resolver := &Resolver{
		ProjectRoot: projectRoot,
		Cache:       GetGlobalCache(), // Assumes GetGlobalCache() is an exported function from load.go
		Mode:        ModeMerge,        // Default merge mode
	}

	// Apply all provided options.
	for _, opt_ := range opts {
		opt_(resolver)
	}

	return resolver
}

// Resolve is the main entry point for resolving references in v.
// v can be a pointer to a struct, a map[string]any, or a slice []any.
// meta is optional and provides file context, primarily for struct resolution.
func (r *Resolver) Resolve(ctx context.Context, v any, meta ...Meta) (any, error) {
	if v == nil {
		return nil, nil
	}

	// Determine the actual value and type, handling pointers
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, nil // Nil pointer needs no resolution
		}
		val = val.Elem() // Dereference the pointer to get the actual value
	}

	// Use provided meta if available, otherwise use a default.
	// The primary meta is for the top-level object 'v'.
	docMeta := DocMetadata{ // This is the existing DocMetadata from resolve.go
		ProjectRoot: r.ProjectRoot,
		MaxDepth:    DefaultMaxDepth, // Use default max depth
		// FilePath will be set from 'meta' if provided, especially for structs
	}
	if len(meta) > 0 {
		docMeta.FilePath = meta[0].FilePath
		// docMeta.RefPath = meta[0].RefPath // If DocMetadata needs this (currently not a field)
	}

	// The core resolution logic will be adapted from resolve.go's deepResolveValue
	// and similar functions. For now, we set up the main switching.

	switch val.Kind() {
	case reflect.Struct:
		// For structs, we need 'v' to be a pointer to modify it.
		if reflect.ValueOf(v).Kind() != reflect.Ptr || reflect.ValueOf(v).IsNil() {
			return v, errors.New("structs must be passed as a non-nil pointer to resolve in-place")
		}
		// Struct resolution modifies the original struct pointed to by v.
		// The returned 'any' will be the same pointer 'v'.
		return r.resolveStruct(ctx, v, docMeta) // v is the original pointer to struct
	case reflect.Map:
		// Ensure it's map[string]any, as that's what our YAML/JSON processing expects
		mapData, ok := val.Interface().(map[string]any)
		if !ok {
			// If it's a map of a different type, we might not be able to resolve it directly
			// or we might need a conversion step. For now, return original if not map[string]any.
			// Consider logging a warning or returning an error based on strictness requirements.
			return v, nil // Or: errors.Errorf("map type %T not supported for direct resolution, expected map[string]any", val.Interface())
		}
		return r.resolveMap(ctx, mapData, docMeta)
	case reflect.Slice:
		sliceData, ok := val.Interface().([]any)
		if !ok {
			// Similar to maps, if it's a slice of a different type.
			// Return original if not []any.
			return v, nil // Or: errors.Errorf("slice type %T not supported for direct resolution, expected []any", val.Interface())
		}
		return r.resolveSlice(ctx, sliceData, docMeta)
	default:
		// Non-composite types are returned as is (or potentially normalized if it were a direct $ref value)
		// Since this is resolving *within* a structure, just return the value.
		// Normalization happens when a $ref resolves to a primitive.
		return val.Interface(), nil
	}
}

// resolveStruct resolves references within a struct.
// v is a pointer to the struct.
// The struct is modified in-place.
func (r *Resolver) resolveStruct(ctx context.Context, v any, docMeta DocMetadata) (any, error) {
	// This will adapt logic from WithRef.resolveFields and parts of resolve.go/Ref.Resolve
	// 1. Get field plan for the struct type. (using getFieldPlan from withref.go perhaps, or reimplement for Resolver)
	// 2. Iterate over fields with `is_ref:"true"`.
	// 3. For each ref field, parse the reference value (string or map using parseRefFromInterface).
	// 4. Create a Ref object.
	// 5. Call the core resolution logic (adapted from Ref.resolveWithMetadata) using r.Cache, r.ProjectRoot.
	//    The `docMeta` passed in will be the starting point. `docMeta.CurrentDoc` needs to be set to the struct itself.
	// 6. The Ref's mode should be influenced by r.Mode (Resolver's mode).
	// 7. Set the resolved value back to the struct field, handling type compatibility.
	// The original struct 'v' is modified in place. The return is typically (v, nil) or (nil, err).

	if reflect.ValueOf(v).Kind() != reflect.Ptr {
		return v, errors.New("resolveStruct expects a pointer to a struct")
	}
	elem := reflect.ValueOf(v).Elem()
	elemType := elem.Type()

	// Get field plan (simplified: direct iteration for now, can optimize with cached plan later)
	for i := 0; i < elem.NumField(); i++ {
		fieldVal := elem.Field(i)
		fieldType := elemType.Field(i)

		// Resolve only exported fields that have the `is_ref:"true"` tag
		if !fieldType.IsExported() {
			continue
		}
		if fieldType.Tag.Get("is_ref") != "true" {
			continue
		}
		if !fieldVal.CanInterface() || !fieldVal.CanSet() {
			continue
		}

		refDef := fieldVal.Interface() // This is the $ref string or map[string]any
		if refDef == nil {
			continue
		}

		parsedRef, err := parseRefFromInterface(refDef)
		if err != nil {
			return v, errors.Wrapf(err, "failed to parse ref for field '%s'", fieldType.Name)
		}
		if parsedRef == nil { // Not a valid ref definition or empty string ref
			continue
		}

		// Let resolver's mode dictate the merge strategy for this reference
		parsedRef.Mode = r.Mode

		// The currentDoc for a struct field resolution is the struct itself.
		// Prepare DocMetadata for resolving this specific field's reference.
		// FilePath and ProjectRoot are inherited from the parent docMeta.
		fieldRefDocMeta := docMeta                 // Create a copy
		fieldRefDocMeta.CurrentDoc = &simpleDocument{data: v} // The struct itself is the context for property refs
		// For CurrentDocJSON, ideally marshal 'v' once if many property refs,
		// but resolveWithMetadata also marshals if CurrentDocJSON is nil.
		// fieldRefDocMeta.CurrentDocJSON = nil // Let resolveWithMetadata handle it

		resolvedField, err := parsedRef.resolveWithMetadata(ctx, &fieldRefDocMeta)
		if err != nil {
			return v, errors.Wrapf(err, "failed to resolve ref for field '%s' (%s)", fieldType.Name, parsedRef.String())
		}

		// Set the resolved value back to the field
		if resolvedField != nil {
			newVal := reflect.ValueOf(resolvedField)
			if newVal.Type().AssignableTo(fieldVal.Type()) {
				fieldVal.Set(newVal)
			} else if newVal.Type().ConvertibleTo(fieldVal.Type()) {
				fieldVal.Set(newVal.Convert(fieldVal.Type()))
			} else {
				// TODO: Handle complex type conversions (e.g., map to struct) using mapstructure if needed.
				// For now, log or return error if not directly assignable/convertible.
				// This might occur if a field is `foo.Bar` but $ref resolves to `map[string]any`.
				// return v, errors.Errorf("resolved type %T for field '%s' is not assignable/convertible to field type %T",
				// 	resolvedField, fieldType.Name, fieldVal.Interface())
			}
		} else {
			// If resolved to nil, set the field to its zero value if it's settable.
			// This handles cases where a $ref resolves to nil and should clear the field.
			if fieldVal.CanSet() {
				fieldVal.Set(reflect.Zero(fieldVal.Type()))
			}
		}
	}
	return v, nil // Return the (potentially modified) original struct pointer
}

// resolveMap resolves references within a map[string]any.
// It handles cases where the map itself is a $ref or contains $refs in its values.
func (r *Resolver) resolveMap(ctx context.Context, m map[string]any, docMeta DocMetadata) (any, error) {
	// If map contains a $ref key, it's a reference itself.
	if refValInterface, isRef := m[refKey]; isRef {
		parsedRef, err := parseRefFromInterface(refValInterface)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse $ref in map")
		}
		if parsedRef == nil { // Invalid $ref definition (e.g. $ref: 123)
			// Treat as a literal map, but what to return? The original map or error?
			// For now, assume if $ref key exists, it must be valid or it's an error.
			// However, parseRefFromInterface returns nil for non-string/map, so this path might be taken.
			// If $ref value is not string/map, it's not a valid ref, so we resolve its values normally.
			// Let's refine: if parseRefFromInterface returns nil AND refValInterface was not nil, it's an error.
			if refValInterface != nil { // e.g. $ref: 123
				return m, errors.Errorf("invalid $ref value type: %T in map, must be string or map", refValInterface)
			}
			// If refValInterface was nil, or parseRefFromInterface returned nil for empty string, it's not a ref. Fall through.
		} else {
			// Valid $ref found at the map's top level.
			parsedRef.Mode = r.Mode // Resolver's mode dictates the merge strategy.

			// Prepare DocMetadata for this $ref resolution.
			// CurrentDoc for a $ref found in a map 'm' is 'm' itself (for potential property refs inside the $ref path).
			refResolutionDocMeta := docMeta
			refResolutionDocMeta.CurrentDoc = &simpleDocument{data: m}
			// refResolutionDocMeta.CurrentDocJSON = nil // Let resolveWithMetadata handle marshaling

			resolvedRefVal, err := parsedRef.resolveWithMetadata(ctx, &refResolutionDocMeta)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to resolve $ref '%s' in map", parsedRef.String())
			}

			// The $ref has been resolved. Now merge it with other inline keys from the original map 'm'.
			inlineData := make(map[string]any)
			for k, v := range m {
				if k != refKey {
					// IMPORTANT: As per standard behavior, inline values are NOT pre-resolved before merging with a top-level $ref.
					// They are taken as-is. If they themselves contain $refs, those $refs will be resolved
					// when the final merged structure is processed in a subsequent pass, or if ApplyMergeMode
					// were to trigger resolution (which it currently does not).
					// For now, we take inline values literally for the merge.
					inlineData[k] = v
				}
			}

			if len(inlineData) > 0 {
				// ApplyMergeMode will use parsedRef.Mode (which is r.Mode)
				return parsedRef.ApplyMergeMode(resolvedRefVal, inlineData)
			}
			return resolvedRefVal, nil
		}
	}

	// No $ref key at this map's top level, or $ref was invalid and ignored.
	// Resolve values recursively.
	resultMap := make(map[string]any, len(m))
	for k, val := range m {
		// For recursive calls, the FilePath from docMeta is passed down.
		// The 'itemMeta' here is for the context of resolving 'val'.
		itemMetaForResolve := Meta{FilePath: docMeta.FilePath}
		resolvedVal, err := r.Resolve(ctx, val, itemMetaForResolve) // Recurse
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve value for key '%s'", k)
		}
		resultMap[k] = resolvedVal
	}
	return resultMap, nil
}

// resolveSlice resolves references within a slice []any.
func (r *Resolver) resolveSlice(ctx context.Context, s []any, docMeta DocMetadata) (any, error) {
	resultSlice := make([]any, len(s))
	for i, item := range s {
		// For recursive calls, the FilePath from docMeta is passed down.
		itemMetaForResolve := Meta{FilePath: docMeta.FilePath}
		resolvedItem, err := r.Resolve(ctx, item, itemMetaForResolve) // Recurse
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve item at slice index %d", i)
		}
		resultSlice[i] = resolvedItem
	}
	return resultSlice, nil
}

// parseRefFromInterface parses a $ref value which can be a string or map[string]any.
// It's a helper for the new Resolver.
func parseRefFromInterface(refDef any) (*Ref, error) {
	switch v := refDef.(type) {
	case string:
		if v == "" { return nil, nil } // Empty string is not a ref, or handled by caller.
		return parseStringRef(v) // Assumes parseStringRef exists (e.g., from parse.go)
	case map[string]any:
		// This logic adapted from WithRef.parseRefFromMap
		ref := &Ref{Type: TypeProperty, Mode: ModeMerge} // Default values

		if typeValStr, ok := v["type"].(string); ok {
			ref.Type = Type(typeValStr) // Directly convert string to Type
			if ref.Type != TypeProperty && ref.Type != TypeFile && ref.Type != TypeGlobal {
				return nil, errors.Errorf("unknown reference type: '%s'", typeValStr)
			}
		}
		if pathStr, ok := v["path"].(string); ok {
			ref.Path = pathStr
		}
		if fileStr, ok := v["file"].(string); ok {
			ref.File = fileStr
		}
		if modeStr, ok := v["mode"].(string); ok {
			ref.Mode = Mode(modeStr) // Directly convert string to Mode
			if ref.Mode != ModeMerge && ref.Mode != ModeReplace && ref.Mode != ModeAppend && ref.Mode != "" { // "" mode defaults to Merge
				return nil, errors.Errorf("unknown merge mode: '%s'", modeStr)
			}
			if ref.Mode == "" { // Explicitly set default if mode was an empty string
			    ref.Mode = ModeMerge
			}
		}
		// Basic validation for file type having a file path
		if ref.Type == TypeFile && ref.File == "" {
			return nil, errors.New("file reference type must include a 'file' path")
		}
		return ref, nil
	default:
		// Not a string or map, so not a valid $ref definition that we can parse here.
		return nil, nil // Or: return nil, errors.Errorf("invalid $ref definition type: %T, must be string or map[string]any", refDef)
	}
}
