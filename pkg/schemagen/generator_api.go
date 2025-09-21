package main

import "context"

func GenerateParserSchemas(ctx context.Context, outDir string) error {
	generator := NewSchemaGenerator()
	return generator.Generate(ctx, outDir)
}
