package ref

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"sync"

	"slices"

	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"golang.org/x/sync/errgroup"
)

const (
	// refKey is the JSON/YAML key for references
	refKey = "$ref"
)

// getCachedPath returns GJSON result for optimal performance with optional path caching
func getCachedPath(jsonBytes []byte, path string) gjson.Result {
	// Try to use cached compiled path if available
	if compiledPath, exists := GetGlobalCache().Get(path); exists {
		// Ensure that compiledPath is a string before using it with gjson.GetBytes
		if pathStr, ok := compiledPath.(string); ok {
			return gjson.GetBytes(jsonBytes, pathStr)
		}
	}

	// Cache the path for future use (GJSON paths don't need compilation but caching helps with repetition)
	// Cost is 1 as the path string is small.
	GetGlobalCache().Set(path, path, 1)

	return gjson.GetBytes(jsonBytes, path)
}

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

	// Use cached GJSON path lookup
	result := getCachedPath(jsonBytes, path)
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
		MaxDepth:        DefaultMaxDepth,
		ResolutionStack: make([]string, 0),
		inStack:         make(map[string]struct{}),
	}
	// Pre-marshal the current document to optimize repeated path lookups
	if jsonBytes, err := json.Marshal(currentDoc); err == nil {
		metadata.CurrentDocJSON = jsonBytes
	}
	return r.resolveWithMetadata(ctx, metadata)
}

// resolveWithMetadata resolves the reference with the given metadata.
func (r *Ref) resolveWithMetadata(ctx context.Context, metadata *DocMetadata) (any, error) {
	if metadata.inStack == nil {
		metadata.inStack = make(map[string]struct{})
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

	// Check for cycles using per-goroutine stack
	if metadata.pushRef(refID) {
		return nil, errors.Errorf("circular reference detected: %s", r.String())
	}
	defer metadata.popRef()

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

	// Ensure the final resolved value has consistent types with JSON/YAML processing
	return normalizeValueRecursive(resolvedValue, true), nil
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

		// Check cache first
		if cached, ok := GetGlobalCache().Get(absoluteFilePath); ok {
			doc := &simpleDocument{data: cached}
			metadata.FilePath = absoluteFilePath
			metadata.CurrentDoc = doc
			metadata.CurrentDocJSON = nil // Reset JSON cache for new document
			return doc, nil
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

		// Check cache first
		if cached, ok := GetGlobalCache().Get(globalPath); ok {
			doc := &simpleDocument{data: cached}
			metadata.FilePath = globalPath
			metadata.CurrentDoc = doc
			metadata.CurrentDocJSON = nil // Reset JSON cache for new document
			return doc, nil
		}

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
		return normalizeValueRecursive(value, true), nil
	}
	refValue, hasRef := valueMap[refKey]
	if !hasRef {
		// No $ref, but still need to recursively resolve any nested references
		resolved, err := r.deepResolveValue(ctx, value, metadata)
		if err != nil {
			return nil, err
		}
		return normalizeValueRecursive(resolved, true), nil
	}

	nestedRef, err := parseRefValue(refValue)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse nested $ref")
	}
	if nestedRef == nil {
		return normalizeValueRecursive(value, true), nil
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
		return normalizeValueRecursive(mergedValue, true), nil
	}

	return normalizeValueRecursive(resolvedValue, true), nil
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

// deepResolveValue recursively resolves all references in a value with optional parallel processing.
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
				// Share the same cycle detection for consistent detection across all resolution paths
				nestedMetadata := &DocMetadata{
					CurrentDoc:      metadata.CurrentDoc,
					CurrentDocJSON:  metadata.CurrentDocJSON,
					FilePath:        metadata.FilePath,
					ProjectRoot:     metadata.ProjectRoot,
					MaxDepth:        metadata.MaxDepth,
					ResolutionStack: metadata.ResolutionStack,
					inStack:         metadata.inStack, // Share the same stack for nested resolution
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
					return normalizeValueRecursive(mergedValue, true), nil
				}

				return normalizeValueRecursive(resolvedValue, true), nil
			}
		}

		// No $ref in this map, resolve values (potentially in parallel)
		return r.resolveMapParallel(ctx, v, metadata)

	case []any:
		// Resolve array elements (potentially in parallel)
		return r.resolveSliceParallel(ctx, v, metadata)

	default:
		// Primitive value, normalize and return
		return normalizeValueRecursive(value, true), nil
	}
}

// resolveMapParallel resolves map values with optional parallel processing.
func (r *Ref) resolveMapParallel(ctx context.Context, v map[string]any, metadata *DocMetadata) (map[string]any, error) {
	result := make(map[string]any, len(v))

	// For small maps or single-core systems (or if map len is less than 4), use sequential processing
	if runtime.NumCPU() <= 1 || len(v) < 4 {
		for key, val := range v {
			resolved, err := r.deepResolveValue(ctx, val, metadata)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to resolve value at key '%s'", key)
			}
			result[key] = resolved
		}
		return result, nil
	}

	// Parallel processing for larger maps
	type mapResult struct {
		key   string
		value any
		err   error
	}

	resultCh := make(chan mapResult, len(v))
	g, gctx := errgroup.WithContext(ctx)
	limit := runtime.NumCPU() * 2
	if limit < 1 {
		limit = 1
	}
	g.SetLimit(limit)

	// Launch goroutines for each key-value pair
	for key, val := range v {
		key, val := key, val // Capture loop variables
		g.Go(func() error {
			// Create independent metadata for each parallel branch to avoid race conditions
			localMetadata := &DocMetadata{
				CurrentDoc:      metadata.CurrentDoc,
				CurrentDocJSON:  metadata.CurrentDocJSON,
				FilePath:        metadata.FilePath,
				ProjectRoot:     metadata.ProjectRoot,
				MaxDepth:        metadata.MaxDepth,
				ResolutionStack: append([]string(nil), metadata.ResolutionStack...),
				inStack:         make(map[string]struct{}),
			}

			// Copy current stack state to new goroutine
			for _, refID := range metadata.ResolutionStack {
				localMetadata.inStack[refID] = struct{}{}
			}

			resolved, err := r.deepResolveValue(gctx, val, localMetadata)
			resultCh <- mapResult{key: key, value: resolved, err: err}
			return err
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}
	close(resultCh)

	// Collect results
	for res := range resultCh {
		if res.err != nil {
			return nil, errors.Wrapf(res.err, "failed to resolve value at key '%s'", res.key)
		}
		result[res.key] = res.value
	}

	return result, nil
}

// resolveSliceParallel resolves slice elements with optional parallel processing.
func (r *Ref) resolveSliceParallel(ctx context.Context, v []any, metadata *DocMetadata) ([]any, error) {
	result := make([]any, len(v))

	// For small slices or single-core systems (or if slice len is less than 4), use sequential processing
	if runtime.NumCPU() <= 1 || len(v) < 4 {
		for i, item := range v {
			resolved, err := r.deepResolveValue(ctx, item, metadata)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to resolve array item at index %d", i)
			}
			result[i] = resolved
		}
		return result, nil
	}

	// Parallel processing for larger slices
	g, gctx := errgroup.WithContext(ctx)
	limit := runtime.NumCPU() * 2
	if limit < 1 {
		limit = 1
	}
	g.SetLimit(limit)

	// Use mutex to protect result slice writes
	var mu sync.Mutex

	// Launch goroutines for each element
	for i, item := range v {
		i, item := i, item // Capture loop variables
		g.Go(func() error {
			// Create independent metadata for each parallel branch to avoid race conditions
			localMetadata := &DocMetadata{
				CurrentDoc:      metadata.CurrentDoc,
				CurrentDocJSON:  metadata.CurrentDocJSON,
				FilePath:        metadata.FilePath,
				ProjectRoot:     metadata.ProjectRoot,
				MaxDepth:        metadata.MaxDepth,
				ResolutionStack: slices.Clone(metadata.ResolutionStack),
				inStack:         make(map[string]struct{}),
			}

			// Copy current stack state to new goroutine
			for _, refID := range metadata.ResolutionStack {
				localMetadata.inStack[refID] = struct{}{}
			}

			resolved, err := r.deepResolveValue(gctx, item, localMetadata)
			if err != nil {
				return errors.Wrapf(err, "failed to resolve array item at index %d", i)
			}

			mu.Lock()
			result[i] = resolved
			mu.Unlock()

			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return result, nil
}

// DocMetadata cycle detection helpers
func (md *DocMetadata) pushRef(refID string) bool {
	if md.inStack == nil {
		md.inStack = make(map[string]struct{})
	}

	// Check if already in current stack (real cycle)
	if _, exists := md.inStack[refID]; exists {
		return true // Cycle detected
	}

	// Add to stack
	md.ResolutionStack = append(md.ResolutionStack, refID)
	md.inStack[refID] = struct{}{}
	return false // No cycle
}

func (md *DocMetadata) popRef() {
	if len(md.ResolutionStack) == 0 {
		return
	}

	// Remove from end of stack
	refID := md.ResolutionStack[len(md.ResolutionStack)-1]
	md.ResolutionStack = md.ResolutionStack[:len(md.ResolutionStack)-1]
	delete(md.inStack, refID)
}
