package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// usageCurrency is the only currency compozy.cost counts.
const usageCurrency = "USD"

// eventMetrics turns the usage and activity observe accepted into OTel counters. It reads the global meter per
// call, so it follows the provider telemetry installs after the observer is built.
type eventMetrics struct{}

var _ observe.EventMetrics = eventMetrics{}

func (eventMetrics) RecordUsage(ctx context.Context, usage observe.AcceptedUsage) {
	meter := telemetry.Meter(telemetry.ScopeObserve)
	tokens, err := meter.Int64Counter(telemetry.MetricTokens)
	if err == nil && tokens.Enabled(ctx) {
		for _, point := range []struct {
			kind  string
			count int64
		}{
			{telemetry.TokenInput, usage.InputTokens},
			{telemetry.TokenOutput, usage.OutputTokens},
			{telemetry.TokenCacheRead, usage.CacheReadTokens},
			{telemetry.TokenCacheWrite, usage.CacheWriteTokens},
		} {
			if point.count > 0 {
				tokens.Add(ctx, point.count, metric.WithAttributes(
					usageAttributes(usage, telemetry.AttrTokenType.String(point.kind))...,
				))
			}
		}
	}
	if !countsCost(usage) {
		return
	}
	cost, err := meter.Float64Counter(telemetry.MetricCost)
	if err == nil && cost.Enabled(ctx) {
		cost.Add(ctx, usage.CostAmount, metric.WithAttributes(
			usageAttributes(usage, telemetry.AttrCostStatus.String(string(usage.CostStatus)))...,
		))
	}
}

// countsCost admits provider-reported and catalog-estimated USD amounts; plan-included, unknown, and non-USD costs add
// nothing.
func countsCost(usage observe.AcceptedUsage) bool {
	switch usage.CostStatus {
	case modelcatalog.CostStatusActual, modelcatalog.CostStatusEstimated:
		return usage.CostAmount > 0 && strings.EqualFold(strings.TrimSpace(usage.CostCurrency), usageCurrency)
	default:
		return false
	}
}

func usageAttributes(usage observe.AcceptedUsage, kind attribute.KeyValue) []attribute.KeyValue {
	return []attribute.KeyValue{
		telemetry.AttrGenAISystem.String(usage.Provider),
		telemetry.AttrGenAIModel.String(telemetry.ModelLabel(usage.Provider, usage.Model)),
		telemetry.AttrAgent.String(usage.Agent),
		kind,
	}
}

func (eventMetrics) RecordEvent(ctx context.Context, event observe.AcceptedEvent) {
	var (
		name  string
		attrs []attribute.KeyValue
	)
	switch event.Kind {
	case observe.EventToolCallTerminal:
		name = telemetry.MetricToolCalls
		attrs = []attribute.KeyValue{
			telemetry.AttrTool.String(event.ToolID),
			telemetry.AttrOutcome.String(event.Outcome),
		}
	case observe.EventPermission:
		name = telemetry.MetricPermissionRequests
		attrs = []attribute.KeyValue{telemetry.AttrOutcome.String(event.Outcome)}
	case observe.EventAgentError:
		name = telemetry.MetricAgentErrors
		attrs = []attribute.KeyValue{
			telemetry.AttrGenAISystem.String(event.Provider),
			telemetry.AttrErrorType.String(event.ErrorType),
		}
	default:
		return
	}
	counter, err := telemetry.Meter(telemetry.ScopeObserve).Int64Counter(name)
	if err == nil && counter.Enabled(ctx) {
		counter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// activitySource is the observer's cheap activity view: the in-memory session snapshot and daemon uptime.
type activitySource interface {
	ActiveCounts(ctx context.Context) (sessions, agents int, err error)
	Uptime() time.Duration
}

// registerActivityGauges reports compozy.session.active, compozy.agent.active, and compozy.uptime at each collection
// from source, never from the observer's Health view. It registers on the provider installed at that moment, whose
// shutdown ends the collection.
func registerActivityGauges(source activitySource) error {
	meter := telemetry.Meter(telemetry.ScopeObserve)
	sessions, sessionsErr := meter.Int64ObservableGauge(telemetry.MetricSessionActive)
	agents, agentsErr := meter.Int64ObservableGauge(telemetry.MetricAgentActive)
	uptime, uptimeErr := meter.Float64ObservableGauge(telemetry.MetricUptime, metric.WithUnit("s"))
	if err := errors.Join(sessionsErr, agentsErr, uptimeErr); err != nil {
		return fmt.Errorf("daemon: create activity gauges: %w", err)
	}
	_, err := meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		observer.ObserveFloat64(uptime, source.Uptime().Seconds())
		active, distinctAgents, err := source.ActiveCounts(ctx)
		if err != nil {
			return err
		}
		observer.ObserveInt64(sessions, int64(active))
		observer.ObserveInt64(agents, int64(distinctAgents))
		return nil
	}, sessions, agents, uptime)
	if err != nil {
		return fmt.Errorf("daemon: register activity gauges: %w", err)
	}
	return nil
}
