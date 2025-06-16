---
status: excluded
---

<task_context>
<domain>engine/infra/monitoring</domain>
<type>implementation</type>
<scope>tooling</scope>
<complexity>high</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 8.0: Implement CI Label Validation

## Overview

Create a custom CI linter that validates metric labels against an allow-list to prevent high cardinality issues. This is a critical safeguard to ensure only approved labels are used in metrics.

## Subtasks

- [ ] 8.1 Research Go AST analysis frameworks (go/analysis) for metric parsing
- [ ] 8.2 Design label validation rules and centralized allow-list format
- [ ] 8.3 Implement metric declaration parser for OTEL instruments
- [ ] 8.4 Create validation logic to check labels against allow-list
- [ ] 8.5 Integrate linter into Makefile as part of lint target
- [ ] 8.6 Add linter to CI pipeline with failing builds on violations
- [ ] 8.7 Create tests for the linter with positive and negative cases

## Implementation Details

### Label Allow-List

Based on the tech spec (lines 53-60), the allowed labels are:

| Metric Category | Allowed Labels                         |
| --------------- | -------------------------------------- |
| HTTP            | `method`, `path`, `status_code`        |
| Temporal        | `workflow_type`                        |
| System          | `version`, `commit_hash`, `go_version` |

### Linter Architecture

```go
// cmd/metriclinter/main.go
package main

import (
    "go/ast"
    "go/parser"
    "go/token"
    "golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
    Name: "metriclabels",
    Doc:  "Validates OpenTelemetry metric labels against allow-list",
    Run:  run,
}

// Allow-list configuration
var allowedLabels = map[string][]string{
    "http":     {"method", "path", "status_code"},
    "temporal": {"workflow_type"},
    "system":   {"version", "commit_hash", "go_version"},
}
```

### AST Analysis Implementation

```go
func run(pass *analysis.Pass) (interface{}, error) {
    for _, file := range pass.Files {
        ast.Inspect(file, func(n ast.Node) bool {
            // Look for metric creation calls
            call, ok := n.(*ast.CallExpr)
            if !ok {
                return true
            }

            // Check if it's a metric creation
            if isMetricCreation(call) {
                validateMetricLabels(pass, call)
            }

            return true
        })
    }
    return nil, nil
}

func isMetricCreation(call *ast.CallExpr) bool {
    // Identify OTEL metric creation patterns:
    // - meter.Int64Counter()
    // - meter.Float64Histogram()
    // - meter.Int64UpDownCounter()
    // etc.

    sel, ok := call.Fun.(*ast.SelectorExpr)
    if !ok {
        return false
    }

    metricMethods := []string{
        "Int64Counter",
        "Float64Counter",
        "Int64UpDownCounter",
        "Float64UpDownCounter",
        "Int64Histogram",
        "Float64Histogram",
        "Int64Gauge",
        "Float64Gauge",
    }

    for _, method := range metricMethods {
        if sel.Sel.Name == method {
            return true
        }
    }

    return false
}
```

### Label Extraction and Validation

```go
func validateMetricLabels(pass *analysis.Pass, call *ast.CallExpr) {
    metricName := extractMetricName(call)
    if metricName == "" {
        return
    }

    // Determine metric category from name
    category := getMetricCategory(metricName)
    allowedForCategory := allowedLabels[category]

    // Find attribute.String() calls in metric recording
    labels := extractLabelsFromUsage(pass, metricName)

    for _, label := range labels {
        if !isAllowed(label, allowedForCategory) {
            pass.Reportf(call.Pos(),
                "metric %q uses disallowed label %q. Allowed labels for %s metrics: %v",
                metricName, label, category, allowedForCategory)
        }
    }
}

func getMetricCategory(metricName string) string {
    switch {
    case strings.Contains(metricName, "_http_"):
        return "http"
    case strings.Contains(metricName, "_temporal_"):
        return "temporal"
    case strings.Contains(metricName, "_build_") || strings.Contains(metricName, "_uptime_"):
        return "system"
    default:
        return "unknown"
    }
}
```

### Usage Pattern Detection

```go
func extractLabelsFromUsage(pass *analysis.Pass, metricName string) []string {
    var labels []string

    // Find where this metric is used with Add/Record calls
    for _, file := range pass.Files {
        ast.Inspect(file, func(n ast.Node) bool {
            call, ok := n.(*ast.CallExpr)
            if !ok {
                return true
            }

            // Look for metric.Add() or metric.Record() calls
            if isMetricRecording(call, metricName) {
                // Extract attribute.String("key", value) calls
                labels = append(labels, extractAttributeKeys(call)...)
            }

            return true
        })
    }

    return unique(labels)
}

func extractAttributeKeys(call *ast.CallExpr) []string {
    var keys []string

    // Look for attribute.String("key", ...) patterns
    for _, arg := range call.Args {
        if attrCall, ok := arg.(*ast.CallExpr); ok {
            if isAttributeCall(attrCall) {
                if key := extractStringLiteral(attrCall.Args[0]); key != "" {
                    keys = append(keys, key)
                }
            }
        }
    }

    return keys
}
```

### Allow-List Configuration File

```yaml
# .metriclabels.yaml
metric_categories:
    http:
        prefix: "_http_"
        allowed_labels:
            - method
            - path
            - status_code

    temporal:
        prefix: "_temporal_"
        allowed_labels:
            - workflow_type

    system:
        prefix: "_build_|_uptime_"
        allowed_labels:
            - version
            - commit_hash
            - go_version

# Global settings
strict_mode: true # Fail on unknown metric categories
```

### Makefile Integration

```makefile
# Add to Makefile
.PHONY: lint-metrics
lint-metrics:
	@echo "Running metric label validation..."
	@go run ./cmd/metriclinter ./...

# Update main lint target
lint: fmt lint-metrics
	golangci-lint run
```

### CI Pipeline Integration

```yaml
# .github/workflows/ci.yml
- name: Validate Metric Labels
  run: make lint-metrics

# Or in GitLab CI
lint-metrics:
  stage: test
  script:
    - make lint-metrics
  allow_failure: false
```

### Testing the Linter

```go
// cmd/metriclinter/main_test.go
func TestMetricLabelValidation(t *testing.T) {
    tests := []struct {
        name     string
        code     string
        wantErr  bool
        errMsg   string
    }{
        {
            name: "valid HTTP metric labels",
            code: `
httpRequestsTotal.Add(ctx, 1,
    metric.WithAttributes(
        attribute.String("method", "GET"),
        attribute.String("path", "/users/:id"),
        attribute.String("status_code", "200"),
    ))`,
            wantErr: false,
        },
        {
            name: "invalid HTTP metric label",
            code: `
httpRequestsTotal.Add(ctx, 1,
    metric.WithAttributes(
        attribute.String("user_id", "123"),
    ))`,
            wantErr: true,
            errMsg:  "disallowed label \"user_id\"",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := runLinter(tt.code)
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Error Reporting Format

```
engine/infra/monitoring/middleware/gin.go:45:5: metric "compozy_http_requests_total" uses disallowed label "user_id". Allowed labels for http metrics: [method, path, status_code]
engine/infra/monitoring/interceptor/temporal.go:67:5: metric "compozy_temporal_workflow_failed_total" uses disallowed label "error_type". Allowed labels for temporal metrics: [workflow_type]
```

## Success Criteria

- CI linter successfully validates metric labels against allow-list
- Linter fails builds on disallowed or high-cardinality labels
- Tool properly integrated into CI pipeline
- Documentation explains linter usage and label validation rules
- All existing metrics pass validation
- Tool can be easily extended for future label requirements

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test-all` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
