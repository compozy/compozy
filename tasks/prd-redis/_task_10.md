## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>test/integration/cache</domain>
<type>testing</type>
<scope>contract_validation</scope>
<complexity>medium</complexity>
<dependencies>cache|miniredis</dependencies>
</task_context>

# Task 10.0: Contract Tests & Validation [Size: M - 1 day]

## Overview

Create comprehensive contract tests that verify miniredis adapter behaves identically to external Redis adapter. These tests validate that all 48 methods of the `cache.RedisInterface` work correctly with both backends, ensuring complete behavioral parity.

<critical>
- **ALWAYS READ** @.cursor/rules/test-standards.mdc before start
- **ALWAYS READ** the technical docs from this PRD before start
- **YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
- **MUST** use `t.Context()` in all tests - NEVER `context.Background()`
- **MUST** follow `t.Run("Should ...")` naming convention
- **MUST** use testify assertions (require/assert)
- **MUST** test ALL 48 RedisInterface methods
</critical>

<research>
# When you need information about a library or external API:
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase, the same for the topic param of context7
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</research>

<requirements>
- All 48 RedisInterface methods must be tested with both adapters
- Behavioral equivalence must be verified (same inputs → same outputs)
- Error cases must behave identically
- Lua scripts must execute identically
- TxPipeline operations must behave identically
- Pub/Sub must work identically
- Mode switching between adapters must work correctly
- Contract tests must be comprehensive and exhaustive
</requirements>

## Subtasks

- [ ] 10.1 Create contract test framework for cache adapters
- [ ] 10.2 Implement basic operations contract tests (Get, Set, Del, Exists)
- [ ] 10.3 Implement Lua script contract tests (Eval, EvalSha)
- [ ] 10.4 Implement TxPipeline contract tests (transactions, atomicity)
- [ ] 10.5 Implement Pub/Sub contract tests (Subscribe, Publish, patterns)
- [ ] 10.6 Implement data structure contract tests (Hash, List, Set, ZSet)
- [ ] 10.7 Implement error handling contract tests
- [ ] 10.8 Implement mode switching tests

## Implementation Details

### Contract Test Pattern

Create a test suite that runs the same tests against both Redis and miniredis adapters. This ensures behavioral parity between the two implementations.

### Relevant Files

**New/Updated Files:**
- `test/integration/cache/adapter_contract_test.go` - Cache adapter contract tests
- `test/integration/cache/helpers.go` - Test helpers for adapter setup

### Dependent Files

- `engine/infra/cache/redis.go` - Redis interface definition
- `engine/infra/cache/miniredis_standalone.go` - Miniredis implementation (Task 2.0)
- `engine/infra/cache/mod.go` - Cache factory (Task 3.0)

## Deliverables

- Contract test framework that runs same tests against both adapters
- Basic operations tests (Get, Set, Del, Exists, TTL, etc.)
- Lua script tests (Eval, EvalSha with all memory store scripts)
- TxPipeline tests (atomic multi-key operations)
- Pub/Sub tests (Subscribe, Publish, PSubscribe, pattern matching)
- Data structure tests (Hash, List, Set, Sorted Set operations)
- Error handling tests (connection errors, invalid operations)
- Mode switching tests (config-based adapter selection)
- Test report documenting all 48 methods tested
- All tests passing with `make test`

## Tests

Unit tests mapped from `_tests.md` for this feature:

### Cache Adapter Contract Tests (`test/integration/cache/adapter_contract_test.go`)

**Framework:**
- [ ] Should run same test suite against both Redis and miniredis
- [ ] Should verify identical behavior for all operations
- [ ] Should compare outputs byte-for-byte where possible
- [ ] Should validate error types and messages match

**Basic Operations (String Commands):**
- [ ] Should satisfy cache.RedisInterface contract
- [ ] Get, Set, Del, Exists should behave identically
- [ ] SetNX, SetEX, GetSet should behave identically
- [ ] Incr, Decr, IncrBy, DecrBy should behave identically
- [ ] MGet, MSet should behave identically
- [ ] TTL, Expire, ExpireAt, Persist should behave identically

**Lua Scripts:**
- [ ] Eval should execute scripts identically
- [ ] EvalSha should work with script caching
- [ ] AppendAndTrimWithMetadataScript should execute correctly
- [ ] PutIfMatch script should execute correctly
- [ ] All memory store Lua scripts should produce same results

**TxPipeline (Transactions):**
- [ ] TxPipeline should support atomic operations
- [ ] Multi-key operations should be atomic
- [ ] Watch should detect concurrent modifications
- [ ] Pipeline commands should batch correctly
- [ ] Rollback on error should work identically

**Pub/Sub:**
- [ ] Subscribe should receive published messages
- [ ] PSubscribe should match patterns correctly
- [ ] Publish should deliver to all subscribers
- [ ] Unsubscribe should stop receiving messages
- [ ] Multiple subscribers should all receive messages

**Hash Operations:**
- [ ] HGet, HSet, HDel, HExists should behave identically
- [ ] HMGet, HMSet should behave identically
- [ ] HGetAll, HKeys, HVals, HLen should behave identically
- [ ] HIncrBy should behave identically

**List Operations:**
- [ ] LPush, RPush, LPop, RPop should behave identically
- [ ] LLen, LRange, LIndex should behave identically
- [ ] LTrim should behave identically

**Set Operations:**
- [ ] SAdd, SRem, SMembers should behave identically
- [ ] SIsMember, SCard should behave identically
- [ ] SInter, SUnion, SDiff should behave identically

**Sorted Set Operations:**
- [ ] ZAdd, ZRem, ZScore should behave identically
- [ ] ZRange, ZRevRange should behave identically
- [ ] ZCard, ZCount should behave identically
- [ ] ZIncrBy should behave identically

**Error Handling:**
- [ ] Invalid operations should return same error types
- [ ] Connection errors should be handled identically
- [ ] Type errors should be handled identically
- [ ] Script errors should propagate identically

**Mode Switching:**
- [ ] Should create correct adapter based on config mode
- [ ] Should switch between adapters on config change
- [ ] Should handle invalid mode configurations
- [ ] Should respect mode overrides

### Edge Cases

- [ ] Empty values should behave identically
- [ ] Nil returns should behave identically
- [ ] Concurrent operations should behave identically
- [ ] Large values (>1MB) should behave identically
- [ ] Special characters in keys should behave identically

### Coverage Requirements

- [ ] All 48 RedisInterface methods tested
- [ ] Test coverage >95% for contract tests
- [ ] Document any behavioral differences found

## Success Criteria

- Contract test framework implemented and working
- All 48 RedisInterface methods have contract tests
- All tests pass for both Redis and miniredis adapters
- Zero behavioral differences detected between adapters
- Lua scripts execute identically on both backends
- TxPipeline operations are atomic on both backends
- Pub/Sub works identically on both backends
- Error handling is consistent across adapters
- Mode switching works correctly based on configuration
- Test report documents complete method coverage
- Tests pass with `go test -v -race ./test/integration/cache/`
- No flaky tests in contract test suite
- All tests follow project test standards
