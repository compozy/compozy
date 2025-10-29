## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/resources</domain>
<type>testing</type>
<scope>integration_testing</scope>
<complexity>medium</complexity>
<dependencies>miniredis|cache_adapter</dependencies>
</task_context>

# Task 5.0: Resource Store Integration

## Overview

Verify that the resource store works correctly with miniredis backend, ensuring full compatibility with atomic operations, optimistic locking, ETag consistency, and concurrent resource updates. This task validates that miniredis provides identical behavior to external Redis for all resource store operations including TxPipeline atomicity, Lua script-based locking (PutIfMatch), and watch notifications via Pub/Sub.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- Resource store MUST work identically with miniredis and external Redis
- TxPipeline operations MUST maintain atomicity guarantees (value + etag together)
- Optimistic locking (PutIfMatch) MUST function via native Lua scripts
- ETag consistency MUST be maintained across all operations
- Concurrent resource updates MUST handle race conditions correctly
- Watch notifications MUST work via native Redis Pub/Sub
- All tests MUST use t.Context() (never context.Background())
- All tests MUST follow "Should..." naming convention with testify assertions
- MUST use real miniredis (no mocks) with temp directories
</requirements>

## Subtasks

- [x] 5.1 Create test/integration/standalone/resource_store_test.go with test suite
- [x] 5.2 Verify TxPipeline atomic operations (value + etag stored atomically)
- [x] 5.3 Verify optimistic locking via Lua scripts (PutIfMatch works natively)
- [x] 5.4 Verify ETag consistency across all resource operations
- [x] 5.5 Verify concurrent resource update handling (race conditions)
- [x] 5.6 Verify watch notifications via Pub/Sub
- [x] 5.7 Add test fixtures and helpers in test/helpers/standalone.go
- [x] 5.8 Run full test suite and ensure >80% coverage for integration code

## Implementation Details

This task verifies that the resource store, which relies on Redis TxPipeline for atomic multi-key operations and Lua scripts for optimistic locking, works identically with miniredis.

### Relevant Files

- `engine/resources/redis_store.go` - Resource store implementation using cache.RedisInterface
- `engine/infra/cache/miniredis_standalone.go` - MiniredisStandalone wrapper (created in Task 2.0)
- `engine/infra/cache/mod.go` - Mode-aware factory (updated in Task 3.0)
- `test/integration/standalone/resource_store_test.go` - NEW: Integration tests for resource store
- `test/helpers/standalone.go` - NEW: Test environment helpers

### Dependent Files

- `engine/infra/cache/redis.go` - Cache interface used by resource store
- `pkg/config/config.go` - Configuration structs for mode selection
- `pkg/config/resolver.go` - Mode resolution logic (created in Task 1.0)

### Key Technical Details from Tech Spec

**Resource Store Uses**:
- TxPipeline for atomic operations: Store resource value and ETag together atomically
- Lua scripts for optimistic locking: PutIfMatch checks ETag before updating
- Pub/Sub for watch notifications: Notify subscribers when resources change

**Miniredis Compatibility**:
- Miniredis natively supports TxPipeline operations
- Miniredis natively executes Lua scripts (no emulation needed)
- Miniredis natively supports Redis Pub/Sub
- Zero consumer code changes required

## Deliverables

- `test/integration/standalone/resource_store_test.go` - Full integration test suite
- Test fixtures for resources in `test/fixtures/standalone/`
- Helper functions in `test/helpers/standalone.go` for resource store testing
- Updated CI pipeline in `.github/workflows/test.yml` (if needed)
- Documentation of any discovered edge cases or limitations

## Tests

Integration tests mapped from `_tests.md`:

- [ ] Should store and retrieve resources atomically via TxPipeline
  - Test: Create resource, verify value and ETag stored together
  - Test: Update resource, verify old value not visible before ETag updated

- [ ] Should support optimistic locking via PutIfMatch Lua script
  - Test: Update with correct ETag succeeds
  - Test: Update with incorrect ETag fails
  - Test: Concurrent updates with stale ETag properly rejected

- [ ] Should maintain ETag consistency across operations
  - Test: ETag changes on every resource update
  - Test: ETag retrieved matches last stored ETag
  - Test: Concurrent reads see consistent ETags

- [ ] Should handle concurrent resource updates correctly
  - Test: Multiple goroutines updating same resource
  - Test: Last writer wins with proper ETag verification
  - Test: No lost updates due to race conditions

- [ ] Should publish watch notifications via Pub/Sub
  - Test: Subscribe to resource watch channel
  - Test: Publish notification on resource update
  - Test: Multiple subscribers receive notifications
  - Test: Pattern subscriptions work correctly

- [ ] Should handle error cases gracefully
  - Test: Missing resource returns proper error
  - Test: ETag mismatch returns conflict error
  - Test: Pub/Sub connection failures handled

### Test Structure Example

```go
// test/integration/standalone/resource_store_test.go

func TestResourceStore_MiniredisCompatibility(t *testing.T) {
    t.Run("Should support TxPipeline atomic operations", func(t *testing.T) {
        ctx := t.Context()
        env := setupResourceStoreWithMiniredis(ctx, t)
        defer env.Cleanup()

        resource := generateTestResource()

        // Store resource (atomic: value + etag)
        err := env.Store.Put(ctx, resource)
        require.NoError(t, err)

        // Retrieve and verify atomicity
        retrieved, err := env.Store.Get(ctx, resource.ID)
        require.NoError(t, err)
        assert.Equal(t, resource.Value, retrieved.Value)
        assert.Equal(t, resource.ETag, retrieved.ETag)
    })

    t.Run("Should handle optimistic locking via PutIfMatch", func(t *testing.T) {
        ctx := t.Context()
        env := setupResourceStoreWithMiniredis(ctx, t)
        defer env.Cleanup()

        // Create initial resource
        resource := generateTestResource()
        err := env.Store.Put(ctx, resource)
        require.NoError(t, err)

        // Update with correct ETag should succeed
        resource.Value = "updated"
        err = env.Store.PutIfMatch(ctx, resource, resource.ETag)
        require.NoError(t, err)

        // Update with stale ETag should fail
        staleResource := resource
        staleResource.Value = "should-fail"
        err = env.Store.PutIfMatch(ctx, staleResource, "stale-etag")
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "conflict")
    })

    t.Run("Should handle concurrent resource updates", func(t *testing.T) {
        ctx := t.Context()
        env := setupResourceStoreWithMiniredis(ctx, t)
        defer env.Cleanup()

        // Create initial resource
        resource := generateTestResource()
        err := env.Store.Put(ctx, resource)
        require.NoError(t, err)

        // Concurrent updates
        var wg sync.WaitGroup
        errors := make([]error, 10)
        for i := 0; i < 10; i++ {
            wg.Add(1)
            go func(idx int) {
                defer wg.Done()
                r := resource
                r.Value = fmt.Sprintf("update-%d", idx)
                errors[idx] = env.Store.PutIfMatch(ctx, r, resource.ETag)
            }(i)
        }
        wg.Wait()

        // Only one update should succeed
        successCount := 0
        for _, err := range errors {
            if err == nil {
                successCount++
            }
        }
        assert.Equal(t, 1, successCount, "Only one concurrent update should succeed")
    })

    t.Run("Should publish watch notifications via Pub/Sub", func(t *testing.T) {
        ctx := t.Context()
        env := setupResourceStoreWithMiniredis(ctx, t)
        defer env.Cleanup()

        // Subscribe to notifications
        notifications := make(chan string, 10)
        err := env.Store.Watch(ctx, "resource:*", notifications)
        require.NoError(t, err)

        // Update resource
        resource := generateTestResource()
        err = env.Store.Put(ctx, resource)
        require.NoError(t, err)

        // Verify notification received
        select {
        case notif := <-notifications:
            assert.Contains(t, notif, resource.ID)
        case <-time.After(5 * time.Second):
            t.Fatal("Watch notification not received")
        }
    })
}

// Helper functions
func setupResourceStoreWithMiniredis(ctx context.Context, t *testing.T) *ResourceStoreTestEnv {
    // Setup miniredis via mode-aware factory
    // Create resource store with miniredis client
    // Return test environment with cleanup
}

func generateTestResource() *resources.Resource {
    // Generate sample resource with ID, value, ETag
}
```

## Success Criteria

- [ ] All integration tests pass with miniredis backend
- [ ] TxPipeline operations maintain atomicity (value + etag together)
- [ ] Optimistic locking (PutIfMatch) works via native Lua scripts
- [ ] ETag consistency maintained across all operations
- [ ] Concurrent updates handle race conditions correctly
- [ ] Watch notifications work via native Pub/Sub
- [ ] Test coverage >80% for integration code
- [ ] `make test` passes with no failures
- [ ] All tests use `t.Context()` (no `context.Background()`)
- [ ] All tests follow "Should..." naming convention
- [ ] Test output clearly shows miniredis backend being tested
- [ ] No behavioral differences between miniredis and external Redis
- [ ] Documentation updated with any edge cases discovered

## Dependencies

- **Blocks**: Task 9.0 (End-to-End Workflow Tests) - requires resource store validation
- **Blocked By**: Task 3.0 (Mode-Aware Cache Factory) - requires factory to create miniredis clients

## Estimated Effort

**Size**: M (Medium - 1 day)

**Breakdown**:
- Test suite creation: 3 hours
- TxPipeline atomicity tests: 2 hours
- Optimistic locking tests: 2 hours
- Concurrent update tests: 2 hours
- Watch notification tests: 1 hour
- Edge case testing and documentation: 2 hours

**Total**: ~12 hours (1 day)

## Risk Assessment

**Risks**:
1. TxPipeline behavior differences between miniredis and Redis
2. Lua script execution differences
3. Pub/Sub notification delivery differences
4. Race condition test flakiness

**Mitigations**:
1. Run identical test suite against both miniredis and external Redis (contract tests)
2. Use deterministic test data and proper synchronization
3. Add retry logic for Pub/Sub tests with reasonable timeouts
4. Document any discovered behavioral differences

## Validation Checklist

Before marking this task complete:

- [ ] All subtasks completed
- [ ] All tests in "Tests" section implemented and passing
- [ ] Test coverage verified (>80%)
- [ ] `make lint` passes with no warnings
- [ ] `make test` passes with no failures
- [ ] Integration tests added to CI pipeline
- [ ] Code follows `.cursor/rules/test-standards.mdc`
- [ ] All uses of context follow patterns (t.Context() in tests)
- [ ] Test fixtures and helpers properly organized
- [ ] Documentation updated if edge cases discovered
