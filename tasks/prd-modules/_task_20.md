## status: pending

<task_context>
<domain>v2/task</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 20.0: Task: Signal (Unified) (M)

## Overview

Implement unified SignalBuilder supporting both send and wait operations for workflow coordination and event-driven patterns.

<critical>
- **MANDATORY** align with engine task type: `TaskTypeSignal = "signal"`
- **MANDATORY** use context-first Build(ctx) pattern
- **MANDATORY** support both send and wait modes in single builder
- **MANDATORY** validate mode-specific requirements (data for send, timeout for wait)
</critical>

<requirements>
- SignalBuilder with unified send/wait interface
- Signal ID configuration
- Mode selection (send vs wait)
- Data payload for send mode
- Timeout configuration for wait mode
- Success/error transitions
- Error accumulation pattern
</requirements>

## Subtasks

- [ ] 20.1 Create v2/task/signal.go
- [ ] 20.2 Define SignalMode type (send, wait)
- [ ] 20.3 Implement SignalBuilder struct and constructor
- [ ] 20.4 Add WithSignalID method
- [ ] 20.5 Add WithMode method for send/wait selection
- [ ] 20.6 Add WithData method for send payload
- [ ] 20.7 Add WithTimeout method for wait timeout
- [ ] 20.8 Add OnSuccess/OnError transition methods
- [ ] 20.9 Implement Build(ctx) with mode-specific validation
- [ ] 20.10 Write unit tests for both modes

## Implementation Details

Reference:
- `tasks/prd-modules/03-sdk-entities.md` (Section 5.8: Signal Task, Section 15: Signal System)
- `tasks/prd-modules/02-architecture.md` (Task Type Definitions)

### Key APIs

```go
// v2/task/signal.go
type SignalMode string
const (
    SignalModeSend SignalMode = "send"
    SignalModeWait SignalMode = "wait"
)

func NewSignal(id string) *SignalBuilder
func (b *SignalBuilder) WithSignalID(signalID string) *SignalBuilder
func (b *SignalBuilder) WithMode(mode SignalMode) *SignalBuilder
func (b *SignalBuilder) WithData(data map[string]interface{}) *SignalBuilder      // send mode
func (b *SignalBuilder) WithTimeout(timeout time.Duration) *SignalBuilder          // wait mode
func (b *SignalBuilder) OnSuccess(taskID string) *SignalBuilder
func (b *SignalBuilder) OnError(taskID string) *SignalBuilder
func (b *SignalBuilder) Build(ctx context.Context) (*task.Config, error)

// Convenience constructors
func (b *SignalBuilder) Send(signalID string, payload map[string]interface{}) *SignalBuilder
func (b *SignalBuilder) Wait(signalID string) *SignalBuilder
```

### Relevant Files

- `v2/task/signal.go` - SignalBuilder implementation
- `engine/task/config.go` - Task config struct

### Dependent Files

- `v2/internal/errors/build_error.go` - Error aggregation

## Deliverables

- ✅ `v2/task/signal.go` with unified SignalBuilder
- ✅ SignalMode type definition (send, wait)
- ✅ Send mode with data payload
- ✅ Wait mode with timeout
- ✅ Transition configuration (OnSuccess, OnError)
- ✅ Convenience methods (Send, Wait)
- ✅ Build(ctx) with mode-specific validation
- ✅ Unit tests for both modes

## Tests

Unit tests from `_tests.md`:
- [ ] Signal send task with payload
- [ ] Signal wait task with timeout
- [ ] Convenience Send() method
- [ ] Convenience Wait() method
- [ ] Success transition configuration
- [ ] Error transition configuration
- [ ] Error: missing signal ID
- [ ] Error: send mode without data
- [ ] Error: wait mode with data (should warn or ignore)
- [ ] Error: invalid mode
- [ ] BuildError aggregation

## Success Criteria

- Builder creates valid `TaskTypeSignal` config
- Both send and wait modes work correctly
- Mode-specific validation enforced
- Convenience methods simplify common patterns
- Transitions configured properly
- Validation rejects invalid states
- Test coverage ≥95%
- `make lint && make test` pass
