package daemon

import (
	"context"
	"encoding/json"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type loopToolSchemaSource struct {
	ctx      context.Context
	registry toolspkg.Registry
}

var _ looppkg.ToolSchemaSource = loopToolSchemaSource{}

func newLoopToolSchemaSource(ctx context.Context, registry toolspkg.Registry) looppkg.ToolSchemaSource {
	if registry == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return loopToolSchemaSource{ctx: ctx, registry: registry}
}

func (s loopToolSchemaSource) Snapshot(toolID string) (looppkg.ToolSchemaSnapshot, bool) {
	if s.registry == nil {
		return looppkg.ToolSchemaSnapshot{}, false
	}
	id := toolspkg.ToolID(strings.TrimSpace(toolID))
	if err := id.Validate(); err != nil {
		return looppkg.ToolSchemaSnapshot{}, false
	}
	view, err := s.registry.Get(s.ctx, toolspkg.Scope{Operator: true}, id)
	if err != nil {
		return looppkg.ToolSchemaSnapshot{}, false
	}
	descriptor := view.Descriptor
	return looppkg.ToolSchemaSnapshot{
		ToolID:             descriptor.ID.String(),
		InputSchema:        cloneLoopSchemaRaw(descriptor.InputSchema),
		OutputSchema:       cloneLoopSchemaRaw(descriptor.OutputSchema),
		InputSchemaDigest:  strings.TrimSpace(descriptor.InputSchemaDigest),
		OutputSchemaDigest: strings.TrimSpace(descriptor.OutputSchemaDigest),
	}, true
}

func newLoopCompilerWithSchemaSource(source looppkg.ToolSchemaSource) *looppkg.Compiler {
	if source == nil {
		return looppkg.NewCompiler()
	}
	return looppkg.NewCompiler(looppkg.WithCompilerToolSchemaSource(source))
}

func newLoopCompilerFactory(registry toolspkg.Registry) func(context.Context) *looppkg.Compiler {
	return func(ctx context.Context) *looppkg.Compiler {
		return newLoopCompilerWithSchemaSource(newLoopToolSchemaSource(ctx, registry))
	}
}

func newLoopLinterWithSchemaSource(source looppkg.ToolSchemaSource) looppkg.Linter {
	if source == nil {
		return looppkg.NewLinter()
	}
	return looppkg.NewLinter(looppkg.WithToolSchemaSource(source))
}

func cloneLoopSchemaRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
