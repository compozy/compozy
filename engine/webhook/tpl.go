package webhook

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/tplengine"
)

// TemplateRenderer provides shared template engine functionality
// to avoid per-call allocations when rendering webhook templates
type TemplateRenderer struct {
	engine *tplengine.TemplateEngine
}

// NewTemplateRenderer creates a new template renderer with a shared engine
func NewTemplateRenderer() *TemplateRenderer {
	return &TemplateRenderer{
		engine: tplengine.NewEngine(tplengine.FormatJSON),
	}
}

type RenderContext struct {
	Payload map[string]any
}

// RenderTemplate renders webhook templates using the shared template engine
func (r *TemplateRenderer) RenderTemplate(
	ctx context.Context,
	rctx RenderContext,
	input map[string]string,
) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make(map[string]any, len(input))
	for k, tmpl := range input {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		val, err := r.engine.ParseAny(tmpl, map[string]any{"payload": rctx.Payload})
		if err != nil {
			if isMissingKeyErr(err) {
				out[k] = ""
				continue
			}
			return nil, fmt.Errorf("failed to render field %s: %w", k, err)
		}
		out[k] = val
	}
	return out, nil
}

func ValidateTemplate(ctx context.Context, payload map[string]any, s *schema.Schema) error {
	if s == nil {
		return nil
	}
	res, err := s.Validate(ctx, payload)
	if err != nil {
		return err
	}
	_ = res
	return nil
}

func isMissingKeyErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, tplengine.ErrMissingKey) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "map has no entry for key") || strings.Contains(msg, "missing key") ||
		strings.Contains(msg, "missingkey")
}
