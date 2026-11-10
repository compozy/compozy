package telemetry

import (
	"context"
	"maps"
	"slices"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

// meterProviderOptions configures a MeterProvider for export: the code-built resource, exemplars
// disabled, and one view per registered instrument that keeps only that instrument's attribute keys.
func meterProviderOptions(res *resource.Resource) []sdkmetric.Option {
	views := make([]sdkmetric.View, 0, len(instruments))
	for name, registered := range instruments {
		views = append(views, sdkmetric.NewView(
			sdkmetric.Instrument{Name: name},
			sdkmetric.Stream{
				AttributeFilter: attribute.NewAllowKeysFilter(slices.Collect(maps.Keys(registered.keys))...),
			},
		))
	}
	return []sdkmetric.Option{
		sdkmetric.WithResource(res),
		sdkmetric.WithExemplarFilter(exemplar.AlwaysOffFilter),
		sdkmetric.WithView(views...),
	}
}

// sanitizingMetricExporter is the metric side of the privacy boundary: every collection is rebuilt
// from registered instruments, scopes, and attribute values, without exemplars, before it reaches next.
type sanitizingMetricExporter struct {
	next      sdkmetric.Exporter
	sanitizer *sanitizer
}

var _ sdkmetric.Exporter = (*sanitizingMetricExporter)(nil)

func newSanitizingMetricExporter(next sdkmetric.Exporter, s *sanitizer) *sanitizingMetricExporter {
	return &sanitizingMetricExporter{next: next, sanitizer: s}
}

func (e *sanitizingMetricExporter) Temporality(kind sdkmetric.InstrumentKind) metricdata.Temporality {
	return e.next.Temporality(kind)
}

// Aggregation pins the SDK defaults, so an environment-selected aggregation (such as exponential
// histograms) never produces data the sanitizer does not rebuild.
func (*sanitizingMetricExporter) Aggregation(kind sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.DefaultAggregationSelector(kind)
}

func (e *sanitizingMetricExporter) Export(ctx context.Context, metrics *metricdata.ResourceMetrics) error {
	return e.next.Export(ctx, e.sanitizer.resourceMetrics(metrics))
}

func (e *sanitizingMetricExporter) ForceFlush(ctx context.Context) error {
	return e.next.ForceFlush(ctx)
}

func (e *sanitizingMetricExporter) Shutdown(ctx context.Context) error {
	return e.next.Shutdown(ctx)
}

func (s *sanitizer) resourceMetrics(in *metricdata.ResourceMetrics) *metricdata.ResourceMetrics {
	clean := &metricdata.ResourceMetrics{Resource: s.exportResource()}
	if in == nil {
		return clean
	}
	for _, scopeMetrics := range in.ScopeMetrics {
		metrics := make([]metricdata.Metrics, 0, len(scopeMetrics.Metrics))
		for _, metric := range scopeMetrics.Metrics {
			if rebuilt, ok := s.metric(metric); ok {
				metrics = append(metrics, rebuilt)
			}
		}
		if len(metrics) > 0 {
			clean.ScopeMetrics = append(clean.ScopeMetrics, metricdata.ScopeMetrics{
				Scope:   s.scope(scopeMetrics.Scope),
				Metrics: metrics,
			})
		}
	}
	return clean
}

// metric rebuilds a registered instrument's data of its catalog type (monotonic sum, gauge, or explicit
// histogram); any other instrument, type, or aggregation is dropped.
func (s *sanitizer) metric(in metricdata.Metrics) (metricdata.Metrics, bool) {
	registered, ok := instruments[in.Name]
	if !ok {
		return metricdata.Metrics{}, false
	}
	attrs := func(set attribute.Set) attribute.Set {
		return attribute.NewSet(s.attributes(set.ToSlice(), registered.keys)...)
	}
	var data metricdata.Aggregation
	conforms := false
	switch typed := in.Data.(type) {
	case metricdata.Sum[int64]:
		data, conforms = sanitizeSum(typed, attrs), registered.kind == kindCounter && typed.IsMonotonic
	case metricdata.Sum[float64]:
		data, conforms = sanitizeSum(typed, attrs), registered.kind == kindCounter && typed.IsMonotonic
	case metricdata.Gauge[int64]:
		data, conforms = sanitizeGauge(typed, attrs), registered.kind == kindGauge
	case metricdata.Gauge[float64]:
		data, conforms = sanitizeGauge(typed, attrs), registered.kind == kindGauge
	case metricdata.Histogram[int64]:
		data, conforms = sanitizeHistogram(typed, attrs), registered.kind == kindHistogram
	case metricdata.Histogram[float64]:
		data, conforms = sanitizeHistogram(typed, attrs), registered.kind == kindHistogram
	}
	if !conforms {
		return metricdata.Metrics{}, false
	}
	return metricdata.Metrics{Name: s.finalString(in.Name), Unit: s.finalString(registered.unit), Data: data}, true
}

// Points whose sanitized attribute sets collide (for example two unregistered models, both exported
// as "other") are merged so every exported series carries one point. Exemplars are never copied.

func sanitizeSum[N int64 | float64](in metricdata.Sum[N], attrs func(attribute.Set) attribute.Set) metricdata.Sum[N] {
	clean := metricdata.Sum[N]{Temporality: in.Temporality, IsMonotonic: in.IsMonotonic}
	index := make(map[attribute.Distinct]int, len(in.DataPoints))
	for _, point := range in.DataPoints {
		set := attrs(point.Attributes)
		if at, ok := index[set.Equivalent()]; ok {
			merged := &clean.DataPoints[at]
			merged.Value += point.Value
			merged.StartTime, merged.Time = earliest(merged.StartTime, point.StartTime), latest(merged.Time, point.Time)
			continue
		}
		index[set.Equivalent()] = len(clean.DataPoints)
		clean.DataPoints = append(clean.DataPoints, metricdata.DataPoint[N]{
			Attributes: set, StartTime: point.StartTime, Time: point.Time, Value: point.Value,
		})
	}
	return clean
}

func sanitizeGauge[N int64 | float64](
	in metricdata.Gauge[N],
	attrs func(attribute.Set) attribute.Set,
) metricdata.Gauge[N] {
	var clean metricdata.Gauge[N]
	index := make(map[attribute.Distinct]int, len(in.DataPoints))
	for _, point := range in.DataPoints {
		set := attrs(point.Attributes)
		rebuilt := metricdata.DataPoint[N]{
			Attributes: set,
			StartTime:  point.StartTime,
			Time:       point.Time,
			Value:      point.Value,
		}
		if at, ok := index[set.Equivalent()]; ok {
			if !rebuilt.Time.Before(clean.DataPoints[at].Time) {
				clean.DataPoints[at] = rebuilt
			}
			continue
		}
		index[set.Equivalent()] = len(clean.DataPoints)
		clean.DataPoints = append(clean.DataPoints, rebuilt)
	}
	return clean
}

func sanitizeHistogram[N int64 | float64](
	in metricdata.Histogram[N],
	attrs func(attribute.Set) attribute.Set,
) metricdata.Histogram[N] {
	clean := metricdata.Histogram[N]{Temporality: in.Temporality}
	index := make(map[attribute.Distinct]int, len(in.DataPoints))
	for _, point := range in.DataPoints {
		set := attrs(point.Attributes)
		if at, ok := index[set.Equivalent()]; ok && slices.Equal(clean.DataPoints[at].Bounds, point.Bounds) {
			mergeHistogramPoint(&clean.DataPoints[at], point)
			continue
		}
		index[set.Equivalent()] = len(clean.DataPoints)
		clean.DataPoints = append(clean.DataPoints, metricdata.HistogramDataPoint[N]{
			Attributes:   set,
			StartTime:    point.StartTime,
			Time:         point.Time,
			Count:        point.Count,
			Bounds:       slices.Clone(point.Bounds),
			BucketCounts: slices.Clone(point.BucketCounts),
			Min:          point.Min,
			Max:          point.Max,
			Sum:          point.Sum,
		})
	}
	return clean
}

func mergeHistogramPoint[N int64 | float64](
	dst *metricdata.HistogramDataPoint[N],
	src metricdata.HistogramDataPoint[N],
) {
	dst.StartTime, dst.Time = earliest(dst.StartTime, src.StartTime), latest(dst.Time, src.Time)
	dst.Count += src.Count
	dst.Sum += src.Sum
	for bucket := range min(len(dst.BucketCounts), len(src.BucketCounts)) {
		dst.BucketCounts[bucket] += src.BucketCounts[bucket]
	}
	if value, ok := src.Min.Value(); ok {
		if current, defined := dst.Min.Value(); !defined || value < current {
			dst.Min = src.Min
		}
	}
	if value, ok := src.Max.Value(); ok {
		if current, defined := dst.Max.Value(); !defined || value > current {
			dst.Max = src.Max
		}
	}
}

func earliest(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func latest(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
