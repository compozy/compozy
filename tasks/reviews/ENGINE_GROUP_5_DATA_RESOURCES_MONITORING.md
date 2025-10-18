# Engine Group 5: Data & Resources - Monitoring Improvements

**Packages:** model, resources, resourceutil, schema, core

---

## Executive Summary

Comprehensive monitoring for data and resource management components.

**Current State:**

- ❌ No schema compile/validate metrics
- ❌ No resource operation metrics
- ❌ No import/export tracking
- ❌ No serialization metrics

---

## Missing Metrics

### 1. Schema Compile and Validate Metrics

**Priority:** 🔴 CRITICAL

**Metrics:**

```yaml
schema_compiles_total:
  type: counter
  labels:
    - cache_hit: bool
  description: "Total schema compilation attempts"

schema_compile_cache_hits_total:
  type: counter
  description: "Schema compilation cache hits"

schema_validations_total:
  type: counter
  labels:
    - outcome: enum[valid, invalid]
  description: "Schema validations performed"

schema_compile_duration_seconds:
  type: histogram
  unit: seconds
  buckets: [0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1]
  description: "Schema compilation duration"

schema_validate_duration_seconds:
  type: histogram
  unit: seconds
  buckets: [0.00001, 0.0001, 0.0005, 0.001, 0.005, 0.01]
  description: "Schema validation duration"

schema_cache_size:
  type: gauge
  description: "Number of compiled schemas in cache"
```

**PromQL Queries:**

```promql
# Cache hit rate
rate(schema_compile_cache_hits_total[5m]) / rate(schema_compiles_total[5m]) * 100

# P95 compilation time
histogram_quantile(0.95, rate(schema_compile_duration_seconds_bucket[5m]))

# Validation throughput
rate(schema_validations_total[5m])

# Invalid schema rate
rate(schema_validations_total{outcome="invalid"}[5m]) / rate(schema_validations_total[5m])
```

**Effort:** M (3h)  
**Risk:** Low

---

### 2. Resource Operation Metrics

**Priority:** 🔴 CRITICAL

**Metrics:**

```yaml
resource_operations_total:
  type: counter
  labels:
    - operation: enum[get, put, delete, list]
    - resource_type: enum[agent, tool, schema, model, memory]
    - outcome: enum[success, error]
  description: "Resource store operations"

resource_operation_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - operation: enum[get, put, delete, list]
    - resource_type: enum[agent, tool, schema, model, memory]
  buckets: [0.0001, 0.001, 0.01, 0.1, 1]
  description: "Resource operation latency"

resource_store_size:
  type: gauge
  labels:
    - resource_type: enum[agent, tool, schema, model, memory]
  description: "Number of resources stored by type"

resource_etag_mismatches_total:
  type: counter
  labels:
    - resource_type: enum[agent, tool, schema, model, memory]
  description: "Optimistic locking conflicts (ETag mismatches)"
```

**PromQL Queries:**

```promql
# Operation latency by type
histogram_quantile(0.95,
  rate(resource_operation_duration_seconds_bucket[5m])) by (operation, resource_type)

# Operations per second
rate(resource_operations_total[5m])

# Error rate
rate(resource_operations_total{outcome="error"}[5m])
  / rate(resource_operations_total[5m])

# ETag conflict rate
rate(resource_etag_mismatches_total[5m])
```

**Effort:** M (4h)  
**Risk:** Low

---

### 3. Resource Import/Export Metrics

**Priority:** 🟡 MEDIUM

**Metrics:**

```yaml
resource_import_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - format: enum[json, yaml]
  buckets: [0.1, 0.5, 1, 5, 10, 30]
  description: "Resource import operation duration"

resource_export_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - format: enum[json, yaml]
  buckets: [0.1, 0.5, 1, 5, 10, 30]
  description: "Resource export operation duration"

resource_import_items_total:
  type: counter
  labels:
    - resource_type: enum[agent, tool, schema, model, memory]
  description: "Number of resources imported"

resource_export_items_total:
  type: counter
  labels:
    - resource_type: enum[agent, tool, schema, model, memory]
  description: "Number of resources exported"

resource_import_errors_total:
  type: counter
  labels:
    - error_type: enum[parse_error, validation_error, duplicate]
  description: "Resource import errors"
```

**PromQL Queries:**

```promql
# Import/export throughput
rate(resource_import_items_total[5m])
rate(resource_export_items_total[5m])

# Import duration
rate(resource_import_duration_seconds_sum[5m])
  / rate(resource_import_duration_seconds_count[5m])

# Import error rate
rate(resource_import_errors_total[5m])
```

**Effort:** S (2h)  
**Risk:** None

---

### 4. Serialization/Deserialization Metrics

**Priority:** 🟢 LOW

**Metrics:**

```yaml
serialization_duration_seconds:
  type: histogram
  unit: seconds
  labels:
    - format: enum[json, yaml]
    - operation: enum[marshal, unmarshal]
  buckets: [0.00001, 0.0001, 0.001, 0.01, 0.1]
  description: "Serialization operation duration"

serialization_bytes:
  type: histogram
  unit: bytes
  labels:
    - format: enum[json, yaml]
  buckets: [100, 1000, 10000, 100000, 1000000]
  description: "Serialized data size"

serialization_errors_total:
  type: counter
  labels:
    - format: enum[json, yaml]
    - operation: enum[marshal, unmarshal]
  description: "Serialization errors"
```

**PromQL Queries:**

```promql
# Serialization throughput
rate(serialization_duration_seconds_count[5m])

# Average serialized size
rate(serialization_bytes_sum[5m]) / rate(serialization_bytes_count[5m])

# Error rate
rate(serialization_errors_total[5m])
```

**Effort:** S (2h)  
**Risk:** None

---

## Implementation Priorities

### Phase 1: Critical Observability (Week 1)

1. Schema metrics (#1) - **3h**
2. Resource operation metrics (#2) - **4h**

### Phase 2: Import/Export Tracking (Week 2)

3. Import/export metrics (#3) - **2h**

### Phase 3: Low-Level Metrics (Week 3)

4. Serialization metrics (#4) - **2h**

**Total effort:** 11 hours

---

## Dashboards

### Schema Performance Dashboard

```yaml
panels:
  - title: Cache Hit Rate
    query: rate(schema_compile_cache_hits_total[5m]) / rate(schema_compiles_total[5m]) * 100

  - title: Validation Latency
    query: histogram_quantile(0.95, rate(schema_validate_duration_seconds_bucket[5m]))

  - title: Validation Throughput
    query: rate(schema_validations_total[5m])
```

### Resource Operations Dashboard

```yaml
panels:
  - title: Operations by Type
    query: sum by (operation, resource_type) (rate(resource_operations_total[5m]))

  - title: Operation Latency
    query: histogram_quantile(0.95, rate(resource_operation_duration_seconds_bucket[5m])) by (operation)

  - title: Store Size
    query: resource_store_size
```

---

## Related Documentation

- **GROUP_5_PERFORMANCE.md** - Data & resource performance optimizations
- **GROUP_4_MONITORING.md** - Infrastructure monitoring patterns
