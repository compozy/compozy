## status: completed

<task_context>
<domain>sdk/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>sdk/task</dependencies>
</task_context>

# Task 45.0: Example: Parallel Tasks (S)

## Overview

Create example demonstrating parallel task execution using ParallelBuilder and AggregateBuilder. Shows concurrent execution patterns and result aggregation.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/05-examples.md (Example 2: Parallel Tasks)
- **MUST** demonstrate Parallel and Aggregate task types
- **MUST** use context-first pattern throughout
</critical>

<requirements>
- Runnable example: sdk/examples/02_parallel_tasks.go
- Demonstrates: ParallelBuilder, AggregateBuilder, BasicBuilder
- Shows concurrent execution (sentiment, entity extraction, summarization)
- Result aggregation pattern
- Clear comments on parallel execution benefits
</requirements>

## Subtasks

- [x] 45.1 Create sdk/examples/02_parallel_tasks.go
- [x] 45.2 Build multiple specialized agents (sentiment, entity, summary)
- [x] 45.3 Create individual analysis tasks
- [x] 45.4 Build parallel task to run analyses concurrently
- [x] 45.5 Build aggregate task to combine results
- [x] 45.6 Build workflow orchestrating parallel execution
- [x] 45.7 Add comments explaining parallel execution
- [x] 45.8 Update README.md with parallel example
- [x] 45.9 Test example runs successfully

## Implementation Details

Per 05-examples.md section 2:

**Parallel task pattern:**
```go
parallelTask, _ := task.NewParallel("parallel-analysis").
    AddTask("sentiment-task").
    AddTask("entity-task").
    AddTask("summary-task").
    WithWaitAll(true).  // Wait for all tasks to complete
    Build(ctx)
```

**Aggregate task pattern:**
```go
aggregateTask, _ := task.NewAggregate("combine-results").
    AddTask("parallel-analysis").
    WithStrategy("merge").
    WithFinal(true).
    Build(ctx)
```

### Relevant Files

- `sdk/examples/02_parallel_tasks.go` - Main example
- `sdk/examples/README.md` - Updated instructions

### Dependent Files

- `sdk/task/parallel.go` - ParallelBuilder
- `sdk/task/aggregate.go` - AggregateBuilder
- `sdk/task/basic.go` - BasicBuilder

## Deliverables

- [x] sdk/examples/02_parallel_tasks.go (runnable)
- [x] Updated README.md with parallel example section
- [x] Comments explaining concurrent execution benefits
- [x] Example demonstrates all 3 task types (Basic, Parallel, Aggregate)
- [x] Verified example runs successfully

## Tests

From _tests.md:

- Example validation:
  - [x] Code compiles without errors
  - [x] Example runs with parallel execution
  - [x] All tasks execute concurrently (verify logs)
  - [x] Aggregate task combines results correctly
  - [x] WithWaitAll(true) behavior verified
  - [x] Error handling for parallel failures

## Success Criteria

- Example clearly demonstrates parallel execution
- Aggregate pattern shown correctly
- Comments explain when to use parallel tasks
- README updated with run instructions
- Example runs end-to-end successfully
- Code passes `make lint`
