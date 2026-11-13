package otlptest

import (
	"context"
	"maps"
	"testing"

	"go.opentelemetry.io/otel"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

// MetricType is the OTLP data type of an exported metric.
type MetricType string

// Exported metric data types.
const (
	MetricSum       MetricType = "sum"
	MetricGauge     MetricType = "gauge"
	MetricHistogram MetricType = "histogram"
	MetricOther     MetricType = "other"
)

// Metric is one metric of the latest export that carried it; cumulative temporality makes it the running total.
type Metric struct {
	Name      string
	Unit      string
	Scope     string
	Type      MetricType
	Monotonic bool
	Points    []Point
}

// Point is one exported series. Value is the sum or gauge value; histograms set Count and Sum, plus Bounds and
// Exemplars to check the exported shape.
type Point struct {
	Attributes map[string]any
	Value      float64
	Count      uint64
	Sum        float64
	Bounds     []float64
	Exemplars  int
}

// Metrics returns the latest export of every metric received so far, by name.
func (c *Collector) Metrics() map[string]Metric {
	c.mu.Lock()
	defer c.mu.Unlock()
	return maps.Clone(c.metrics)
}

// FlushMetrics collects and exports every metric through the installed meter provider and returns everything
// received.
func (c *Collector) FlushMetrics(t testing.TB) map[string]Metric {
	t.Helper()
	provider, ok := otel.GetMeterProvider().(interface{ ForceFlush(context.Context) error })
	if !ok {
		t.Fatalf("global meter provider %T cannot flush; start export with metrics first", otel.GetMeterProvider())
	}
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()
	if err := provider.ForceFlush(ctx); err != nil {
		t.Fatalf("MeterProvider.ForceFlush() error = %v", err)
	}
	return c.Metrics()
}

func (c *Collector) keepMetrics(export *colmetricpb.ExportMetricsServiceRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.metrics == nil {
		c.metrics = make(map[string]Metric)
	}
	for _, resourceMetrics := range export.GetResourceMetrics() {
		for _, scopeMetrics := range resourceMetrics.GetScopeMetrics() {
			for _, metric := range scopeMetrics.GetMetrics() {
				c.metrics[metric.GetName()] = decodeMetric(scopeMetrics.GetScope().GetName(), metric)
			}
		}
	}
}

func decodeMetric(scope string, metric *metricpb.Metric) Metric {
	decoded := Metric{Name: metric.GetName(), Unit: metric.GetUnit(), Scope: scope, Type: MetricOther}
	switch data := metric.GetData().(type) {
	case *metricpb.Metric_Sum:
		decoded.Type, decoded.Monotonic = MetricSum, data.Sum.GetIsMonotonic()
		decoded.Points = decodeNumberPoints(data.Sum.GetDataPoints())
	case *metricpb.Metric_Gauge:
		decoded.Type = MetricGauge
		decoded.Points = decodeNumberPoints(data.Gauge.GetDataPoints())
	case *metricpb.Metric_Histogram:
		decoded.Type = MetricHistogram
		for _, point := range data.Histogram.GetDataPoints() {
			decoded.Points = append(decoded.Points, Point{
				Attributes: decodeAttributes(point.GetAttributes()),
				Count:      point.GetCount(),
				Sum:        point.GetSum(),
				Bounds:     point.GetExplicitBounds(),
				Exemplars:  len(point.GetExemplars()),
			})
		}
	}
	return decoded
}

func decodeNumberPoints(points []*metricpb.NumberDataPoint) []Point {
	decoded := make([]Point, 0, len(points))
	for _, point := range points {
		value := point.GetAsDouble()
		if _, isInt := point.GetValue().(*metricpb.NumberDataPoint_AsInt); isInt {
			value = float64(point.GetAsInt())
		}
		decoded = append(decoded, Point{
			Attributes: decodeAttributes(point.GetAttributes()),
			Value:      value,
			Exemplars:  len(point.GetExemplars()),
		})
	}
	return decoded
}
