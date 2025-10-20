# Issues for `engine/knowledge/ingest/metrics.go`

## Issue 6 - Review Thread Comment

**File:** `engine/knowledge/ingest/metrics.go:41`
**Date:** 2025-10-20 03:07:20 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Optional: make bucket bounds configurable or at least unit‑explicit.**

If these need environment tuning, source them from config (e.g., cfg.Metrics.IngestLatencyBuckets, cfg.Metrics.BatchSizeBuckets) instead of hardcoding; otherwise consider suffixing names with units (e.g., pipelineLatencyBucketsSeconds) to avoid ambiguity. 

As per coding guidelines.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/knowledge/ingest/metrics.go around lines 38 to 41, the metric bucket
bounds are hardcoded and unit‑ambiguous; either load these arrays from
configuration (e.g., cfg.Metrics.IngestLatencyBuckets and
cfg.Metrics.BatchSizeBuckets) and fall back to current defaults, or rename the
variables to make units explicit (e.g., pipelineLatencyBucketsSeconds and
batchSizeBucketsCount) and update all references; implement the config option by
adding fields to the metrics config struct, reading them at init, validating
non-empty numeric slices, and using them when registering histograms, or if
choosing rename-only, change the identifiers and comments to include units and
run tests to ensure no references break.
```

</details>

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyP5`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyP5
```

---
*Generated from PR review - CodeRabbit AI*


---

## Issue 7 - Review Thread Comment

**File:** `engine/knowledge/ingest/metrics.go:160`
**Date:** 2025-10-20 03:07:20 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Prefer Int64Histogram for batch size (integer metric).**

Counts are discrete; use Int64Histogram to better convey type and avoid float conversions. 


Apply these diffs:

1) Change the instrument variable type:
```diff
 var (
   metricsOnce        sync.Once
   metricsInitErr     error
   pipelineLatency    metric.Float64Histogram
   documentsCounter   metric.Int64Counter
   chunksCounter      metric.Int64Counter
-  batchSizeHistogram metric.Float64Histogram
+  batchSizeHistogram metric.Int64Histogram
   errorsCounter      metric.Int64Counter
 )
```

2) Create an Int64Histogram:
```diff
- batchSizeHistogram, err = meter.Float64Histogram(
+ batchSizeHistogram, err = meter.Int64Histogram(
   monitoringmetrics.MetricNameWithSubsystem("knowledge_ingestion", "batch_size"),
   metric.WithDescription("Documents per ingestion batch"),
   metric.WithUnit("1"),
   metric.WithExplicitBucketBoundaries(batchSizeBuckets...),
 )
```

3) Record using int64:
```diff
 func RecordBatchSize(ctx context.Context, size int) {
   if size <= 0 {
     return
   }
   if err := ensureMetrics(); err != nil || batchSizeHistogram == nil {
     return
   }
-  batchSizeHistogram.Record(ctx, float64(size))
+  batchSizeHistogram.Record(ctx, int64(size))
 }
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
var (
  metricsOnce        sync.Once
  metricsInitErr     error
  pipelineLatency    metric.Float64Histogram
  documentsCounter   metric.Int64Counter
  chunksCounter      metric.Int64Counter
  batchSizeHistogram metric.Int64Histogram
  errorsCounter      metric.Int64Counter
)
```

```suggestion
batchSizeHistogram, err = meter.Int64Histogram(
  monitoringmetrics.MetricNameWithSubsystem("knowledge_ingestion", "batch_size"),
  metric.WithDescription("Documents per ingestion batch"),
  metric.WithUnit("1"),
  metric.WithExplicitBucketBoundaries(batchSizeBuckets...),
)
```

```suggestion
func RecordBatchSize(ctx context.Context, size int) {
  if size <= 0 {
    return
  }
  if err := ensureMetrics(); err != nil || batchSizeHistogram == nil {
    return
  }
  batchSizeHistogram.Record(ctx, int64(size))
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/knowledge/ingest/metrics.go around lines 156 to 160, the batch_size
metric is currently defined as a floating-point histogram; change the instrument
variable type to Int64Histogram, instantiate the metric with
metric.Must(meter).NewInt64Histogram(...) (preserving name, description, unit,
and explicit bucket boundaries), and when recording measurements use int64
values (e.g., int64(len(docs))) with histogram.Record(ctx, int64Value) so the
metric uses integer semantics and avoids float conversions.
```

</details>

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyP6`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyP6
```

---
*Generated from PR review - CodeRabbit AI*
