package schema

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/pkg/ref"
	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// -----------------------------------------------------------------------------
// Schema
// -----------------------------------------------------------------------------

type Schema map[string]any

// resolveSchemaRef is a private function to handle schema reference resolution
func resolveSchemaRef(
	ctx context.Context,
	withRef *ref.WithRef,
	refNode *ref.Node,
	schema *Schema,
	currentDoc any,
	projectRoot, filePath, schemaType string,
) error {
	if refNode != nil && !refNode.IsEmpty() {
		withRef.SetRefMetadata(filePath, projectRoot)
		var schemaMap map[string]any
		if *schema != nil {
			schemaMap = map[string]any(*schema)
		} else {
			schemaMap = make(map[string]any)
		}
		schemaMap["$ref"] = refNode.String()
		resolvedMap, err := withRef.ResolveMapReference(ctx, schemaMap, currentDoc)
		if err != nil {
			return errors.Wrapf(err, "failed to resolve %s schema $ref", schemaType)
		}
		*schema = Schema(resolvedMap)
	}
	return nil
}

type InputSchema struct {
	ref.WithRef
	Ref    *ref.Node `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Schema Schema    `yaml:",inline"`
}

func (s *InputSchema) ResolveRef(ctx context.Context, currentDoc any, projectRoot, filePath string) error {
	return resolveSchemaRef(
		ctx,
		&s.WithRef,
		s.Ref,
		&s.Schema,
		currentDoc,
		projectRoot,
		filePath,
		"input",
	)
}

type OutputSchema struct {
	ref.WithRef
	Ref    *ref.Node `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Schema Schema    `yaml:",inline"`
}

func (s *OutputSchema) ResolveRef(ctx context.Context, currentDoc any, projectRoot, filePath string) error {
	return resolveSchemaRef(
		ctx,
		&s.WithRef,
		s.Ref,
		&s.Schema,
		currentDoc,
		projectRoot,
		filePath,
		"output",
	)
}

func (s *Schema) Validate(value any) error {
	if s == nil {
		return nil
	}

	// Convert schema to JSON for jsonschema
	schemaJSON, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Compile the schema
	schema, err := jsonschema.CompileString("schema.json", string(schemaJSON))
	if err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	// Perform validation
	if err := schema.Validate(value); err != nil {
		return err
	}

	return nil
}

func (s *Schema) GetType() string {
	if typ, ok := (*s)["type"].(string); ok {
		return typ
	}
	return ""
}

func (s *Schema) GetProperties() map[string]*Schema {
	props, ok := (*s)["properties"].(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]*Schema)
	for k, v := range props {
		if schema, ok := v.(map[string]any); ok {
			s := Schema(schema)
			result[k] = &s
		}
	}
	return result
}
