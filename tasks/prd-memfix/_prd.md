# PRD: Memory Task Integration Fix

## Executive Summary

**Problem**: Memory tasks (`task: memory`) in Compozy workflows are non-functional due to incomplete integration with the Task2 execution framework, preventing critical AI agent memory features from working.

**Root Cause**: Missing Task2 factory implementations for memory task normalizer and response handler components.

**Solution**: Complete the Task2 factory integration by implementing memory-specific normalizer and response handler classes following established project patterns.

**Impact**: Unblocks the entire memory task feature, enabling AI agent conversation memory, context persistence, and advanced memory operations.

**Effort**: Low (2-3 days) - High-impact integration fix with clear implementation path.

## Problem Statement

### Current State

Memory tasks are completely non-functional in Compozy workflows, failing with "unsupported task type" errors despite having:

- Complete memory architecture with manager, service layer, and operations
- Proper task routing and worker registration
- Comprehensive memory operations (read, write, append, delete, flush, health, clear, stats)
- Redis-based persistence and distributed locking mechanisms

### Specific Failure Points

1. **exec_memory.go:69**: `normalizer, err := a.task2Factory.CreateNormalizer(task.TaskTypeMemory)` fails
2. **exec_memory.go:104**: `handler, err := a.task2Factory.CreateResponseHandler(task.TaskTypeMemory)` fails

Both fail with "unsupported task type: memory" errors in factory.go default cases.

### Business Impact

- **Blocked Feature**: AI agent memory capabilities are completely unusable
- **User Experience**: Workflows with memory tasks fail entirely
- **Development Impact**: Memory-dependent features cannot be tested or deployed
- **Strategic Risk**: Core AI agent functionality unavailable for production use

## Root Cause Analysis

### Investigation Summary

Comprehensive analysis using GoGraph tools, zen MCP analysis with Gemini Pro, and systematic code examination revealed:

#### Primary Issue: Incomplete Task2 Factory Integration

**File**: `/engine/task2/factory.go`

- **Lines 64-84**: `CreateNormalizer()` switch statement missing `case task.TaskTypeMemory:`
- **Lines 119-154**: `CreateResponseHandler()` switch statement missing `case task.TaskTypeMemory:`
- **Missing Directory**: `/engine/task2/memory/` package doesn't exist

#### Supporting Evidence

- **Test Confirmation**: `handler_factory_concurrent_test.go:49,161` shows memory task type exists and is tested
- **Architecture Completeness**: Memory system is fully implemented and architecturally sound
- **Pattern Consistency**: All other task types follow normalizer + response handler pattern

#### Secondary Issue: ResourceRegistry Configuration

**File**: `/engine/worker/mod.go:305-308`

```go
if config.ResourceRegistry == nil {
    log.Warn("Resource registry not provided, memory features will be disabled")
    return nil, nil
}
```

Memory manager requires proper ResourceRegistry configuration via autoload.

## Solution Design

### Approach 1: Immediate Fix (Recommended)

**Timeline**: 1-2 days
**Effort**: Low
**Risk**: Minimal

Extend existing Task2 factory to support memory tasks using basic normalizer/handler patterns:

**factory.go Changes**:

```go
// In CreateNormalizer method (line ~81)
case task.TaskTypeMemory:
    return basic.NewNormalizer(f.templateEngine), nil

// In CreateResponseHandler method (line ~151)
case task.TaskTypeMemory:
    return basic.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
```

### Approach 2: Full Implementation (Future Enhancement)

**Timeline**: 3-5 days
**Effort**: Medium
**Risk**: Low

Create dedicated memory normalizer and response handler following project patterns:

1. Create `/engine/task2/memory/` package
2. Implement `memory.NewNormalizer()` and `memory.NewResponseHandler()`
3. Add factory cases to use dedicated implementations
4. Add comprehensive tests

### Architecture Validation

**Expert Analysis Score**: A- (90/100)

- **Scalability**: Excellent Redis-based horizontal scaling
- **Security**: Strong privacy framework with identified hardening opportunities
- **Performance**: Good caching with token counting optimization potential
- **Maintainability**: Clean separation of concerns and interface-driven design

## Implementation Plan

### Phase 1: Critical Fix (P0)

**Timeline**: 1-2 days

1. **Add Factory Cases** (30 minutes)
    - Modify `engine/task2/factory.go`
    - Add memory cases to both factory methods
    - Use basic normalizer/handler implementations

2. **Verify ResourceRegistry Setup** (2 hours)
    - Ensure autoload configuration includes memory resources
    - Validate memory resource discovery and loading
    - Test memory manager initialization

3. **Integration Testing** (4 hours)
    - Create simple memory task workflow test
    - Verify all memory operations work end-to-end
    - Test error handling and edge cases

4. **Documentation Update** (2 hours)
    - Update memory task usage documentation
    - Add configuration examples
    - Document limitations of basic implementation

### Phase 2: Security & Performance (P1)

**Timeline**: 3-5 days

1. **Template Security Hardening** (1 day)
    - Add template injection validation
    - Implement key sanitization for Redis
    - Add rate limiting for memory operations

2. **Performance Optimization** (2 days)
    - Implement batch token counting
    - Add memory pooling for high-throughput
    - Optimize Redis operations

3. **Enhanced Monitoring** (1 day)
    - Add memory operation metrics
    - Implement health check endpoints
    - Create operational dashboards

### Phase 3: Architecture Enhancement (P2)

**Timeline**: 3-4 days

1. **Dedicated Memory Components** (2 days)
    - Create `/engine/task2/memory/` package
    - Implement memory-specific normalizer
    - Implement memory-specific response handler

2. **Advanced Features** (2 days)
    - Enhanced validation for memory operations
    - Memory-specific error handling
    - Optimized normalization logic

## Testing Strategy

### Unit Tests

- Memory normalizer functionality
- Response handler processing
- Factory method coverage
- Error handling scenarios

### Integration Tests

- End-to-end memory task execution
- Multi-operation workflow testing
- Resource cleanup verification
- Concurrent access validation

### Performance Tests

- High-throughput memory operations
- Memory limit enforcement
- Token counting accuracy
- Redis connection pooling

## Risks & Mitigation

### Technical Risks

1. **ResourceRegistry Configuration**
    - **Risk**: Memory manager initialization failure
    - **Mitigation**: Validate autoload setup, provide clear error messages

2. **Template Security**
    - **Risk**: Injection vulnerabilities in key templates
    - **Mitigation**: Implement validation in Phase 2

3. **Performance Impact**
    - **Risk**: Token counting overhead in high-throughput scenarios
    - **Mitigation**: Batch optimization in Phase 2

### Process Risks

1. **Incomplete Integration Testing**
    - **Risk**: Edge cases not covered
    - **Mitigation**: Comprehensive test suite development

2. **Documentation Gaps**
    - **Risk**: User confusion about memory task configuration
    - **Mitigation**: Clear examples and troubleshooting guides

## Success Metrics

### Immediate Success (Phase 1)

- [ ] Memory tasks execute without "unsupported task type" errors
- [ ] All 8 memory operations (read, write, append, delete, flush, health, clear, stats) work
- [ ] Memory manager initializes successfully with proper ResourceRegistry
- [ ] Basic memory workflow completes end-to-end

### Long-term Success (Phase 2-3)

- [ ] Memory operations handle high-throughput scenarios (1000+ ops/sec)
- [ ] Template security validation prevents injection attacks
- [ ] Memory system scales horizontally with Redis clustering
- [ ] Comprehensive monitoring and alerting operational

## Technical Debt & Future Considerations

### Architectural Improvements

1. **New Task Type Onboarding Process**
    - Create formal checklist for adding task types
    - Automate factory integration verification
    - Standardize component creation patterns

2. **Memory System Enhancements**
    - Advanced privacy controls for regulatory compliance
    - Cross-region memory replication
    - Advanced analytics and memory usage insights

### Strategic Opportunities

1. **Memory-as-a-Service Architecture**
    - Extract memory system as standalone service
    - Enable external memory providers
    - Support for specialized memory backends (vector DBs, etc.)

2. **AI-Specific Optimizations**
    - Context-aware memory management
    - Semantic memory clustering
    - Automated memory lifecycle optimization

## Appendix

### Code References

- **Primary Failure**: `/engine/task2/factory.go:64-84, 119-154`
- **Memory Architecture**: `/engine/memory/` (complete implementation)
- **Task Execution**: `/engine/task/activities/exec_memory.go`
- **Worker Integration**: `/engine/worker/mod.go`

### Expert Analysis References

- **Scalability Assessment**: Redis-based horizontal scaling architecture
- **Security Analysis**: Privacy framework with ReDoS protection
- **Performance Profile**: I/O bound operations with optimization opportunities
- **Architecture Quality**: A- (90/100) enterprise-grade implementation

### Related Documentation

- **Project Architecture**: `.cursor/rules/architecture.mdc`
- **Go Patterns**: `.cursor/rules/go-patterns.mdc`
- **Memory Configuration**: Autoload resource discovery patterns
- **Testing Standards**: `.cursor/rules/testing-standards.mdc`
