## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>engine/infra/server</domain>
<type>testing</type>
<scope>integration_testing</scope>
<complexity>medium</complexity>
<dependencies>miniredis|pub_sub</dependencies>
</task_context>

# Task 6.0: Streaming & Pub/Sub Integration

## Overview

Verify that streaming and Pub/Sub functionality work correctly with miniredis backend, ensuring full compatibility with event publishing, pattern subscriptions, multiple subscribers, and event delivery reliability. This task validates that miniredis provides native Redis Pub/Sub support for workflow and task event notifications without emulation complexity.

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
- Streaming MUST work identically with miniredis and external Redis
- Pub/Sub MUST support publish/subscribe operations natively
- Pattern subscriptions (workflow:*, task:*) MUST work correctly
- Multiple concurrent subscribers MUST receive all events
- Event delivery MUST be reliable (no lost events)
- Native go-redis PubSub types MUST be used (no emulation)
- All tests MUST use t.Context() (never context.Background())
- All tests MUST follow "Should..." naming convention with testify assertions
- MUST use real miniredis (no mocks) with temp directories
</requirements>

## Subtasks

- [x] 6.1 Create test/integration/standalone/streaming_test.go with test suite
- [x] 6.2 Verify basic publish/subscribe functionality
- [x] 6.3 Verify pattern subscriptions (wildcard channels)
- [x] 6.4 Verify multiple subscribers receive events
- [x] 6.5 Verify event delivery reliability (no lost events)
- [x] 6.6 Verify subscription lifecycle (subscribe, unsubscribe, cleanup)
- [x] 6.7 Add test fixtures and event generators
- [x] 6.8 Run full test suite and ensure >80% coverage for integration code

## Implementation Details

This task verifies that the streaming/pub-sub functionality, which uses Redis Pub/Sub for real-time workflow and task event notifications, works identically with miniredis.

### Relevant Files

- `engine/infra/server/dependencies.go` - Sets up streaming/pub-sub connections
- `engine/infra/cache/miniredis_standalone.go` - MiniredisStandalone wrapper (created in Task 2.0)
- `engine/infra/cache/mod.go` - Mode-aware factory (updated in Task 3.0)
- `test/integration/standalone/streaming_test.go` - NEW: Integration tests for streaming
- `test/helpers/standalone.go` - NEW: Test environment helpers for pub-sub

### Dependent Files

- `engine/infra/cache/redis.go` - Cache interface with Pub/Sub methods
- `pkg/config/config.go` - Configuration structs for mode selection
- `pkg/config/resolver.go` - Mode resolution logic (created in Task 1.0)

### Key Technical Details from Tech Spec

**Streaming Features Use**:
- Redis Pub/Sub for real-time event notifications
- Pattern subscriptions for workflow/task events (e.g., `workflow:*`, `task:*`)
- Native go-redis PubSub types (no custom implementation)
- Multiple concurrent subscribers supported

**Miniredis Compatibility**:
- Miniredis natively supports Redis Pub/Sub protocol
- Pattern subscriptions work identically to external Redis
- Multiple subscribers work without emulation
- Zero consumer code changes required

## Deliverables

- `test/integration/standalone/streaming_test.go` - Full integration test suite
- Event generators and test fixtures in `test/fixtures/standalone/`
- Helper functions in `test/helpers/standalone.go` for pub-sub testing
- Updated CI pipeline in `.github/workflows/test.yml` (if needed)
- Documentation of any discovered edge cases or limitations

## Tests

Integration tests mapped from `_tests.md`:

- [ ] Should publish and subscribe to events
  - Test: Publish event to channel, verify subscriber receives it
  - Test: Multiple events published in sequence
  - Test: Event payload integrity maintained

- [ ] Should support pattern subscriptions (wildcards)
  - Test: Subscribe to `workflow:*` pattern
  - Test: Receive events for `workflow:123`, `workflow:456`, etc.
  - Test: Pattern subscriptions don't match unrelated channels

- [ ] Should support multiple concurrent subscribers
  - Test: Multiple subscribers to same channel
  - Test: All subscribers receive same events
  - Test: Subscribers don't interfere with each other

- [ ] Should deliver events reliably
  - Test: No events lost under normal conditions
  - Test: Events delivered in order published
  - Test: Large event payloads delivered correctly

- [ ] Should handle subscription lifecycle correctly
  - Test: Subscribe, receive events, unsubscribe cleanly
  - Test: Re-subscribe to same channel after unsubscribe
  - Test: Cleanup on context cancellation

- [ ] Should handle error cases gracefully
  - Test: Subscribe to invalid channel pattern
  - Test: Publish to channel with no subscribers (no error)
  - Test: Subscriber disconnection handling

### Test Structure Example

```go
// test/integration/standalone/streaming_test.go

func TestStreaming_MiniredisCompatibility(t *testing.T) {
    t.Run("Should publish and subscribe to events", func(t *testing.T) {
        ctx := t.Context()
        env := setupStreamingWithMiniredis(ctx, t)
        defer env.Cleanup()

        // Subscribe to channel
        events := make(chan string, 10)
        err := env.Subscribe(ctx, "test-channel", events)
        require.NoError(t, err)

        // Publish event
        testEvent := "test-event-payload"
        err = env.Publish(ctx, "test-channel", testEvent)
        require.NoError(t, err)

        // Verify event received
        select {
        case evt := <-events:
            assert.Equal(t, testEvent, evt)
        case <-time.After(5 * time.Second):
            t.Fatal("Event not received within timeout")
        }
    })

    t.Run("Should support pattern subscriptions", func(t *testing.T) {
        ctx := t.Context()
        env := setupStreamingWithMiniredis(ctx, t)
        defer env.Cleanup()

        // Subscribe to pattern
        events := make(chan string, 10)
        err := env.SubscribePattern(ctx, "workflow:*", events)
        require.NoError(t, err)

        // Publish to matching channels
        channels := []string{"workflow:123", "workflow:456", "workflow:789"}
        for _, ch := range channels {
            err = env.Publish(ctx, ch, "event-data")
            require.NoError(t, err)
        }

        // Verify all events received
        receivedCount := 0
        timeout := time.After(5 * time.Second)
        for receivedCount < len(channels) {
            select {
            case <-events:
                receivedCount++
            case <-timeout:
                t.Fatalf("Only received %d of %d events", receivedCount, len(channels))
            }
        }
        assert.Equal(t, len(channels), receivedCount)
    })

    t.Run("Should support multiple subscribers", func(t *testing.T) {
        ctx := t.Context()
        env := setupStreamingWithMiniredis(ctx, t)
        defer env.Cleanup()

        // Create multiple subscribers
        numSubscribers := 5
        subscribers := make([]chan string, numSubscribers)
        for i := 0; i < numSubscribers; i++ {
            subscribers[i] = make(chan string, 10)
            err := env.Subscribe(ctx, "broadcast-channel", subscribers[i])
            require.NoError(t, err)
        }

        // Publish event
        testEvent := "broadcast-event"
        err := env.Publish(ctx, "broadcast-channel", testEvent)
        require.NoError(t, err)

        // Verify all subscribers received event
        for i, sub := range subscribers {
            select {
            case evt := <-sub:
                assert.Equal(t, testEvent, evt, "Subscriber %d didn't receive event", i)
            case <-time.After(5 * time.Second):
                t.Fatalf("Subscriber %d didn't receive event", i)
            }
        }
    })

    t.Run("Should deliver events reliably", func(t *testing.T) {
        ctx := t.Context()
        env := setupStreamingWithMiniredis(ctx, t)
        defer env.Cleanup()

        // Subscribe
        events := make(chan string, 100)
        err := env.Subscribe(ctx, "test-channel", events)
        require.NoError(t, err)

        // Publish multiple events
        numEvents := 50
        for i := 0; i < numEvents; i++ {
            err = env.Publish(ctx, "test-channel", fmt.Sprintf("event-%d", i))
            require.NoError(t, err)
        }

        // Verify all events received
        receivedCount := 0
        timeout := time.After(10 * time.Second)
        for receivedCount < numEvents {
            select {
            case <-events:
                receivedCount++
            case <-timeout:
                t.Fatalf("Only received %d of %d events", receivedCount, numEvents)
            }
        }
        assert.Equal(t, numEvents, receivedCount, "Some events were lost")
    })

    t.Run("Should handle subscription lifecycle", func(t *testing.T) {
        ctx := t.Context()
        env := setupStreamingWithMiniredis(ctx, t)
        defer env.Cleanup()

        // Subscribe
        events := make(chan string, 10)
        sub := env.SubscribeRaw(ctx, "test-channel")

        // Receive event
        err := env.Publish(ctx, "test-channel", "event-1")
        require.NoError(t, err)

        msg, err := sub.ReceiveMessage(ctx)
        require.NoError(t, err)
        assert.Equal(t, "event-1", msg.Payload)

        // Unsubscribe
        err = sub.Unsubscribe(ctx, "test-channel")
        require.NoError(t, err)

        // Close
        err = sub.Close()
        require.NoError(t, err)

        // Re-subscribe should work
        events2 := make(chan string, 10)
        err = env.Subscribe(ctx, "test-channel", events2)
        require.NoError(t, err)

        err = env.Publish(ctx, "test-channel", "event-2")
        require.NoError(t, err)

        select {
        case evt := <-events2:
            assert.Equal(t, "event-2", evt)
        case <-time.After(5 * time.Second):
            t.Fatal("Event not received after re-subscribe")
        }
    })
}

// Helper functions
func setupStreamingWithMiniredis(ctx context.Context, t *testing.T) *StreamingTestEnv {
    // Setup miniredis via mode-aware factory
    // Create pub-sub connections
    // Return test environment with cleanup
}
```

## Success Criteria

- [x] All integration tests pass with miniredis backend
- [x] Basic publish/subscribe functionality works correctly
- [x] Pattern subscriptions (wildcards) work identically to Redis
- [x] Multiple subscribers receive all events
- [x] Event delivery is reliable (no lost events)
- [x] Subscription lifecycle (subscribe/unsubscribe/cleanup) works correctly
- [x] Test coverage >80% for integration code
- [x] `make test` passes with no failures
- [x] All tests use `t.Context()` (no `context.Background()`)
- [x] All tests follow "Should..." naming convention
- [x] Test output clearly shows miniredis backend being tested
- [x] No behavioral differences between miniredis and external Redis
- [x] Documentation updated with any edge cases discovered

## Dependencies

- **Blocks**: Task 9.0 (End-to-End Workflow Tests) - requires streaming validation
- **Blocked By**: Task 3.0 (Mode-Aware Cache Factory) - requires factory to create miniredis clients

## Estimated Effort

**Size**: M (Medium - 1 day)

**Breakdown**:
- Test suite creation: 3 hours
- Basic publish/subscribe tests: 2 hours
- Pattern subscription tests: 2 hours
- Multiple subscriber tests: 2 hours
- Reliability and lifecycle tests: 2 hours
- Edge case testing and documentation: 1 hour

**Total**: ~12 hours (1 day)

## Risk Assessment

**Risks**:
1. Pub/Sub behavior differences between miniredis and Redis
2. Pattern subscription matching differences
3. Event delivery timing issues causing flaky tests
4. Subscription cleanup not working correctly

**Mitigations**:
1. Run identical test suite against both miniredis and external Redis (contract tests)
2. Use deterministic test patterns and reasonable timeouts
3. Add retry logic for timing-sensitive tests
4. Ensure proper cleanup in all test cases with t.Cleanup()
5. Document any discovered behavioral differences

## Validation Checklist

Before marking this task complete:

- [x] All subtasks completed
- [x] All tests in "Tests" section implemented and passing
- [x] Test coverage verified (>80%)
- [x] `make lint` passes with no warnings
- [x] `make test` passes with no failures
- [x] Integration tests added to CI pipeline
- [x] Code follows `.cursor/rules/test-standards.mdc`
- [x] All uses of context follow patterns (t.Context() in tests)
- [x] Test fixtures and helpers properly organized
- [x] No flaky tests (all tests deterministic)
- [x] Documentation updated if edge cases discovered
