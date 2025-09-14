package attachment

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/tplengine"
)

// ContextNormalizer exposes phase-based normalization entrypoints.
type ContextNormalizer struct {
	Engine *tplengine.TemplateEngine
	CWD    *core.PathCWD
}

// NewContextNormalizer creates a new normalization adapter.
func NewContextNormalizer(engine *tplengine.TemplateEngine, cwd *core.PathCWD) *ContextNormalizer {
	return &ContextNormalizer{Engine: engine, CWD: cwd}
}

// Phase1 performs structural expansion with template deferral.
func (n *ContextNormalizer) Phase1(
	ctx context.Context,
	atts []Attachment,
	templateContext map[string]any,
) ([]Attachment, error) {
	return NormalizePhase1(ctx, n.Engine, n.CWD, atts, templateContext)
}

// Phase2 finalizes any deferred templates and completes expansion.
func (n *ContextNormalizer) Phase2(
	ctx context.Context,
	atts []Attachment,
	templateContext map[string]any,
) ([]Attachment, error) {
	return NormalizePhase2(ctx, n.Engine, n.CWD, atts, templateContext)
}
