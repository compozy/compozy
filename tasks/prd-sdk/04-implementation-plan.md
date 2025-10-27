# Implementation Plan: Compozy GO SDK

**Date:** 2025-01-25
**Version:** 2.0.0
**Timeline:** 1 week prep + 8-10 weeks to v1.0
**Estimated Reading Time:** 25 minutes

---

## Overview

This document provides a detailed implementation plan for the Compozy GO SDK, broken down into phases, PRs, and deliverables.

**Updated Approach:** Phase 0 prototype first, then incremental implementation
**Risk:** LOW-MEDIUM (architectural validation needed)
**Resources:** 2-3 developers (1 lead, 1-2 supporting)

---

## Timeline Summary

```
Week 0:   Phase 0 - Prototype & Validation
Week 1-2: Phase 1 - Foundation (workspace, core builders, integration layer)
Week 3-4: Phase 2 - Complete Entity Coverage (all 9 task types, knowledge, memory)
Week 5-6: Phase 3 - Advanced Features (MCP, runtime, schema, monitoring)
Week 7-8: Phase 4 - Polish (docs, examples, testing, performance)
Week 9+:  Beta Testing & v1.0 Release
```

---

## Phase 0: Prototype & Validation (Week 0 - MANDATORY)

### Goals
- Validate SDK → Engine integration architecture
- Prove context-first pattern works in builders
- Confirm task type alignment
- Test simple workflow end-to-end

### Tasks

**Day 1-2: Minimal sdk Module**
- Create minimal `go.work` and `sdk/go.mod`
- Define sdk module dependencies on main module
- Create basic project builder (no other builders yet)
- Create basic workflow builder (minimal)
- Implement context-first Build() methods

**Day 3-4: Integration Layer Prototype**
- Implement `sdk/compozy/integration.go` (SDK → Engine loading)
- Test project config registration in engine
- Validate resource store integration
- Test workflow execution with SDK-built config

**Day 5: Validation**
- Build simple workflow with SDK
- Load into embedded engine
- Execute workflow
- Verify full pipeline works
- **Decision Gate:** Continue only if prototype succeeds

### Deliverables
- ✅ Minimal `sdk/` module with workspace
- ✅ Basic Project + Workflow builders
- ✅ Integration layer (SDK → Engine)
- ✅ Simple workflow execution test
- ✅ Architecture validation report

### Success Criteria
- Workflow built with SDK executes in engine
- Context-first pattern works
- No circular dependencies
- Resource store integration works
- Integration layer complexity is manageable

### PR #0: Prototype
**Title:** Phase 0: SDK → Engine integration prototype
**Files Changed:** 15-20 files (new)
**Lines of Code:** ~800-1,000 LOC
**Breakdown:**
- `go.work`, `sdk/go.mod`: 20 lines
- `sdk/project/builder.go`: 150 LOC
- `sdk/workflow/builder.go`: 100 LOC
- `sdk/compozy/integration.go`: 300 LOC
- Tests: 400 LOC

**Test Coverage:** 90%+ for integration layer
**Review Focus:** Architecture validation, integration approach
**Estimated Review Time:** 3-4 hours

**Rollback:** If prototype fails, revisit architecture decisions before proceeding

---

## Phase 1: Foundation (Week 1-2)

### Goals
- Complete workspace setup
- Implement all core builders (Project, Model, Workflow, Agent, Task)
- Full integration layer implementation
- Error handling infrastructure
- First comprehensive examples

### Tasks

**Week 1: Core Infrastructure**
- Expand Project builder with all methods
- Implement Model builder
- Complete Workflow builder
- Implement Agent builder + ActionBuilder
- Implement ALL 9 task builders:
  - Basic, Parallel, Collection, Router
  - Wait, Aggregate, Composite, Signal, Memory
- Create BuildError infrastructure in `sdk/internal/errors`
- Create validation helpers in `sdk/internal/validate`

**Week 2: Integration & Testing**
- Complete `sdk/compozy` package:
  - `compozy.go` - Main struct and lifecycle
  - `builder.go` - Configuration builder
  - `integration.go` - Full SDK → Engine integration
  - `execution.go` - Direct workflow execution
- Comprehensive unit tests for all builders
- Integration tests for SDK → Engine loading
- First 3 examples:
  - 01_simple_workflow.go
  - 02_parallel_tasks.go
  - 03_agent_with_actions.go

### Deliverables
- ✅ Complete workspace setup (`go.work`, `sdk/go.mod`)
- ✅ `sdk/project/` - Project builder (complete)
- ✅ `sdk/model/` - Model builder
- ✅ `sdk/workflow/` - Workflow builder (complete)
- ✅ `sdk/agent/` - Agent + Action builders
- ✅ `sdk/task/` - All 9 task type builders
- ✅ `sdk/compozy/` - Embedded engine package (complete)
- ✅ `sdk/internal/errors` - BuildError infrastructure
- ✅ `sdk/internal/validate` - Validation helpers
- ✅ 3 working examples
- ✅ CI/CD configured for workspace

### Validation
```bash
# Test workspace
go work sync
cd sdk && go test ./...

# Test task builders (all 9 types)
go test ./sdk/task/...

# Test integration
go test ./sdk/compozy/...

# Run examples
go run sdk/examples/01_simple_workflow.go
go run sdk/examples/02_parallel_tasks.go
go run sdk/examples/03_agent_with_actions.go
```

### PR #1: Core Foundation
**Title:** Phase 1: Core builders and integration layer
**Files Changed:** 50-60 files (new)
**Lines of Code:** ~4,000-5,000 LOC
**Breakdown:**
- `sdk/project/*.go`: 400 LOC
- `sdk/model/*.go`: 200 LOC
- `sdk/workflow/*.go`: 400 LOC
- `sdk/agent/*.go`: 600 LOC (includes ActionBuilder)
- `sdk/task/*.go`: 900 LOC (9 task types)
- `sdk/compozy/*.go`: 1,000 LOC
- `sdk/internal/*.go`: 300 LOC
- Tests: 1,500 LOC
- Examples: 300 LOC

**Test Coverage:** 95%+ for all builders
**Review Focus:** Builder API consistency, context propagation, integration layer
**Estimated Review Time:** 6-8 hours

**Rollback Procedure:**
```bash
git revert <commit-hash>
rm -rf sdk/
git checkout go.work
```

---

## Phase 2: Complete Entity Coverage (Week 3-4)

### Goals
- Implement Knowledge system (5 builders)
- Implement Memory system (2 builders, full features)
- Implement Tool builder
- Implement Schema builders (2 builders)
- Comprehensive examples for each system

### Tasks

**Week 3: Knowledge & Memory**
- Implement `sdk/knowledge/`:
  - `embedder.go` - EmbedderBuilder
  - `vectordb.go` - VectorDBBuilder
  - `source.go` - SourceBuilder (file, dir, URL, API)
  - `base.go` - BaseBuilder
  - `binding.go` - BindingBuilder
- Implement `sdk/memory/`:
  - `config.go` - ConfigBuilder (full features: flush, privacy, persistence)
  - `reference.go` - ReferenceBuilder
- Unit tests for all builders
- Examples:
  - 04_knowledge_rag.go
  - 05_memory_conversation.go

**Week 4: Tools & Schema**
- Implement `sdk/tool/builder.go` - Tool builder
- Implement `sdk/schema/`:
  - `builder.go` - Schema builder with validation
  - `property.go` - Property builder
- Unit tests
- Examples:
  - 06_custom_tools.go
  - 07_schema_validation.go

### Deliverables
- ✅ `sdk/knowledge/` - 5 builders (complete)
- ✅ `sdk/memory/` - 2 builders with full features
- ✅ `sdk/tool/` - Tool builder
- ✅ `sdk/schema/` - Schema + Property builders
- ✅ 4 new examples (knowledge, memory, tools, schema)
- ✅ Unit tests for all

### Validation
```bash
# Test knowledge system
go test ./sdk/knowledge/...

# Test memory system
go test ./sdk/memory/...

# Test tools and schema
go test ./sdk/tool/...
go test ./sdk/schema/...

# Run examples
go run sdk/examples/04_knowledge_rag.go
go run sdk/examples/05_memory_conversation.go
```

### PR #2: Knowledge, Memory, Tools, Schema
**Title:** Phase 2: Knowledge, memory, tools, and schema builders
**Files Changed:** 35-40 files (new)
**Lines of Code:** ~3,000-3,500 LOC
**Breakdown:**
- `sdk/knowledge/*.go`: 700 LOC
- `sdk/memory/*.go`: 500 LOC (full features)
- `sdk/tool/*.go`: 200 LOC
- `sdk/schema/*.go`: 400 LOC
- Tests: 1,000 LOC
- Examples: 400 LOC

**Test Coverage:** 95%+
**Review Focus:** Knowledge system API, memory features, schema validation
**Estimated Review Time:** 5-6 hours

---

## Phase 3: Advanced Features (Week 5-6)

### Goals
- Implement MCP integration (full config)
- Implement Runtime configuration (+ native tools)
- Implement Schedule builder
- Implement Monitoring builder

### Tasks

**Week 5: MCP & Runtime**
- Implement `sdk/mcp/builder.go`:
  - Full MCP configuration (URL, command, transport, headers, protocol)
  - Session management
  - Environment variables
- Implement `sdk/runtime/`:
  - `builder.go` - Runtime builder (bun)
  - `native_tools.go` - NativeToolsBuilder (call_agents, call_workflows)
- Unit tests
- Examples:
  - 08_mcp_integration.go
  - 09_runtime_native_tools.go

**Week 6: Schedule & Monitoring**
- Implement `sdk/schedule/builder.go` - Schedule builder
- Implement `sdk/monitoring/builder.go` - Monitoring builder (Prometheus, tracing)
- Unit tests
- Examples:
  - 10_scheduled_workflow.go
  - 11_monitoring_metrics.go

### Deliverables
- ✅ `sdk/mcp/` - Full MCP configuration
- ✅ `sdk/runtime/` - Runtime + native tools
- ✅ `sdk/schedule/` - Schedule builder
- ✅ `sdk/monitoring/` - Monitoring builder
- ✅ 4 new examples
- ✅ Unit tests for all

### Validation
```bash
go test ./sdk/mcp/...
go test ./sdk/runtime/...
go test ./sdk/schedule/...
go test ./sdk/monitoring/...

go run sdk/examples/08_mcp_integration.go
go run sdk/examples/09_runtime_native_tools.go
```

### PR #3: MCP, Runtime, Schedule, Monitoring
**Title:** Phase 3: MCP, runtime, schedule, and monitoring
**Files Changed:** 25-30 files (new)
**Lines of Code:** ~2,000-2,500 LOC
**Breakdown:**
- `sdk/mcp/*.go`: 400 LOC (full config)
- `sdk/runtime/*.go`: 400 LOC (includes native tools)
- `sdk/schedule/*.go`: 200 LOC
- `sdk/monitoring/*.go`: 300 LOC
- Tests: 800 LOC
- Examples: 400 LOC

**Test Coverage:** 95%+
**Review Focus:** MCP transport options, native tools, monitoring integration
**Estimated Review Time:** 4-5 hours

---

## Phase 4: Polish & Documentation (Week 7-8)

### Goals
- Complete documentation (GoDoc + user guide)
- Performance optimization
- Comprehensive examples
- Integration tests
- Migration guide validation

### Tasks

**Week 7: Documentation**
- Generate complete GoDoc for all packages
- Write comprehensive `sdk/README.md`
- Update all examples with comments and error handling
- Create troubleshooting guide
- Document builder lifecycle and immutability
- Document error handling patterns
- Create performance benchmarks
- Document concurrency safety
 - Plan SDK docs section (top-level) alongside Core/CLI/Schema/API — do not change docs/ in this PR; create plan in `tasks/prd-sdk/_docs.md`
   - New folder: `docs/content/docs/sdk/*`
   - Root nav: add `sdk` to `docs/content/docs/meta.json` (follow-up PR in docs site)
   - Pages: overview, getting-started, architecture, entities, builders/*, examples, migration, testing, troubleshooting

**Week 8: Testing & Validation**
- Integration tests: SDK → Engine → Execution
- Performance tests: Builder overhead, memory usage
- Example validation: All 11 examples work
- Migration guide validation
- Edge case testing
- Concurrency testing
- Error message quality review
 - Docs readiness review: SDK section appears in local docs build once applied on docs site repo; cross-links validated

### Deliverables
- ✅ Complete GoDoc for all packages
- ✅ Comprehensive `sdk/README.md`
- ✅ 11 fully documented examples
- ✅ Troubleshooting guide
- ✅ Performance benchmarks
- ✅ Integration test suite
- ✅ Migration guide validation
- ✅ API reference documentation

### Validation
```bash
# Generate docs
go doc -all ./sdk/...

# Run all tests
go test -cover ./sdk/...

# Run all examples
for f in sdk/examples/*.go; do go run "$f"; done

# Performance benchmarks
go test -bench=. ./sdk/...

# Integration tests
go test ./sdk/compozy/... -tags=integration
```

### PR #4: Documentation & Polish
**Title:** Phase 4: Complete documentation and polish
**Files Changed:** 30-40 files (mostly updated)
**Lines of Code:** ~2,000-3,000 LOC (mostly docs)
**Breakdown:**
- Documentation: 1,500 LOC
- Tests: 800 LOC
- Examples updates: 500 LOC

**Test Coverage:** 95%+ overall
**Review Focus:** Documentation quality, example completeness
**Estimated Review Time:** 4-5 hours

---

## Testing Strategy

### Unit Tests
- **Coverage Target:** 95%+ for all builder packages
- **Pattern:** Table-driven tests with comprehensive cases
- **Context:** All tests use `t.Context()` instead of `context.Background()`

### Integration Tests
- **SDK → Engine Integration:** Test full pipeline
- **Resource Registration:** Validate resource store integration
- **Workflow Execution:** End-to-end execution tests
- **Error Handling:** BuildError aggregation tests

### Performance Tests
- **Builder Overhead:** <1ms per method call
- **Build() Latency:** <10ms for typical workflow
- **Memory Usage:** <100KB per builder instance
- **Benchmarks:** Go benchmark suite

### Concurrency Tests
- **Thread Safety:** Document non-thread-safe builders
- **Concurrent Execution:** Test parallel workflow execution
- **Race Detector:** Run with `-race` flag

---

## CI/CD Configuration

### GitHub Actions Workflow

```yaml
name: Compozy SDK CI

on: [push, pull_request]

jobs:
  test-workspace:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.2'
      
      - name: Initialize Go Workspace
        run: |
          go work init . ./sdk
          go work sync
      
      - name: Lint All Code
        run: golangci-lint run ./...
      
      - name: Test Main Module
        run: go test -race -cover ./...
      
      - name: Test SDK
        run: cd sdk && go test -race -cover ./...
      
      - name: Test Examples
        run: |
          cd sdk/examples
          for f in *.go; do
            echo "Testing $f..."
            go run "$f" || exit 1
          done
      
      - name: Performance Benchmarks
        run: cd sdk && go test -bench=. -benchmem ./...
      
      - name: Integration Tests
        run: cd sdk && go test -tags=integration ./compozy/...
```

---

## Version Compatibility Strategy

### SDK Versioning

| SDK Version | Engine Version | Compatibility | Notes |
|-------------|---------------|---------------|-------|
| sdk v0.1.0   | v1.0.0+       | ✅ Compatible | MVP release |
| sdk v0.2.0   | v1.1.0+       | ✅ Compatible | Full features |
| sdk v1.0.0   | sdk.0.0+       | ✅ Compatible | Stable release |

### Breaking Change Policy
- **Minor versions (sdk.X.0):** New features, backward compatible
- **Patch versions (sdk.0.X):** Bug fixes only
- **Major versions (v3.0.0):** Breaking changes allowed (rare)

### Deprecation Policy
- Deprecated features: 6-month warning period
- Documentation: Clear migration paths
- Support: 2 major versions maintained

---

## Risk Management

### Implementation Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| **Phase 0 prototype fails** | MEDIUM | CRITICAL | Revisit architecture, extend Phase 0 |
| **Integration layer too complex** | MEDIUM | HIGH | Simplify SDK → Engine mapping |
| **Context pattern breaks existing code** | LOW | MEDIUM | Isolated to sdk, no engine changes |
| **Task types mismatch** | LOW | HIGH | Phase 0 validates alignment |
| **Performance issues** | LOW | MEDIUM | Performance testing in Phase 4 |

### Mitigation Strategies

1. **Phase 0 Gate:** Do not proceed past Phase 0 if prototype fails
2. **Incremental Rollout:** Each phase is independently valuable
3. **Rollback Plan:** Clear rollback procedures for each PR
4. **Continuous Testing:** CI/CD on every commit
5. **Early Feedback:** Share examples with users in Week 4

---

## Resource Requirements

### Team Composition
- **1 Lead Developer** (architecture, integration layer, reviews)
- **1-2 Supporting Developers** (builders, tests, examples)
- **1 Technical Writer** (documentation, migration guide)

### Time Allocation
- **Phase 0:** 1 week (full-time, lead developer)
- **Phase 1:** 2 weeks (2-3 developers)
- **Phase 2:** 2 weeks (2-3 developers)
- **Phase 3:** 2 weeks (2-3 developers)
- **Phase 4:** 2 weeks (2-3 developers + tech writer)
- **Beta Testing:** 1+ weeks (community feedback)

### Infrastructure
- GitHub Actions for CI/CD
- Test database (PostgreSQL, Redis)
- Temporal server for integration tests

---

## Success Metrics

### Development Metrics
- ✅ All 30 builders implemented
- ✅ 95%+ test coverage
- ✅ All 11 examples working
- ✅ Zero circular dependencies
- ✅ Context-first pattern throughout
- ✅ BuildError aggregation working
- ✅ Full engine task type coverage (9 types)

### Quality Metrics
- ✅ GoDoc for all public APIs
- ✅ Performance benchmarks passing
- ✅ Integration tests passing
- ✅ All lints passing
- ✅ No race conditions detected
- ✅ Error messages are clear and actionable

### User Metrics (Post-Release)
- 10+ early adopters using SDK
- 5+ community contributions
- Migration guide validated by users
- Positive feedback on builder API
- Performance within 5% of YAML approach

---

## Release Checklist

### Pre-Release (sdk v0.1.0 MVP)
- [ ] All Phase 1-4 tasks complete
- [ ] All tests passing (unit + integration)
- [ ] All examples working
- [ ] Documentation complete
- [ ] Migration guide validated
- [ ] Performance benchmarks passing
- [ ] Security review complete
- [ ] Release notes written

### Release Process
1. Tag release: `git tag sdk/v0.1.0`
2. Push tags: `git push --tags`
3. Publish release notes on GitHub
4. Update documentation site
5. Announce on community channels
6. Monitor feedback and issues

### Post-Release
- Monitor GitHub issues
- Collect user feedback
- Plan v0.2.0 improvements
- Update migration guide based on feedback

---

## Summary

### Updated Timeline

| Phase | Duration | Deliverables | Status |
|-------|---------|--------------|--------|
| Phase 0 | 1 week | Prototype & validation | ⏳ Pending |
| Phase 1 | 2 weeks | Core builders + integration | ⏳ Pending |
| Phase 2 | 2 weeks | Knowledge, memory, tools, schema | ⏳ Pending |
| Phase 3 | 2 weeks | MCP, runtime, schedule, monitoring | ⏳ Pending |
| Phase 4 | 2 weeks | Documentation & polish | ⏳ Pending |
| **Total** | **9 weeks** | **sdk v0.1.0 MVP** | ⏳ Pending |

### Key Changes from v1.0

1. ✅ **Phase 0 added** - Mandatory prototype to validate architecture
2. ✅ **Task builders** - All 9 types (not 6)
3. ✅ **Context-first** - Throughout all builders
4. ✅ **Full features** - Memory, MCP, monitoring expanded
5. ✅ **Native tools** - Integrated in runtime
6. ✅ **Error handling** - BuildError infrastructure
7. ✅ **Testing strategy** - Integration tests added
8. ✅ **Version compatibility** - Clear compatibility matrix

---

**End of Implementation Plan**

**Status:** ✅ Complete (All P0, P1, P2 issues addressed)
**Next Document:** 05-examples.md
