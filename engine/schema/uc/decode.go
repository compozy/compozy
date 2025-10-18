package uc

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/schema"
)

func decodeSchemaBody(ctx context.Context, body map[string]any, pathID string) (*schema.Schema, error) {
	if body == nil {
		return nil, ErrInvalidInput
	}
	s := schema.Schema(body)
	return normalizeSchema(ctx, &s, pathID)
}

func decodeStoredSchema(ctx context.Context, value any, pathID string) (*schema.Schema, error) {
	switch v := value.(type) {
	case *schema.Schema:
		return normalizeSchema(ctx, v, pathID)
	case schema.Schema:
		clone := v
		return normalizeSchema(ctx, &clone, pathID)
	case map[string]any:
		s := schema.Schema(v)
		return normalizeSchema(ctx, &s, pathID)
	default:
		return nil, ErrInvalidInput
	}
}

func normalizeSchema(ctx context.Context, sc *schema.Schema, pathID string) (*schema.Schema, error) {
	if sc == nil {
		return nil, ErrInvalidInput
	}
	sid := strings.TrimSpace(pathID)
	if sid == "" {
		return nil, ErrIDMissing
	}
	if ref, ok := (*sc)["id"]; ok {
		if idStr, ok2 := ref.(string); ok2 {
			trimmed := strings.TrimSpace(idStr)
			if trimmed != "" && trimmed != sid {
				return nil, ErrIDMismatch
			}
		}
	}
	(*sc)["id"] = sid
	if _, err := sc.Compile(ctx); err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return sc, nil
}
