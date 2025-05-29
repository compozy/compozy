package ref

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

// walkGJSONPath extracts a value from a document using a GJSON path.
func walkGJSONPath(doc any, path string, metadata *DocMetadata) (any, error) {
	if path == "" {
		return doc, nil
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
		if metadata != nil {
			metadata.CurrentDocJSON = jsonBytes
		}
	}
	result := gjson.Get(string(jsonBytes), path)
	if !result.Exists() {
		return nil, errors.Errorf("path '%s' not found", path)
	}
	return result.Value(), nil
}

// Resolve resolves the reference using the provided context and document metadata.
func (r *Ref) Resolve(ctx context.Context, currentDoc any, currentFilePath, projectRoot string) (any, error) {
	if r == nil {
		return nil, errors.New("cannot resolve nil reference")
	}
	metadata := &DocMetadata{
		CurrentDoc:      &simpleDocument{data: currentDoc},
		FilePath:        currentFilePath,
		ProjectRoot:     projectRoot,
		VisitedRefs:     make(map[string]int),
		MaxDepth:        DefaultMaxDepth,
		ResolutionStack: make([]string, 0),
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
	currentRefString := r.String()
	for _, ref := range metadata.ResolutionStack {
		if ref == currentRefString {
			return nil, errors.Errorf("circular reference detected: %s", currentRefString)
		}
	}
	metadata.ResolutionStack = append(metadata.ResolutionStack, currentRefString)
	defer func() {
		metadata.ResolutionStack = metadata.ResolutionStack[:len(metadata.ResolutionStack)-1]
	}()

	doc, err := r.selectSourceDocument(ctx, metadata)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to select source document for %s", r.String())
	}

	value, err := doc.Get(r.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path '%s' in %s", r.Path, metadata.FilePath)
	}

	return r.resolveNestedRef(ctx, value, doc, metadata)
}

// selectSourceDocument selects the source document based on the reference type.
func (r *Ref) selectSourceDocument(ctx context.Context, metadata *DocMetadata) (Document, error) {
	switch r.Type {
	case TypeProperty:
		return metadata.CurrentDoc, nil
	case TypeFile:
		currentDir := filepath.Dir(metadata.FilePath)
		absoluteFilePath := filepath.Join(currentDir, r.File)
		if !filepath.IsAbs(r.File) {
			absoluteFilePath = filepath.Clean(absoluteFilePath)
		} else {
			absoluteFilePath = r.File
		}
		doc, err := loadDocument(ctx, r.File, currentDir)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load file %s", r.File)
		}
		metadata.FilePath = absoluteFilePath
		return doc, nil
	case TypeGlobal:
		globalPath := filepath.Join(metadata.ProjectRoot, "compozy.yaml")
		doc, err := loadDocument(ctx, globalPath, metadata.ProjectRoot)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load global config")
		}
		metadata.FilePath = globalPath
		return doc, nil
	default:
		return nil, errors.Errorf("unknown ref type: %s", r.Type)
	}
}

// resolveNestedRef resolves nested $ref fields in the resolved value.
func (r *Ref) resolveNestedRef(ctx context.Context, value any, parentDoc Document, metadata *DocMetadata) (any, error) {
	valueMap, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}
	refValue, hasRef := valueMap["$ref"]
	if !hasRef {
		return value, nil
	}
	nestedRef, err := parseRefValue(refValue)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse nested $ref")
	}
	if nestedRef == nil {
		return value, nil
	}
	nestedMetadata := &DocMetadata{
		CurrentDoc:      parentDoc,
		FilePath:        metadata.FilePath,
		ProjectRoot:     metadata.ProjectRoot,
		VisitedRefs:     metadata.VisitedRefs,
		MaxDepth:        metadata.MaxDepth,
		ResolutionStack: metadata.ResolutionStack,
	}
	return nestedRef.resolveWithMetadata(ctx, nestedMetadata)
}
