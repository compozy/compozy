package metrics

// WorkflowDurationBuckets defines default latency buckets for workflow duration metrics.
var WorkflowDurationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// HTTPDurationBuckets defines latency buckets for HTTP request duration metrics.
var HTTPDurationBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// HTTPSizeBuckets defines size buckets for HTTP request/response body metrics using integer thresholds.
var HTTPSizeBuckets = []int64{100, 1_000, 10_000, 100_000, 1_000_000, 10_000_000, 100_000_000}

// HTTPSizeBucketBoundaries exposes the histogram boundaries as float64 values for OpenTelemetry configuration.
var HTTPSizeBucketBoundaries = []float64{100, 1_000, 10_000, 100_000, 1_000_000, 10_000_000, 100_000_000}
