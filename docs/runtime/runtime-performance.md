# Runtime Performance Tuning Guide

This guide covers performance optimization techniques for Compozy's runtime system, including configuration tuning, tool optimization, and monitoring strategies.

## Overview

The runtime system's performance is influenced by several factors:

- Runtime configuration (timeouts, buffers, permissions)
- Tool implementation patterns
- System resource management
- Network and I/O operations

## Configuration Optimization

### Timeout Tuning

Proper timeout configuration balances responsiveness with reliability.

#### Production Settings

```yaml
# compozy.yaml - Production optimized
runtime:
    type: bun
    timeout: 30s # Shorter for better responsiveness
    entrypoint: "./entrypoint.ts"
    permissions:
        - --allow-read # Minimal permissions for security
```

```go
// Go configuration - Production
config := &runtime.Config{
    ToolExecutionTimeout:   30 * time.Second,   // Shorter timeout
    BackoffInitialInterval: 50 * time.Millisecond,  // Faster retries
    BackoffMaxInterval:     2 * time.Second,    // Shorter max backoff
    BackoffMaxElapsedTime:  15 * time.Second,   // Faster failure
}
```

#### Development Settings

```yaml
# compozy.yaml - Development
runtime:
    type: bun
    timeout: 120s # Longer for debugging
    entrypoint: "./entrypoint.ts"
    permissions:
        - --allow-read
        - --allow-write
        - --allow-net
```

```go
// Go configuration - Development
config := &runtime.Config{
    ToolExecutionTimeout:   120 * time.Second,  // Longer for debugging
    BackoffInitialInterval: 100 * time.Millisecond,
    BackoffMaxInterval:     5 * time.Second,
    BackoffMaxElapsedTime:  30 * time.Second,
}
```

### Permission Optimization

Minimize permissions to reduce security overhead:

```yaml
# Optimized permission sets
runtime:
    permissions:
        # Read-only tools (fastest)
        - --allow-read

        # Network tools
        - --allow-read
        - --allow-net

        # File processing tools
        - --allow-read
        - --allow-write

        # Avoid when possible (slower)
        # - --allow-env
        # - --allow-sys
        # - --allow-run
```

### Buffer Pool Configuration

The runtime uses buffer pools for optimal memory usage:

```go
// Custom buffer pool for large outputs
const (
    CustomBufferSize = 16 * 1024 // 16KB for large responses
    MaxPooledBuffers = 100       // Limit pooled buffers
)

var customBufferPool = sync.Pool{
    New: func() interface{} {
        return bytes.NewBuffer(make([]byte, 0, CustomBufferSize))
    },
}
```

## Tool Optimization

### Function Design Patterns

#### Efficient Tool Structure

```typescript
// Optimized tool pattern
export async function weather_tool(input: { city: string }) {
    // Early validation (fail fast)
    if (!input?.city || typeof input.city !== "string") {
        throw new Error("Invalid city parameter");
    }

    // Use efficient data structures
    const cache = new Map<string, WeatherData>();

    // Minimize allocations
    const cacheKey = input.city.toLowerCase();
    if (cache.has(cacheKey)) {
        return cache.get(cacheKey);
    }

    // Implement with minimal overhead
    const result = await fetchWeatherData(input.city);
    cache.set(cacheKey, result);

    return result;
}
```

#### Avoid Performance Anti-Patterns

```typescript
// ❌ Inefficient patterns
export async function slow_tool(input: any) {
    // Don't: Deep object cloning
    const data = JSON.parse(JSON.stringify(input));

    // Don't: Synchronous blocking operations
    const result = fs.readFileSync("large-file.txt");

    // Don't: Memory leaks
    const cache = []; // This grows indefinitely
    cache.push(result);

    return result;
}

// ✅ Optimized patterns
export async function fast_tool(input: any) {
    // Do: Shallow copies when needed
    const data = { ...input };

    // Do: Async I/O operations
    const result = await Bun.file("large-file.txt").text();

    // Do: Bounded caches
    const cache = new Map();
    if (cache.size > 100) cache.clear(); // Prevent memory leaks

    return result;
}
```

### Memory Management

#### Streaming Large Data

```typescript
// Stream large responses instead of loading all at once
export async function large_data_tool(input: { url: string }) {
    // Instead of: const data = await fetch(url).then(r => r.json())

    const response = await fetch(input.url);
    const reader = response.body?.getReader();

    if (!reader) throw new Error("No response body");

    const chunks: Uint8Array[] = [];
    let totalSize = 0;
    const maxSize = 10 * 1024 * 1024; // 10MB limit

    try {
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            totalSize += value.length;
            if (totalSize > maxSize) {
                throw new Error(`Response too large: ${totalSize} bytes`);
            }

            chunks.push(value);
        }
    } finally {
        reader.releaseLock();
    }

    // Process chunks efficiently
    return processChunks(chunks);
}
```

#### Memory Pool Usage

```typescript
// Reuse objects to reduce GC pressure
class ObjectPool<T> {
    private pool: T[] = [];
    private createFn: () => T;
    private resetFn: (obj: T) => void;

    constructor(createFn: () => T, resetFn: (obj: T) => void) {
        this.createFn = createFn;
        this.resetFn = resetFn;
    }

    acquire(): T {
        return this.pool.pop() || this.createFn();
    }

    release(obj: T): void {
        this.resetFn(obj);
        if (this.pool.length < 50) {
            // Limit pool size
            this.pool.push(obj);
        }
    }
}

// Usage in tools
const responsePool = new ObjectPool(
    () => ({ data: null, status: "pending" }),
    (obj) => {
        obj.data = null;
        obj.status = "pending";
    },
);

export function pooled_tool(input: any) {
    const response = responsePool.acquire();
    try {
        response.data = processInput(input);
        response.status = "complete";
        return { ...response }; // Return copy
    } finally {
        responsePool.release(response);
    }
}
```

### Async Optimization

#### Concurrent Processing

```typescript
// Process multiple operations concurrently
export async function batch_tool(input: { urls: string[] }) {
    // ❌ Sequential processing (slow)
    // const results = [];
    // for (const url of input.urls) {
    //     results.push(await fetch(url));
    // }

    // ✅ Concurrent processing (fast)
    const maxConcurrency = 5; // Limit concurrent requests
    const results = [];

    for (let i = 0; i < input.urls.length; i += maxConcurrency) {
        const batch = input.urls.slice(i, i + maxConcurrency);
        const batchResults = await Promise.all(
            batch.map((url) =>
                fetch(url)
                    .then((r) => r.json())
                    .catch((err) => ({ error: err.message })),
            ),
        );
        results.push(...batchResults);
    }

    return results;
}
```

#### Efficient Error Handling

```typescript
// Efficient error handling without try-catch overhead
export async function robust_tool(input: any) {
    // Use Result-like pattern for predictable performance
    const result = await safeOperation(input)
        .then((data) => ({ success: true, data }))
        .catch((error) => ({ success: false, error: error.message }));

    if (!result.success) {
        return { error: result.error };
    }

    return result.data;
}

async function safeOperation(input: any): Promise<any> {
    // Operation implementation
    return processInput(input);
}
```

## Runtime Performance Monitoring

### Built-in Metrics

The runtime automatically tracks performance metrics:

```go
// Enable performance logging
config := &runtime.Config{
    ToolExecutionTimeout: 30 * time.Second,
    // Performance monitoring is automatic
}

// Access metrics programmatically
output, err := runtime.ExecuteTool(ctx, toolID, execID, input, env)
if err == nil && output != nil {
    if metadata, ok := output["metadata"]; ok {
        if meta, ok := metadata.(map[string]interface{}); ok {
            if execTime, ok := meta["execution_time"]; ok {
                log.Printf("Tool %s executed in %v ms", toolID, execTime)
            }
        }
    }
}
```

### Custom Performance Tracking

```typescript
// Add performance tracking to tools
export async function monitored_tool(input: any) {
    const startTime = performance.now();
    const startMemory = process.memoryUsage();

    try {
        const result = await processInput(input);

        const endTime = performance.now();
        const endMemory = process.memoryUsage();

        // Log performance metrics
        console.error(
            JSON.stringify({
                type: "performance",
                tool: "monitored_tool",
                execution_time_ms: endTime - startTime,
                memory_used_mb: (endMemory.heapUsed - startMemory.heapUsed) / 1024 / 1024,
                peak_memory_mb: endMemory.heapUsed / 1024 / 1024,
            }),
        );

        return result;
    } catch (error) {
        const endTime = performance.now();
        console.error(
            JSON.stringify({
                type: "performance",
                tool: "monitored_tool",
                execution_time_ms: endTime - startTime,
                error: error.message,
            }),
        );
        throw error;
    }
}
```

### System Resource Monitoring

```bash
# Monitor runtime resource usage
# CPU and memory usage
ps aux | grep bun

# File descriptor usage
lsof -p $(pgrep bun) | wc -l

# Network connections
netstat -an | grep ESTABLISHED | wc -l
```

## Benchmarking and Testing

### Performance Testing Framework

```go
// Benchmark tool execution performance
func BenchmarkToolExecution(b *testing.B) {
    runtime := setupTestRuntime(b)
    input := &core.Input{"test": "data"}
    env := core.EnvMap{}

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        _, err := runtime.ExecuteTool(
            context.Background(),
            "benchmark_tool",
            core.MustNewID(),
            input,
            env,
        )
        if err != nil {
            b.Fatal(err)
        }
    }
}

// Test different input sizes
func BenchmarkToolExecutionSizes(b *testing.B) {
    sizes := []int{1024, 10240, 102400, 1024000} // 1KB to 1MB

    for _, size := range sizes {
        b.Run(fmt.Sprintf("input_size_%d", size), func(b *testing.B) {
            runtime := setupTestRuntime(b)
            input := &core.Input{"data": strings.Repeat("x", size)}

            b.ResetTimer()
            b.SetBytes(int64(size))

            for i := 0; i < b.N; i++ {
                _, err := runtime.ExecuteTool(
                    context.Background(),
                    "benchmark_tool",
                    core.MustNewID(),
                    input,
                    core.EnvMap{},
                )
                if err != nil {
                    b.Fatal(err)
                }
            }
        })
    }
}
```

### Load Testing

```typescript
// Load testing tool for stress testing
export async function load_test_tool(input: {
    duration_ms: number;
    rps: number;
    target_tool: string;
    test_input: any;
}) {
    const startTime = Date.now();
    const results = [];
    let requestCount = 0;
    const interval = 1000 / input.rps; // ms between requests

    while (Date.now() - startTime < input.duration_ms) {
        const requestStart = performance.now();

        try {
            // Simulate tool execution
            const result = await simulateToolCall(input.target_tool, input.test_input);
            results.push({
                request: ++requestCount,
                duration_ms: performance.now() - requestStart,
                success: true,
            });
        } catch (error) {
            results.push({
                request: ++requestCount,
                duration_ms: performance.now() - requestStart,
                success: false,
                error: error.message,
            });
        }

        // Maintain target RPS
        await new Promise((resolve) => setTimeout(resolve, interval));
    }

    // Calculate statistics
    const successCount = results.filter((r) => r.success).length;
    const avgDuration = results.reduce((sum, r) => sum + r.duration_ms, 0) / results.length;
    const p95Duration = calculatePercentile(
        results.map((r) => r.duration_ms),
        95,
    );

    return {
        total_requests: requestCount,
        success_rate: successCount / requestCount,
        avg_duration_ms: avgDuration,
        p95_duration_ms: p95Duration,
        throughput_rps: requestCount / (input.duration_ms / 1000),
    };
}
```

## Production Optimization

### Runtime Configuration

```yaml
# Production optimized configuration
name: production-app
version: 1.0.0

runtime:
    type: bun
    timeout: 30s
    entrypoint: "./entrypoint.ts"
    permissions:
        - --allow-read
        - --allow-net # Only if needed

# Optimize for production workloads
performance:
    max_concurrent_tools: 10
    enable_metrics: true
    log_slow_queries: true
    slow_query_threshold: 5s
```

### Deployment Considerations

#### Resource Limits

```yaml
# Kubernetes deployment with resource limits
apiVersion: apps/v1
kind: Deployment
metadata:
    name: compozy-runtime
spec:
    template:
        spec:
            containers:
                - name: compozy
                  resources:
                      requests:
                          memory: "256Mi"
                          cpu: "250m"
                      limits:
                          memory: "512Mi" # Prevent memory bloat
                          cpu: "500m" # Prevent CPU starvation
                  env:
                      - name: BUN_CONFIG_PROFILE
                        value: "production"
```

#### Monitoring Integration

```go
// Integration with monitoring systems
func setupPerformanceMonitoring(runtime Runtime) {
    // Prometheus metrics
    executionDuration := prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "compozy_tool_execution_duration_seconds",
            Help: "Tool execution duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"tool_id", "status"},
    )

    // Wrap runtime with monitoring
    monitoredRuntime := &MonitoredRuntime{
        runtime: runtime,
        metrics: executionDuration,
    }

    prometheus.MustRegister(executionDuration)
}
```

## Performance Best Practices Summary

### Configuration

1. **Use minimal permissions** - Only grant necessary access
2. **Tune timeouts appropriately** - Balance responsiveness vs reliability
3. **Configure buffer sizes** - Match expected data sizes
4. **Enable monitoring** - Track performance metrics

### Tool Development

1. **Validate inputs early** - Fail fast for invalid data
2. **Use async patterns** - Avoid blocking operations
3. **Implement backpressure** - Limit concurrent operations
4. **Pool reusable objects** - Reduce GC pressure
5. **Stream large data** - Avoid loading everything in memory

### System Management

1. **Monitor resource usage** - CPU, memory, file descriptors
2. **Implement circuit breakers** - Prevent cascade failures
3. **Use load balancing** - Distribute work across instances
4. **Cache frequently used data** - Reduce external API calls

### Testing

1. **Benchmark critical paths** - Measure performance regularly
2. **Load test under realistic conditions** - Validate scalability
3. **Profile memory usage** - Identify leaks and optimization opportunities
4. **Test timeout scenarios** - Ensure graceful handling

By following these guidelines, you can achieve optimal performance from the Compozy runtime system while maintaining reliability and security.
