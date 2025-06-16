---
status: excluded
---

# Task 7.0: Performance Validation and Testing

## Overview

Conduct formal performance validation to ensure monitoring overhead stays within acceptable limits (<0.5% latency increase, <2% resource usage increase) and verify the rollback mechanism.

## Subtasks

- [ ] 7.1 Set up performance testing environment (AWS t3.medium or equivalent)
- [ ] 7.2 Create baseline performance measurements without monitoring
- [ ] 7.3 Run load tests with ghz (1000 RPS for 5 minutes) with monitoring enabled
- [ ] 7.4 Profile with pprof to identify overhead sources
- [ ] 7.5 Validate <0.5% latency increase and <2% resource usage increase
- [ ] 7.6 Document performance test results and get SRE sign-off
- [ ] 7.7 Verify that setting `MONITORING_ENABLED=false` successfully disables the `/metrics` endpoint and removes instrumentation overhead

## Implementation Details

### Performance Validation Plan

Based on the tech spec (lines 70-77), implement the validation:

1. **Environment**: AWS `t3.medium` instance (2vCPU, 4GB RAM)
2. **Load Profile**: 1,000 requests per second for 5 minutes
3. **Success Criteria**:
    - 95th percentile latency increase ≤0.5%
    - CPU/memory usage within 2% of baseline
4. **Ownership**: SRE team executes validation and provides sign-off

### Test Environment Setup

```bash
# 7.1 - Environment setup script
#!/bin/bash

# Launch t3.medium instance
aws ec2 run-instances \
    --instance-type t3.medium \
    --image-id ami-xxx \
    --key-name perf-test \
    --security-group-ids sg-xxx \
    --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=compozy-perf-test}]'

# Install dependencies
ssh ubuntu@instance-ip << 'EOF'
    # Install Go
    wget https://go.dev/dl/go1.21.linux-amd64.tar.gz
    sudo tar -C /usr/local -xzf go1.21.linux-amd64.tar.gz

    # Install ghz
    go install github.com/bojand/ghz/cmd/ghz@latest

    # Install monitoring tools
    sudo apt-get update
    sudo apt-get install -y htop iotop nethogs

    # Clone and build Compozy
    git clone https://github.com/compozy/compozy
    cd compozy
    make build
EOF
```

### Baseline Measurement

```bash
# 7.2 - Baseline test without monitoring
# Start Compozy with monitoring disabled
MONITORING_ENABLED=false ./compozy serve &
SERVER_PID=$!

# Wait for startup
sleep 5

# Run baseline load test
ghz --insecure \
    --proto ./api/proto/service.proto \
    --call compozy.Service/Execute \
    --data '{"workflow_id": "test-workflow"}' \
    --rps 1000 \
    --duration 5m \
    --format json \
    localhost:8080 > baseline_results.json

# Capture resource usage
pidstat -u -r -d -h -p $SERVER_PID 1 300 > baseline_resources.txt &

# Profile CPU and memory
curl http://localhost:6060/debug/pprof/profile?seconds=30 > baseline_cpu.prof
curl http://localhost:6060/debug/pprof/heap > baseline_heap.prof

kill $SERVER_PID
```

### Load Test with Monitoring

```bash
# 7.3 - Test with monitoring enabled
# Start Compozy with monitoring enabled
MONITORING_ENABLED=true ./compozy serve &
SERVER_PID=$!

# Wait for startup
sleep 5

# Verify metrics endpoint is available
curl -s http://localhost:8080/metrics | head -20

# Run load test with monitoring
ghz --insecure \
    --proto ./api/proto/service.proto \
    --call compozy.Service/Execute \
    --data '{"workflow_id": "test-workflow"}' \
    --rps 1000 \
    --duration 5m \
    --format json \
    localhost:8080 > monitoring_results.json

# Capture resource usage
pidstat -u -r -d -h -p $SERVER_PID 1 300 > monitoring_resources.txt &

# Profile with monitoring enabled
curl http://localhost:6060/debug/pprof/profile?seconds=30 > monitoring_cpu.prof
curl http://localhost:6060/debug/pprof/heap > monitoring_heap.prof

kill $SERVER_PID
```

### Performance Analysis

```go
// 7.4 & 7.5 - Analysis script
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

type GHZResult struct {
    Latencies struct {
        P50  float64 `json:"50"`
        P95  float64 `json:"95"`
        P99  float64 `json:"99"`
        Mean float64 `json:"mean"`
    } `json:"latencyDistribution"`
    Rps float64 `json:"rps"`
}

func analyzeResults() {
    baseline := loadResults("baseline_results.json")
    monitoring := loadResults("monitoring_results.json")

    // Calculate latency increase
    p95Increase := (monitoring.Latencies.P95 - baseline.Latencies.P95) / baseline.Latencies.P95 * 100

    fmt.Printf("Performance Impact Analysis:\n")
    fmt.Printf("P95 Latency - Baseline: %.2fms, Monitoring: %.2fms\n",
        baseline.Latencies.P95, monitoring.Latencies.P95)
    fmt.Printf("P95 Latency Increase: %.2f%%\n", p95Increase)

    // Check success criteria
    if p95Increase > 0.5 {
        fmt.Printf("❌ FAILED: P95 latency increase (%.2f%%) exceeds 0.5%% threshold\n", p95Increase)
    } else {
        fmt.Printf("✅ PASSED: P95 latency increase (%.2f%%) within threshold\n", p95Increase)
    }
}
```

### pprof Analysis

```bash
# 7.4 - Analyze CPU profiles
go tool pprof -top baseline_cpu.prof > baseline_cpu_top.txt
go tool pprof -top monitoring_cpu.prof > monitoring_cpu_top.txt

# Compare heap profiles
go tool pprof -alloc_space -top baseline_heap.prof > baseline_heap_top.txt
go tool pprof -alloc_space -top monitoring_heap.prof > monitoring_heap_top.txt

# Generate flame graphs
go tool pprof -http=:8081 monitoring_cpu.prof
```

### Rollback Verification

```bash
# 7.7 - Test rollback mechanism
# Start with monitoring enabled
MONITORING_ENABLED=true ./compozy serve &
SERVER_PID=$!
sleep 5

# Verify metrics endpoint works
curl -s http://localhost:8080/metrics | grep "compozy_http_requests_total"
if [ $? -eq 0 ]; then
    echo "✅ Metrics endpoint active with MONITORING_ENABLED=true"
fi

kill $SERVER_PID

# Start with monitoring disabled
MONITORING_ENABLED=false ./compozy serve &
SERVER_PID=$!
sleep 5

# Verify metrics endpoint returns 503 or empty
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/metrics)
if [ "$HTTP_CODE" = "503" ]; then
    echo "✅ Metrics endpoint returns 503 with MONITORING_ENABLED=false"
fi

# Run quick performance test to verify no overhead
ghz --insecure \
    --proto ./api/proto/service.proto \
    --call compozy.Service/Execute \
    --data '{"workflow_id": "test-workflow"}' \
    --rps 100 \
    --duration 30s \
    localhost:8080

kill $SERVER_PID
```

### Performance Report Template

```markdown
# Monitoring Performance Validation Report

## Test Environment

- Instance Type: AWS t3.medium (2vCPU, 4GB RAM)
- OS: Ubuntu 22.04
- Go Version: 1.21
- Test Date: [DATE]

## Test Parameters

- Load: 1000 RPS for 5 minutes
- Endpoint: compozy.Service/Execute
- Concurrent Connections: 50

## Results Summary

### Latency Impact

| Metric | Baseline | With Monitoring | Increase | Pass/Fail |
| ------ | -------- | --------------- | -------- | --------- |
| P50    | X.XXms   | X.XXms          | X.XX%    | ✅/❌     |
| P95    | X.XXms   | X.XXms          | X.XX%    | ✅/❌     |
| P99    | X.XXms   | X.XXms          | X.XX%    | ✅/❌     |

### Resource Usage

| Metric    | Baseline | With Monitoring | Increase | Pass/Fail |
| --------- | -------- | --------------- | -------- | --------- |
| CPU (avg) | XX%      | XX%             | X.XX%    | ✅/❌     |
| Memory    | XXXMiB   | XXXMiB          | X.XX%    | ✅/❌     |

## Performance Profile Analysis

[Summary of pprof findings]

## Rollback Test Results

- MONITORING_ENABLED=false successfully disables endpoint: ✅/❌
- No performance overhead when disabled: ✅/❌

## Conclusion

[Overall pass/fail determination]

## SRE Sign-off

- Reviewed by: [Name]
- Date: [Date]
- Approval: ✅/❌
```

## Success Criteria

- Test environment properly configured
- Baseline measurements captured accurately
- Load tests execute successfully at 1000 RPS
- Performance overhead within limits (<0.5% latency, <2% resources)
- pprof analysis identifies any hotspots
- Rollback mechanism verified working
- Formal report generated with SRE sign-off
