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
	// First resolve any top-level Ref field
	if err := resolveSchemaRef(
		ctx,
		&s.WithRef,
		s.Ref,
		&s.Schema,
		currentDoc,
		projectRoot,
		filePath,
		"input",
	); err != nil {
		return err
	}
	// Then resolve any nested $ref fields within the schema itself
	if err := s.resolveNestedRefs(ctx, currentDoc, projectRoot, filePath); err != nil {
		return errors.Wrap(err, "failed to resolve nested input schema references")
	}
	return nil
}

type OutputSchema struct {
	ref.WithRef
	Ref    *ref.Node `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Schema Schema    `yaml:",inline"`
}

func (s *OutputSchema) ResolveRef(ctx context.Context, currentDoc any, projectRoot, filePath string) error {
	// First resolve any top-level Ref field
	if err := resolveSchemaRef(
		ctx,
		&s.WithRef,
		s.Ref,
		&s.Schema,
		currentDoc,
		projectRoot,
		filePath,
		"output",
	); err != nil {
		return err
	}
	// Then resolve any nested $ref fields within the schema itself
	if err := s.resolveNestedRefs(ctx, currentDoc, projectRoot, filePath); err != nil {
		return errors.Wrap(err, "failed to resolve nested output schema references")
	}
	return nil
}

// resolveNestedRefs resolves any $ref fields within the schema itself
func (s *InputSchema) resolveNestedRefs(ctx context.Context, currentDoc any, projectRoot, filePath string) error {
	if s.Schema == nil {
		return nil
	}
	// Check if the schema contains $ref fields and resolve them
	if _, hasRef := s.Schema["$ref"]; hasRef {
		// Ensure the WithRef metadata is properly set
		s.WithRef.SetRefMetadata(filePath, projectRoot)
		resolvedMap, err := s.WithRef.ResolveMapReference(ctx, map[string]any(s.Schema), currentDoc)
		if err != nil {
			return err
		}
		s.Schema = Schema(resolvedMap)
	}
	return nil
}

// resolveNestedRefs resolves any $ref fields within the schema itself
func (s *OutputSchema) resolveNestedRefs(ctx context.Context, currentDoc any, projectRoot, filePath string) error {
	if s.Schema == nil {
		return nil
	}
	// Check if the schema contains $ref fields and resolve them
	if _, hasRef := s.Schema["$ref"]; hasRef {
		// Ensure the WithRef metadata is properly set
		s.WithRef.SetRefMetadata(filePath, projectRoot)
		resolvedMap, err := s.WithRef.ResolveMapReference(ctx, map[string]any(s.Schema), currentDoc)
		if err != nil {
			return err
		}
		s.Schema = Schema(resolvedMap)
	}
	return nil
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

// ProcessSchemasFromResolvedData converts resolved raw input/output data to proper schema objects.
// This is used after reference resolution to populate InputSchema/OutputSchema fields from YAML data.
func ProcessSchemasFromResolvedData(resolvedData map[string]any, inputSchema **InputSchema, outputSchema **OutputSchema) {
	// Process input schema
	if inputData, exists := resolvedData["input"]; exists && *inputSchema == nil {
		if inputMap, ok := inputData.(map[string]any); ok {
			*inputSchema = &InputSchema{
				Schema: Schema(inputMap),
			}
		}
	}
	// Process output schema
	if outputData, exists := resolvedData["output"]; exists && *outputSchema == nil {
		if outputMap, ok := outputData.(map[string]any); ok {
			*outputSchema = &OutputSchema{
				Schema: Schema(outputMap),
			}
		}
	}
}

// ResolveAndProcessSchemas handles the complete reference resolution and schema processing pattern.
// This eliminates duplication and ensures efficient single-pass reference resolution.
func ResolveAndProcessSchemas(
	ctx context.Context,
	withRef *ref.WithRef,
	refNode *ref.Node,
	target any,
	currentDoc map[string]any,
	projectRoot, filePath string,
	inputSchema **InputSchema,
	outputSchema **OutputSchema,
) error {
	if refNode == nil || refNode.IsEmpty() {
		return nil
	}
	withRef.SetRefMetadata(filePath, projectRoot)
	// Resolve and merge the reference into the target struct
	if err := withRef.ResolveAndMergeNode(
		ctx,
		refNode,
		target,
		currentDoc,
		ref.ModeMerge,
	); err != nil {
		return err
	}
	// Get the resolved data for schema processing (without re-resolving)
	resolvedData, err := withRef.ResolveRef(ctx, refNode, currentDoc)
	if err != nil {
		return err
	}
	if resolvedMap, ok := resolvedData.(map[string]any); ok {
		ProcessSchemasFromResolvedData(resolvedMap, inputSchema, outputSchema)
	}
	return nil
}
