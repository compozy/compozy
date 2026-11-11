package telemetry

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	"go.opentelemetry.io/otel/metric/noop"
)

// durationOptions gives every duration histogram second-scaled buckets: the SDK defaults (0 … 10000) suit
// milliseconds. The bounds extend otelgin's HTTP buckets to the ten minutes a long turn can take.
var durationOptions = []metric.Float64HistogramOption{
	metric.WithUnit("s"),
	metric.WithExplicitBucketBoundaries(
		0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10, 30, 60, 120, 300, 600,
	),
}

// DurationHistogram returns scope's global-meter histogram for the registered duration instrument name, and false when
// it does not record (export or metrics off), so callers build attributes only for a measurement that is kept.
func DurationHistogram(ctx context.Context, scope, name string) (metric.Float64Histogram, bool) {
	histogram, err := Meter(scope).Float64Histogram(name, durationOptions...)
	if err != nil || !histogram.Enabled(ctx) {
		return nil, false
	}
	return histogram, true
}

// Outcome is the compozy.outcome of work that ended with err: canceled work is OutcomeCancelled, any other error
// OutcomeError, and no error OutcomeOK.
func Outcome(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return OutcomeCancelled
	case err != nil:
		return OutcomeError
	default:
		return OutcomeOK
	}
}

// MeterProvider returns a provider whose float histograms record through the global provider installed at that
// moment, so the HTTP and UDS middleware, built before Init, follows the provider Init installs. Its other
// instruments are no-ops: the middleware's only registered instrument is http.server.request.duration.
func MeterProvider() metric.MeterProvider {
	return globalMeterProvider{}
}

type globalMeterProvider struct{ embedded.MeterProvider }

func (globalMeterProvider) Meter(name string, options ...metric.MeterOption) metric.Meter {
	return globalMeter{name: name, options: options}
}

type globalMeter struct {
	noop.Meter
	name    string
	options []metric.MeterOption
}

func (m globalMeter) Float64Histogram(
	name string,
	options ...metric.Float64HistogramOption,
) (metric.Float64Histogram, error) {
	return globalFloat64Histogram{meter: m, name: name, options: options}, nil
}

type globalFloat64Histogram struct {
	embedded.Float64Histogram
	meter   globalMeter
	name    string
	options []metric.Float64HistogramOption
}

func (h globalFloat64Histogram) resolve() metric.Float64Histogram {
	histogram, err := otel.Meter(h.meter.name, h.meter.options...).Float64Histogram(h.name, h.options...)
	if err != nil {
		return noop.Float64Histogram{}
	}
	return histogram
}

func (h globalFloat64Histogram) Record(ctx context.Context, value float64, options ...metric.RecordOption) {
	h.resolve().Record(ctx, value, options...)
}

func (h globalFloat64Histogram) Enabled(ctx context.Context) bool {
	return h.resolve().Enabled(ctx)
}
