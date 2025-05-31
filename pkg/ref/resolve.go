package ref

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

const (
	// refKey is the JSON/YAML key for references
	refKey = "$ref"
)

// walkGJSONPath extracts a value from a document using a GJSON path.
func walkGJSONPath(doc any, path string, metadata *DocMetadata) (any, error) {
	if path == "" {
		return doc, nil
	}

	// Ensure we have cached JSON bytes
	if metadata != nil && metadata.CurrentDocJSON == nil {
		jsonBytes, err := json.Marshal(doc)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal document to JSON")
		}
		metadata.CurrentDocJSON = jsonBytes
	}

	var jsonBytes []byte
	if metadata != nil && metadata.CurrentDocJSON != nil {
		jsonBytes = metadata.CurrentDocJSON
	} else {
		var err error
		jsonBytes, err = json.Marshal(doc)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal document to JSON")
		}
	}

	// Use GetBytes directly instead of converting to string
	result := gjson.GetBytes(jsonBytes, path)
	if !result.Exists() {
		return nil, errors.Errorf("path '%s' not found", path)
	}
	return result.Value(), nil
}

// Resolve resolves the reference using the provided context and document metadata.
func (r *Ref) Resolve(ctx context.Context, currentDoc any, filePath, projectRoot string) (any, error) {
	if r == nil {
		return nil, errors.New("cannot resolve nil reference")
	}
	metadata := &DocMetadata{
		CurrentDoc:      &simpleDocument{data: currentDoc},
		FilePath:        filePath,
		ProjectRoot:     projectRoot,
		VisitedRefs:     make(map[string]int),
		MaxDepth:        DefaultMaxDepth,
		ResolutionStack: make([]string, 0),
	}
	// Pre-marshal the current document to optimize repeated path lookups
	if jsonBytes, err := json.Marshal(currentDoc); err == nil {
		metadata.CurrentDocJSON = jsonBytes
	}
	return r.resolveWithMetadata(ctx, metadata)
}

// resolveWithMetadata resolves the reference with the given metadata.
func (r *Ref) resolveWithMetadata(ctx context.Context, metadata *DocMetadata) (any, error) {
	if metadata.VisitedRefs == nil {
		metadata.VisitedRefs = make(map[string]int)
	}
	if metadata.ResolutionStack == nil {
		metadata.ResolutionStack = make([]string, 0)
	}
	if len(metadata.ResolutionStack) >= metadata.MaxDepth {
		return nil, errors.Errorf("max resolution depth exceeded: %d", metadata.MaxDepth)
	}

	// Build a unique identifier for this reference in its context
	refID := r.String()
	if metadata.FilePath != "" {
		refID = metadata.FilePath + "::" + refID
	}

	// Use O(1) map lookup instead of O(N) slice scan for circular reference detection
	if _, visited := metadata.VisitedRefs[refID]; visited {
		return nil, errors.Errorf("circular reference detected: %s", r.String())
	}

	// Mark as visited and add to stack for error reporting
	metadata.VisitedRefs[refID]++
	metadata.ResolutionStack = append(metadata.ResolutionStack, refID)
	defer func() {
		delete(metadata.VisitedRefs, refID)
		metadata.ResolutionStack = metadata.ResolutionStack[:len(metadata.ResolutionStack)-1]
	}()

	// Store the original file path for file references
	originalFilePath := metadata.FilePath

	doc, err := r.selectSourceDocument(ctx, metadata)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to select source document for %s", r.String())
	}

	value, err := doc.Get(r.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path '%s' in %s", r.Path, metadata.FilePath)
	}

	// For file references, we need to use the new file's path as context for nested resolutions
	// This ensures that relative paths in the referenced file are resolved correctly
	var resolvedValue any
	if r.Type == TypeFile {
		// The metadata.FilePath has been updated by selectSourceDocument
		// Recursively resolve all references in the loaded document
		resolvedValue, err = r.deepResolveValue(ctx, value, metadata)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve nested references in %s", r.File)
		}
	} else {
		// For property and global references, maintain the original file context
		resolvedValue, err = r.resolveNestedRef(ctx, value, metadata)
		if err != nil {
			return nil, err
		}
	}

	// Restore original file path after resolution (for property references)
	if r.Type == TypeProperty {
		metadata.FilePath = originalFilePath
	}

	// Ensure the final resolved value has consistent types
	return normalizeValue(resolvedValue), nil
}

// selectSourceDocument selects the source document based on the reference type.
func (r *Ref) selectSourceDocument(ctx context.Context, metadata *DocMetadata) (Document, error) {
	switch r.Type {
	case TypeProperty:
		return metadata.CurrentDoc, nil
	case TypeFile:
		currentDir := filepath.Dir(metadata.FilePath)

		// Resolve the file path relative to the current file's directory
		var absoluteFilePath string
		if filepath.IsAbs(r.File) {
			absoluteFilePath = r.File
		} else {
			absoluteFilePath = filepath.Clean(filepath.Join(currentDir, r.File))
		}

		doc, err := loadDocument(ctx, r.File, currentDir)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load file %s", r.File)
		}

		// Update metadata for the new document context
		// This is crucial: the FilePath must be updated to the newly loaded file
		// so that any relative references within it are resolved correctly
		metadata.FilePath = absoluteFilePath
		metadata.CurrentDoc = doc
		metadata.CurrentDocJSON = nil // Reset JSON cache for new document

		return doc, nil
	case TypeGlobal:
		globalPath := filepath.Join(metadata.ProjectRoot, "compozy.yaml")
		doc, err := loadDocument(ctx, globalPath, metadata.ProjectRoot)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load global config")
		}
		// Update metadata for the global document context
		metadata.FilePath = globalPath
		metadata.CurrentDoc = doc
		metadata.CurrentDocJSON = nil // Reset JSON cache for new document
		return doc, nil
	default:
		return nil, errors.Errorf("unknown ref type: %s", r.Type)
	}
}

// resolveNestedRef resolves nested $ref fields in the resolved value.
func (r *Ref) resolveNestedRef(ctx context.Context, value any, metadata *DocMetadata) (any, error) {
	valueMap, ok := value.(map[string]any)
	if !ok {
		return normalizeValue(value), nil
	}
	refValue, hasRef := valueMap[refKey]
	if !hasRef {
		// No $ref, but still need to recursively resolve any nested references
		resolved, err := r.deepResolveValue(ctx, value, metadata)
		if err != nil {
			return nil, err
		}
		return normalizeValue(resolved), nil
	}

	nestedRef, err := parseRefValue(refValue)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse nested $ref")
	}
	if nestedRef == nil {
		return normalizeValue(value), nil
	}

	// Resolve the nested reference using the current metadata context
	// The metadata already has the correct FilePath from the parent document
	resolvedValue, err := nestedRef.resolveWithMetadata(ctx, metadata)
	if err != nil {
		return nil, err
	}

	// Apply merge mode with inline data
	inlineData := make(map[string]any)
	for k, v := range valueMap {
		if k != refKey {
			inlineData[k] = v
		}
	}

	if len(inlineData) > 0 {
		mergedValue, err := nestedRef.ApplyMergeMode(resolvedValue, inlineData)
		if err != nil {
			return nil, errors.Wrap(err, "failed to apply merge mode")
		}
		return normalizeValue(mergedValue), nil
	}

	return normalizeValue(resolvedValue), nil
}

// normalizeValue ensures consistent types for values when needed for JSON marshaling.
// Only converts integers to float64 if they will be marshaled to JSON.
func normalizeValue(value any) any {
	return normalizeValueRecursive(value, false)
}

// normalizeValueRecursive handles the recursive normalization with an option to force JSON normalization.
func normalizeValueRecursive(value any, forceJSONNormalization bool) any {
	switch v := value.(type) {
	case int, int32, int64:
		return normalizeInteger(v, forceJSONNormalization)
	case map[string]any:
		return normalizeMap(v, forceJSONNormalization)
	case []any:
		return normalizeSlice(v, forceJSONNormalization)
	default:
		return value
	}
}

// normalizeInteger converts integer types to float64 if needed for JSON normalization.
func normalizeInteger(value any, forceJSONNormalization bool) any {
	if !forceJSONNormalization {
		return value
	}
	switch v := value.(type) {
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return value
	}
}

// normalizeMap normalizes a map recursively.
func normalizeMap(v map[string]any, forceJSONNormalization bool) any {
	needsNormalization := forceJSONNormalization
	if !needsNormalization {
		// Quick check to see if we have any integers that might need normalization later
		for _, val := range v {
			if hasIntegers(val) {
				needsNormalization = true
				break
			}
		}
	}

	if needsNormalization {
		result := make(map[string]any, len(v))
		for k, val := range v {
			result[k] = normalizeValueRecursive(val, forceJSONNormalization)
		}
		return result
	}
	return v
}

// normalizeSlice normalizes a slice recursively.
func normalizeSlice(v []any, forceJSONNormalization bool) any {
	needsNormalization := forceJSONNormalization
	if !needsNormalization {
		for _, val := range v {
			if hasIntegers(val) {
				needsNormalization = true
				break
			}
		}
	}

	if needsNormalization {
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = normalizeValueRecursive(val, forceJSONNormalization)
		}
		return result
	}
	return v
}

// hasIntegers quickly checks if a value or its children contain integers.
func hasIntegers(value any) bool {
	switch v := value.(type) {
	case int, int32, int64:
		return true
	case map[string]any:
		for _, val := range v {
			if hasIntegers(val) {
				return true
			}
		}
	case []any:
		for _, val := range v {
			if hasIntegers(val) {
				return true
			}
		}
	}
	return false
}

// deepResolveValue recursively resolves all references in a value (map, slice, or primitive).
func (r *Ref) deepResolveValue(ctx context.Context, value any, metadata *DocMetadata) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		// Check if this map has a $ref
		if refValue, hasRef := v[refKey]; hasRef {
			nestedRef, err := parseRefValue(refValue)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse $ref")
			}
			if nestedRef != nil {
				// IMPORTANT: Create a copy of metadata to preserve the current file context
				// for other references in the same file
				nestedMetadata := &DocMetadata{
					CurrentDoc:      metadata.CurrentDoc,
					CurrentDocJSON:  metadata.CurrentDocJSON,
					FilePath:        metadata.FilePath,
					ProjectRoot:     metadata.ProjectRoot,
					VisitedRefs:     metadata.VisitedRefs,
					MaxDepth:        metadata.MaxDepth,
					ResolutionStack: metadata.ResolutionStack,
				}

				// Resolve the reference
				resolvedValue, err := nestedRef.resolveWithMetadata(ctx, nestedMetadata)
				if err != nil {
					return nil, err
				}

				// Apply merge mode with inline data
				inlineData := make(map[string]any)
				for k, val := range v {
					if k != refKey {
						inlineData[k] = val
					}
				}

				if len(inlineData) > 0 {
					mergedValue, err := nestedRef.ApplyMergeMode(resolvedValue, inlineData)
					if err != nil {
						return nil, errors.Wrap(err, "failed to apply merge mode")
					}
					// Normalize the merged value for consistency
					return normalizeValue(mergedValue), nil
				}

				// Normalize the resolved value for consistency
				return normalizeValue(resolvedValue), nil
			}
		}

		// No $ref in this map, but recursively resolve values
		result := make(map[string]any, len(v))
		for key, val := range v {
			resolved, err := r.deepResolveValue(ctx, val, metadata)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to resolve value at key '%s'", key)
			}
			result[key] = resolved
		}
		return result, nil

	case []any:
		// Recursively resolve array elements
		result := make([]any, len(v))
		for i, item := range v {
			resolved, err := r.deepResolveValue(ctx, item, metadata)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to resolve array item at index %d", i)
			}
			result[i] = resolved
		}
		return result, nil

	default:
		// Primitive value, normalize and return
		return normalizeValue(value), nil
	}
}
