package daemon

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	looppkg "github.com/compozy/compozy/internal/loop"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type loopToolSchemaSource struct {
	ctx      context.Context
	registry toolspkg.Registry
	scope    toolspkg.Scope
	once     sync.Once
	schemas  map[toolspkg.ToolID]looppkg.ToolSchemaSnapshot
}

var _ looppkg.ToolSchemaSource = (*loopToolSchemaSource)(nil)

func newLoopToolSchemaSource(ctx context.Context, registry toolspkg.Registry) looppkg.ToolSchemaSource {
	return newScopedLoopToolSchemaSource(ctx, registry, toolspkg.Scope{Operator: true})
}

func newScopedLoopToolSchemaSource(
	ctx context.Context,
	registry toolspkg.Registry,
	scope toolspkg.Scope,
) looppkg.ToolSchemaSource {
	if registry == nil {
		return nil
	}
	if ctx == nil {
		return nil
	}
	return &loopToolSchemaSource{ctx: ctx, registry: registry, scope: scope}
}

func (s *loopToolSchemaSource) Snapshot(toolID string) (looppkg.ToolSchemaSnapshot, bool) {
	if s.registry == nil {
		return looppkg.ToolSchemaSnapshot{}, false
	}
	id := toolspkg.ToolID(strings.TrimSpace(toolID))
	if err := id.Validate(); err != nil {
		return looppkg.ToolSchemaSnapshot{}, false
	}
	s.once.Do(s.load)
	snapshot, ok := s.schemas[id]
	if !ok {
		return looppkg.ToolSchemaSnapshot{}, false
	}
	return looppkg.ToolSchemaSnapshot{
		ToolID:             snapshot.ToolID,
		InputSchema:        cloneLoopSchemaRaw(snapshot.InputSchema),
		OutputSchema:       cloneLoopSchemaRaw(snapshot.OutputSchema),
		InputSchemaDigest:  snapshot.InputSchemaDigest,
		OutputSchemaDigest: snapshot.OutputSchemaDigest,
	}, true
}

func (s *loopToolSchemaSource) load() {
	if err := s.ctx.Err(); err != nil {
		return
	}
	views, err := s.registry.List(s.ctx, s.scope)
	if err != nil {
		return
	}
	s.schemas = make(map[toolspkg.ToolID]looppkg.ToolSchemaSnapshot, len(views))
	for i := range views {
		descriptor := views[i].Descriptor
		s.schemas[descriptor.ID] = looppkg.ToolSchemaSnapshot{
			ToolID:             descriptor.ID.String(),
			InputSchema:        cloneLoopSchemaRaw(descriptor.InputSchema),
			OutputSchema:       cloneLoopSchemaRaw(descriptor.OutputSchema),
			InputSchemaDigest:  strings.TrimSpace(descriptor.InputSchemaDigest),
			OutputSchemaDigest: strings.TrimSpace(descriptor.OutputSchemaDigest),
		}
	}
}

func newLoopCompilerWithSchemaSource(source looppkg.ToolSchemaSource) *looppkg.Compiler {
	if source == nil {
		return looppkg.NewCompiler()
	}
	return looppkg.NewCompiler(looppkg.WithCompilerToolSchemaSource(source))
}

func newLoopCompilerFactory(
	registry toolspkg.Registry,
) func(context.Context, looppkg.WorkspaceID, string) *looppkg.Compiler {
	return func(ctx context.Context, workspaceID looppkg.WorkspaceID, profileID string) *looppkg.Compiler {
		scope := toolspkg.Scope{
			Operator: true, WorkspaceID: strings.TrimSpace(string(workspaceID)),
			ProfileID: strings.TrimSpace(profileID),
		}
		return newLoopCompilerWithSchemaSource(newScopedLoopToolSchemaSource(ctx, registry, scope))
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
