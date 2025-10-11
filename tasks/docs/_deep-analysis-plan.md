# Deep Analysis Plan Template

## Analysis Overview

[Provide a high-level overview of the analysis scope, objectives, and expected outcomes.]

## Analysis Goals

[List specific, measurable objectives for this analysis:

- Identify root causes and contributing factors
- Map system dependencies and integration points
- Assess code quality and architectural compliance
- Develop actionable solution strategies
- Validate recommendations against project standards]

## Scope Definition

### In Scope

- [Files, modules, or systems to be analyzed]
- [Specific issues or symptoms to investigate]
- [Related components and dependencies to examine]

### Out of Scope

- [Explicitly excluded areas or systems]
- [Future considerations beyond current analysis]
- [Areas requiring separate analysis efforts]

## Context Discovery Strategy

### Phase 1: Scope & Context Discovery

1. **Identify analysis scope** from the request
2. **Use Zen MCP tools** (analyze, tracer) to discover repository structure, related modules, implicated files, interfaces, contracts
3. **Trace dependencies** and integration boundaries using Zen MCP debug and tracer tools
4. **Review applicable `.cursor/rules`** and note constraints that shape acceptable solutions

#### Full-Context Breadth Analysis (REQUIRED)

- Direct targets: specified files/functions/components
- Neighboring code: same package/module peers and shared utilities
- Upstream callers and public entrypoints (HTTP/gRPC/CLI/workflow)
- Downstream callees: repositories, adapters, external services, activities/tools
- Interfaces and implementations; check LSP/ISP/DIP compliance
- Cross-package dependencies and domain boundaries (agent/task/tool/workflow/runtime/infra)
- Configuration and environment: YAML/JSON, defaults, feature flags
- Middleware/interceptors and cross-cutting concerns
- Error handling and context propagation; logger.FromContext(ctx) usage
- Concurrency: goroutines, channels, errgroup, locks, timeouts, cleanup
- Persistence: transactions, migrations, connection lifecycle, retries
- Network boundaries: timeouts, retries, idempotency, rate limits
- Tests and fixtures; documentation cues

## Zen MCP Analysis Methodology

### Phase 2: Comprehensive Analysis Using Zen MCP Tools

**Execute systematic analysis using Zen MCP's comprehensive toolset:**

- **`analyze`**: Understand codebase structure and identify architectural patterns
- **`tracer`**: Map execution flows, data paths, and dependency relationships
- **`debug`**: Systematic issue investigation and root cause analysis
- **`codereview`**: Comprehensive code quality and security assessment
- **`planner`**: Develop structured solution strategies and implementation plans
- **`refactor`**: Identify code improvement opportunities

#### Analysis Steps

- Map control/data flows, state transitions, and cross-boundary interactions using tracer
- Identify critical paths, side effects, and invariants using debug and analyze tools
- Detect runtime/logic/concurrency/memory/resource/performance issues using comprehensive analysis
- Validate API contracts and configuration assumptions using codereview
- Correlate findings into root causes and contributing factors using debug workflows
- Propose solution strategies with trade-offs and phased plans using planner tool

## Deliverables Structure

### Context Map

- Enumerated graph/list of key related components and their roles
- Impacted Areas Matrix: area → impact → risk → suggested focus

### Findings Categories

- **Root Causes**: Primary issues identified with evidence
- **Contributing Factors**: Secondary issues that compound the problem
- **Architectural Issues**: Design patterns or structural problems
- **Code Quality Issues**: Maintainability, performance, or security concerns
- **Integration Issues**: Problems with external dependencies or interfaces

### Solution Strategies

- **Immediate Fixes**: Quick wins that address critical issues
- **Architectural Improvements**: Long-term structural changes needed
- **Code Quality Enhancements**: Refactoring and optimization opportunities
- **Testing Improvements**: Additional test coverage or validation needed

## Standards Compliance Validation

### Required Rules Compliance

- Architecture & Design: @architecture.mdc, @go-coding-standards.mdc, @backwards-compatibility.mdc
- APIs & Docs: @api-standards.mdc, @core-libraries.mdc
- Testing & Review: @test-standards.mdc, @task-review.mdc (or project equivalent), docs/cursor_rules

### Compliance Protocol

- Map each recommendation to relevant rules and call out any potential deviations
- Prefer patterns already used in the codebase (constructor DI, context-first, logger.FromContext(ctx), interface boundaries, clean architecture layers)

## Output Format

### Final Document Structure

1. **Executive Summary**: High-level findings and recommendations
2. **Context Map**: Visual representation of analyzed components
3. **Detailed Findings**: Categorized issues with evidence and impact assessment
4. **Root Cause Analysis**: Systematic breakdown of identified problems
5. **Solution Strategies**: Actionable recommendations with trade-offs
6. **Implementation Plan**: Phased approach with dependencies
7. **Standards Compliance**: Validation against project rules
8. **Risk Assessment**: Potential issues and mitigation strategies

### Acceptance Criteria

- All implicated files and dependencies documented
- Findings supported by evidence from Zen MCP tool analysis
- Recommendations validated against project standards
- Solution strategies provide clear implementation paths
- Document demonstrates comprehensive breadth analysis

## Success Metrics

[Define how the success of this analysis will be measured:

- Completeness of context mapping
- Accuracy of root cause identification
- Actionability of solution recommendations
- Compliance with project standards
- Quality and depth of analysis documentation]
