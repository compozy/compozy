# Architectural Review: Eviction Policy & Flush Strategy Decoupling

**Date**: 2025-06-25  
**Review Type**: Architectural Restructure  
**Severity**: Critical  
**Status**: Approved for Implementation

## Executive Summary

This review documents the complete architectural restructure required to decouple eviction policies from flush strategies in the memory management system. The analysis confirms critical architectural violations including Single Responsibility Principle (SRP) violations, configuration coupling, and 517 lines of duplicate code across two separate systems.

## Problem Analysis

### Current Architecture Violations

#### 1. CRITICAL: Single Responsibility Principle Violation

- **FlushStrategy** (`strategies/priority_strategy.go`): 360 lines implementing message selection logic
- **EvictionPolicy** (`eviction/priority_policy.go`): 157 lines implementing duplicate message selection logic
- **Impact**: Both components perform identical responsibilities with different interfaces

#### 2. CRITICAL: Configuration Coupling

- `FlushingStrategyConfig.PriorityKeywords` consumed by eviction policies instead of flush strategies
- `CreatePolicyWithConfig()` directly couples unrelated configuration domains
- **Impact**: Conceptual confusion and tight coupling between unrelated components

#### 3. HIGH: Code Duplication

- 517 total lines of overlapping priority logic across two systems
- Duplicate priority enums, keyword matching, and selection algorithms
- **Impact**: Maintenance nightmare requiring dual updates for single functionality

#### 4. HIGH: Interface Convergence

- Two interfaces (`FlushStrategy` and `EvictionPolicy`) solving same problem with different approaches
- **Impact**: Poor abstraction indicating architectural design failure

## Proposed Solution Architecture

### Clean Separation of Concerns

#### FlushStrategy Responsibilities

- **WHEN to flush**: Threshold detection and timing decisions
- **HOW MUCH to flush**: Calculate target message/token counts
- **Interface**:
    ```go
    type FlushStrategy interface {
        ShouldFlush(tokenCount, messageCount int, config *core.Resource) bool
        CalculateFlushTarget(tokenCount, messageCount int, config *core.Resource) FlushTarget
        GetType() core.FlushingStrategyType
    }
    ```

#### EvictionPolicy Responsibilities

- **WHICH messages to evict**: Selection logic based on priority, age, access patterns
- **Interface**:
    ```go
    type EvictionPolicy interface {
        SelectMessagesToEvict(messages []llm.Message, targetCount int) []llm.Message
        GetType() string
    }
    ```

### Configuration Decoupling

#### New Configuration Structure

```go
// Separate, focused configurations
type FlushingStrategyConfig struct {
    Type               FlushingStrategyType `yaml:"type" json:"type"`
    SummarizeThreshold float64             `yaml:"summarize_threshold,omitempty" json:"summarize_threshold,omitempty"`
    // No PriorityKeywords - removed coupling
}

type EvictionPolicyConfig struct {
    Type             EvictionPolicyType `yaml:"type" json:"type"`
    PriorityKeywords []string          `yaml:"priority_keywords,omitempty" json:"priority_keywords,omitempty"`
}
```

## Implementation Plan

### Phase 1: Configuration Foundation (Week 1)

1. **Remove** `PriorityKeywords` from `FlushingStrategyConfig`
2. **Create** `EvictionPolicyConfig` with proper fields
3. **Update** factory patterns to use dedicated configurations
4. **Rewrite** instance builder with clean separation

### Phase 2: Strategy Rewrite (Week 2)

1. **Delete** `strategies/priority_strategy.go` (360 lines eliminated)
2. **Rewrite** all flush strategies to only handle timing/thresholds
3. **Consolidate** ALL priority logic in `eviction/priority_policy.go`
4. **Update** memory instance integration for clean workflow

### Phase 3: Builder Restructure (Week 2-3)

1. **Separate** configuration creation methods
2. **Remove** all cross-configuration coupling
3. **Update** resource configuration structure
4. **Implement** proper dependency injection

### Phase 4: Comprehensive Testing (Week 3-4)

1. **Rewrite** all tests with `t.Run("Should...")` pattern
2. **Use** `testify/mock` for all external dependencies
3. **Achieve** >85% coverage on business logic
4. **Add** integration and performance test suites

### Phase 5: System Validation (Week 4)

1. **Architecture validation** tests for dependency direction
2. **End-to-end** scenario testing
3. **Performance** benchmarking and validation
4. **Documentation** and review completion

## Compliance with Project Standards

### Architecture Standards

- ✅ **SRP**: Clear single responsibility per component
- ✅ **DIP**: Dependencies on abstractions, not concretions
- ✅ **ISP**: Small, focused interfaces
- ✅ **Clean Architecture**: Proper layer separation

### Go Patterns

- ✅ **Factory Pattern**: Clean, extensible creation
- ✅ **Constructor Injection**: All dependencies injected
- ✅ **Interface Composition**: Minimal, composable interfaces

### Testing Standards

- ✅ **Mandatory t.Run()**: All tests use "Should..." pattern
- ✅ **Testify Usage**: Use `testify/mock` and assertions only
- ✅ **Coverage**: >85% for business logic packages
- ✅ **No Suite Patterns**: Direct test functions only

## Risk Assessment

### Technical Risks

- **LOW**: No existing users - complete breaking changes acceptable
- **LOW**: Comprehensive test coverage prevents regressions
- **MEDIUM**: Performance validation required for flush operations

### Mitigation Strategies

- **Incremental implementation** with validation at each phase
- **Performance benchmarking** to ensure no regressions
- **Architecture validation tests** to prevent future violations

## Success Metrics

1. **Code Quality**: Eliminate 517 lines of duplicate logic
2. **Architecture**: Zero coupling violations between components
3. **Performance**: No degradation in flush operations
4. **Testing**: >85% coverage on all modified components
5. **Maintainability**: Single source of truth for priority logic

## Approval Status

**✅ APPROVED** for implementation following the phased approach outlined above.

**Conditions**:

- Follow all project standards defined in `.cursor/rules/`
- Achieve >85% test coverage before considering any phase complete
- Performance validation required before production deployment
- Architecture compliance tests must pass for final approval

## Implementation Owner

This architectural restructure addresses critical technical debt and establishes a foundation for maintainable, scalable memory management architecture following SOLID principles and clean code practices.
