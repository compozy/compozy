# Memory Service Test Plan

## Testing Philosophy

- Minimize mocking - use real implementations where possible
- Mock only external dependencies (MemoryManager)
- Use actual data structures instead of mocks for value objects
- Focus on behavior verification, not implementation details

## What to Mock vs Real

### Mock Only:

1. **MemoryManager** - This is the external dependency that creates memory instances
2. **Memory instances** - These interact with storage/persistence layers

### Use Real:

1. **TemplateEngine** - Create a real instance for template tests, nil for non-template tests
2. **Data structures** - Use real core.Input, workflow.State, task.State objects
3. **Validation functions** - Test these directly without mocks
4. **Conversion functions** - Test these directly without mocks
5. **MemoryTransaction** - Test with mock memory but real transaction logic

## Test Categories

### 1. Unit Tests for Pure Functions

- `ValidateMemoryRef()` - Test validation rules
- `ValidateKey()` - Test validation rules
- `PayloadToMessages()` - Test conversion logic
- `MessagesToOutputFormat()` - Test conversion logic
- No mocking needed

### 2. Integration Tests for Service Operations

- Mock only MemoryManager and Memory instances
- Use real request/response objects
- Test happy paths and error scenarios
- Verify atomic transaction behavior

### 3. Template Resolution Tests

- Use real TemplateEngine instance
- Test with actual workflow state data
- Verify template resolution in payloads

### 4. Transaction Tests

- Use mock memory but test real transaction behavior
- Verify rollback on failures
- Test atomic guarantees

## Simplified Test Approach

Instead of extensive mocking, we'll:

1. Create minimal test doubles for MemoryManager and Memory
2. Use table-driven tests for validation and conversion
3. Focus on behavior verification
4. Test error paths and edge cases
