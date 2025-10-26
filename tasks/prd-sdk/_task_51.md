## status: pending

<task_context>
<domain>sdk/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>sdk/task</dependencies>
</task_context>

# Task 51.0: Example: Signals (Unified) (S)

## Overview

Create example demonstrating inter-workflow communication using the unified SignalBuilder for both send and wait operations.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/05-examples.md (Example 8: Signal Communication)
- **MUST** demonstrate both Send() and Wait() signal operations
- **MUST** show timeout configuration for wait operations
</critical>

<requirements>
- Runnable example: sdk/examples/08_signal_communication.go
- Demonstrates: SignalBuilder with Send() and Wait() operations
- Shows: Two workflows communicating via signals
- Signal payload data passing
- Timeout configuration for wait
- Clear comments on signal patterns
</requirements>

## Subtasks

- [ ] 51.1 Create sdk/examples/08_signal_communication.go
- [ ] 51.2 Build workflow 1 (sender):
  - [ ] Processing task
  - [ ] Signal send task with payload
- [ ] 51.3 Build workflow 2 (receiver):
  - [ ] Signal wait task with timeout
  - [ ] Analysis task after signal
- [ ] 51.4 Demonstrate signal task patterns:
  - [ ] Send operation with data payload
  - [ ] Wait operation with timeout
- [ ] 51.5 Add comments explaining signal use cases
- [ ] 51.6 Update README.md with signal example
- [ ] 51.7 Test example runs successfully

## Implementation Details

Per 05-examples.md section 8:

**Signal send task:**
```go
signalSendTask, err := task.NewSignal("notify-ready").
    Send("data-ready", map[string]interface{}{
        "status": "completed",
        "result_id": "{{ .tasks.process-data.output.id }}",
    }).
    WithFinal(true).
    Build(ctx)
```

**Signal wait task:**
```go
waitTask, err := task.NewSignal("wait-for-data").
    Wait("data-ready").
    WithTimeout(5 * time.Minute).
    Build(ctx)
```

**Workflow 1 (sender):**
```go
workflow1, _ := workflow.New("data-processor").
    AddTask(processTask).
    AddTask(signalSendTask).
    Build(ctx)
```

**Workflow 2 (receiver):**
```go
workflow2, _ := workflow.New("data-analyzer").
    AddTask(waitTask).
    AddTask(analyzeTask).
    Build(ctx)
```

### Relevant Files

- `sdk/examples/08_signal_communication.go` - Main example
- `sdk/examples/README.md` - Updated instructions

### Dependent Files

- `sdk/task/signal.go` - SignalBuilder
- `sdk/workflow/builder.go` - Workflows
- `sdk/project/builder.go` - Project with workflows

## Deliverables

- [ ] sdk/examples/08_signal_communication.go (runnable)
- [ ] Updated README.md with signal example section
- [ ] Comments explaining:
  - Signal communication patterns
  - When to use signals (inter-workflow coordination)
  - Send vs Wait operations
  - Payload data passing
  - Timeout configuration
- [ ] Both workflows demonstrated
- [ ] Verified example runs successfully

## Tests

From _tests.md:

- Example validation:
  - [ ] Code compiles without errors
  - [ ] Signal send task with payload
  - [ ] Signal wait task with timeout
  - [ ] Signal names match between send and wait
  - [ ] Payload data structure validated
  - [ ] Timeout validated (positive duration)
  - [ ] Two workflows communicate correctly

## Success Criteria

- Example demonstrates signal send and wait
- Comments explain signal use cases
- Payload data passing shown
- Timeout configuration demonstrated
- README updated with signal pattern explanation
- Example runs end-to-end successfully
- Code passes `make lint`
