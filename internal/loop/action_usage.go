package loop

import "context"

type actionUsageReporterContextKey struct{}

// ActionUsageReporter receives cumulative token usage for one running action task.
type ActionUsageReporter interface {
	ReportActionTokensUsed(tokensUsed int64)
}

type actionSessionReporter interface {
	ReportActionSessionBound(sessionID string)
}

// ActionUsageReporterFunc adapts a function into an ActionUsageReporter.
type ActionUsageReporterFunc func(tokensUsed int64)

// ReportActionTokensUsed calls f with a cumulative token count.
func (f ActionUsageReporterFunc) ReportActionTokensUsed(tokensUsed int64) {
	if f != nil {
		f(tokensUsed)
	}
}

// ContextWithActionUsageReporter attaches a cumulative action usage reporter to ctx.
func ContextWithActionUsageReporter(ctx context.Context, reporter ActionUsageReporter) context.Context {
	if ctx == nil || reporter == nil {
		return ctx
	}
	return context.WithValue(ctx, actionUsageReporterContextKey{}, reporter)
}

func actionUsageReporterFromContext(ctx context.Context) ActionUsageReporter {
	if ctx == nil {
		return nil
	}
	reporter, ok := ctx.Value(actionUsageReporterContextKey{}).(ActionUsageReporter)
	if !ok {
		return nil
	}
	return reporter
}

func reportActionSessionBound(ctx context.Context, sessionID string) {
	reporter := actionUsageReporterFromContext(ctx)
	sessionReporter, ok := reporter.(actionSessionReporter)
	if !ok {
		return
	}
	sessionReporter.ReportActionSessionBound(sessionID)
}
