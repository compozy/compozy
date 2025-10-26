## status: pending

<task_context>
<domain>v2/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>v2/task</dependencies>
</task_context>

# Task 45.0: Example: Parallel Tasks (S)

## Overview

Create example demonstrating parallel task execution using ParallelBuilder and AggregateBuilder. Shows concurrent execution patterns and result aggregation.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-modules/05-examples.md (Example 2: Parallel Tasks)
- **MUST** demonstrate Parallel and Aggregate task types
- **MUST** use context-first pattern throughout
</critical>

<requirements>
- Runnable example: v2/examples/02_parallel_tasks.go
- Demonstrates: ParallelBuilder, AggregateBuilder, BasicBuilder
- Shows concurrent execution (sentiment, entity extraction, summarization)
- Result aggregation pattern
- Clear comments on parallel execution benefits
</requirements>

## Subtasks

- [ ] 45.1 Create v2/examples/02_parallel_tasks.go
- [ ] 45.2 Build multiple specialized agents (sentiment, entity, summary)
- [ ] 45.3 Create individual analysis tasks
- [ ] 45.4 Build parallel task to run analyses concurrently
- [ ] 45.5 Build aggregate task to combine results
- [ ] 45.6 Build workflow orchestrating parallel execution
- [ ] 45.7 Add comments explaining parallel execution
- [ ] 45.8 Update README.md with parallel example
- [ ] 45.9 Test example runs successfully

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

- `v2/examples/02_parallel_tasks.go` - Main example
- `v2/examples/README.md` - Updated instructions

### Dependent Files

- `v2/task/parallel.go` - ParallelBuilder
- `v2/task/aggregate.go` - AggregateBuilder
- `v2/task/basic.go` - BasicBuilder

## Deliverables

- [ ] v2/examples/02_parallel_tasks.go (runnable)
- [ ] Updated README.md with parallel example section
- [ ] Comments explaining concurrent execution benefits
- [ ] Example demonstrates all 3 task types (Basic, Parallel, Aggregate)
- [ ] Verified example runs successfully

## Tests

From _tests.md:

- Example validation:
  - [ ] Code compiles without errors
  - [ ] Example runs with parallel execution
  - [ ] All tasks execute concurrently (verify logs)
  - [ ] Aggregate task combines results correctly
  - [ ] WithWaitAll(true) behavior verified
  - [ ] Error handling for parallel failures

## Success Criteria

- Example clearly demonstrates parallel execution
- Aggregate pattern shown correctly
- Comments explain when to use parallel tasks
- README updated with run instructions
- Example runs end-to-end successfully
- Code passes `make lint`
