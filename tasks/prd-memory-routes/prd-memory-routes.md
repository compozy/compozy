# PRD: Memory Operations Service Consolidation

## Executive Summary

**Problem**: Critical code duplication and architectural inconsistency in memory operations between `task/uc/exec_memory_operation.go` (sophisticated, transactional) and `memory/uc/*.go` files (basic, non-atomic). This creates 70%+ duplicated logic, maintenance burden, and data integrity risks.

**Solution**: Create a centralized `memory/service` package that consolidates all memory operations logic, leveraging the superior transaction handling from `exec_memory_operation.go` while eliminating duplication.

**Impact**: Eliminates tech debt, ensures atomic operations across all code paths, reduces maintenance overhead, and provides a single source of truth for memory operations.

## Current State Analysis

### Architecture Issues Identified

1. **Critical Logic Duplication (70%+ duplication)**

    - Memory instance retrieval patterns repeated across 6+ files
    - Validation logic (`ValidateMemoryRef`, `ValidateKey`, `ValidateRawMessages`) scattered
    - Message conversion functions duplicated (`convertToLLMMessages` vs `ConvertToLLMMessages`)

2. **Transaction Safety Inconsistency**

    - `exec_memory_operation.go`: Advanced `MemoryTransaction` with atomic rollback
    - `memory/uc/*.go`: Basic error handling, non-atomic operations, potential data corruption

3. **Interface Underutilization**

    - Sophisticated `AtomicOperations` interface exists but not used in `memory/uc`
    - Falls back to unsafe manual operations (clear + loop append)

4. **Feature Fragmentation**
    - Template resolution only available in task execution path
    - Advanced features not accessible to API operations

### Files Affected

**Source of Truth**: `/engine/task/uc/exec_memory_operation.go`

- ✅ MemoryTransaction class with rollback
- ✅ Template resolution capabilities
- ✅ Comprehensive error handling
- ✅ All 8 operation types supported

**Duplicated Logic**: `/engine/memory/uc/`

- ❌ `append_memory.go` - Non-atomic append operations
- ❌ `write_memory.go` - Unsafe fallback for non-atomic writes
- ❌ `delete_memory.go` - Basic delete without transaction safety
- ❌ `read_memory.go` - Simple read with duplicated validation
- ❌ `clear_memory.go` - Basic clear operation
- ❌ `flush_memory.go` - Basic flush without advanced controls

## Solution Design

### New Service Architecture

```
engine/memory/service/
├── operations.go          # Main service interface and implementation
├── operations_test.go     # Comprehensive test coverage
├── transaction.go         # MemoryTransaction logic (from exec_memory_operation.go)
├── validation.go          # Consolidated validation functions
├── conversion.go          # Message conversion utilities
└── types.go              # Service-specific types and interfaces
```

### Service Interface Design

```go
type MemoryOperationsService interface {
    // Core operations with atomic guarantees
    Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error)
    Write(ctx context.Context, req *WriteRequest) (*WriteResponse, error)
    Append(ctx context.Context, req *AppendRequest) (*AppendResponse, error)
    Delete(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error)

    // Advanced operations
    Flush(ctx context.Context, req *FlushRequest) (*FlushResponse, error)
    Clear(ctx context.Context, req *ClearRequest) (*ClearResponse, error)
    Health(ctx context.Context, req *HealthRequest) (*HealthResponse, error)
    Stats(ctx context.Context, req *StatsRequest) (*StatsResponse, error)
}
```

### Key Features

1. **Atomic Operations**: All write operations use `MemoryTransaction` with rollback
2. **Template Support**: Maintains payload resolution for workflow integration
3. **Validation Consolidation**: Single source for all validation logic
4. **Interface Leveraging**: Built around `AtomicOperations`, `FlushableMemory` interfaces
5. **Backward Compatibility**: Simple interface for basic use cases, advanced options available

## Implementation Plan

### Phase 1: Core Service Creation (High Priority)

1. **Create Memory Service Package**

    - Location: `/engine/memory/service/`
    - Extract and consolidate logic from `exec_memory_operation.go`

2. **Copy Transaction Logic**

    - Migrate `MemoryTransaction` struct and methods
    - Preserve atomic operation capabilities with rollback

3. **Consolidate Validation**

    - Move all validation functions to single location
    - Ensure consistent validation across all operations

4. **Create Comprehensive Tests**

    - Test all 8 operation types
    - Cover atomic and non-atomic scenarios
    - Test error handling and rollback

5. **Verify Completeness**
    - Ensure all operations from `exec_memory_operation.go` are available
    - Validate test coverage is comprehensive

### Phase 2: Service Integration (Medium Priority)

6. **Refactor Task Execution Path**

    - Update `exec_memory_operation.go` to use new service
    - Maintain template resolution and workflow context

7. **Replace Memory UC Files** (One by one)
    - `append_memory.go` → Service client
    - `write_memory.go` → Service client
    - `delete_memory.go` → Service client
    - `read_memory.go` → Service client
    - `clear_memory.go` → Service client
    - `flush_memory.go` → Service client

### Phase 3: Cleanup and Optimization

8. **Remove Duplicated Code**

    - Delete old implementations once service is integrated
    - Remove unused validation functions

9. **Performance Optimization**

    - Leverage cached metadata operations
    - Optimize common operation patterns

10. **Documentation Update**
    - Update API documentation
    - Add service usage examples

## Success Criteria

### Technical Goals

- ✅ **Zero Code Duplication**: No repeated memory operation logic
- ✅ **Atomic Operations**: All write operations are transaction-safe
- ✅ **Single Source of Truth**: One implementation for all memory operations
- ✅ **Interface Compliance**: Proper use of `AtomicOperations`, `FlushableMemory`
- ✅ **Test Coverage**: >95% coverage for all service operations

### Quality Metrics

- **Maintainability**: Changes require updates in only one location
- **Reliability**: Atomic operations prevent data corruption
- **Performance**: O(1) metadata operations, optimized common paths
- **Developer Experience**: Clear, consistent API for memory operations

## Risk Mitigation

### Implementation Risks

1. **Breaking Changes**: Mitigated by maintaining interface compatibility
2. **Performance Regression**: Mitigated by comprehensive benchmarking
3. **Data Corruption**: Mitigated by extensive testing of transaction logic
4. **Integration Complexity**: Mitigated by phased rollout approach

### Rollback Plan

- Keep existing implementations until service is fully tested
- Feature flags to switch between old and new implementations
- Comprehensive monitoring during migration

## Timeline

**Phase 1**: 1-2 weeks (Core service creation and testing)
**Phase 2**: 2-3 weeks (Integration and replacement)  
**Phase 3**: 1 week (Cleanup and documentation)

**Total**: 4-6 weeks for complete consolidation

## Dependencies

- Existing `memory/core` interfaces must remain stable
- Template engine functionality must be preserved
- All current API contracts must be maintained

## Validation Approach

1. **Unit Tests**: Comprehensive coverage of all operations
2. **Integration Tests**: End-to-end workflow validation
3. **Performance Tests**: Ensure no regression in operation speed
4. **Stress Tests**: Validate atomic operations under high concurrency

---

**Status**: Planning Phase
**Last Updated**: 2025-01-18
**Next Review**: After Phase 1 completion
