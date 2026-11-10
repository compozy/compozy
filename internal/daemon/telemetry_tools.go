package daemon

import (
	"context"
	"log/slog"

	"github.com/compozy/compozy/internal/telemetry"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"go.opentelemetry.io/otel/trace"
)

// toolCallSpan is tool.execute for one registry call; internal/tools cannot import telemetry, so the composition root
// supplies it.
type toolCallSpan struct {
	span trace.Span
}

func traceToolCall(ctx context.Context, id toolspkg.ToolID) (context.Context, toolspkg.CallSpan) {
	ctx, span := telemetry.Tracer(telemetry.ScopeTools).Start(ctx, telemetry.SpanToolExecute)
	if span.IsRecording() {
		span.SetAttributes(telemetry.AttrTool.String(string(id)))
	}
	return ctx, toolCallSpan{span: span}
}

func (s toolCallSpan) SetRisk(risk toolspkg.RiskClass) {
	if s.span.IsRecording() {
		s.span.SetAttributes(telemetry.AttrToolRisk.String(string(risk)))
	}
}

func (s toolCallSpan) End(err error) {
	telemetry.EndSpan(s.span, err)
}

// nativeToolMembership admits the daemon's native tool ids. Extension and MCP server tool names are user-authored, so
// they export as "other".
func nativeToolMembership(
	ctx context.Context,
	native toolspkg.Provider,
	logger *slog.Logger,
) func(string) bool {
	ids := make(map[string]struct{})
	if native != nil {
		descriptors, err := native.List(ctx, toolspkg.Scope{})
		if err != nil {
			logger.WarnContext(ctx, "daemon: list native tools for telemetry", "error", err)
		}
		for i := range descriptors {
			ids[descriptors[i].ID.String()] = struct{}{}
		}
	}
	return func(id string) bool {
		_, ok := ids[id]
		return ok
	}
}
