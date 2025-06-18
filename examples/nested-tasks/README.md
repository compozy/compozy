# Nested Tasks Example

This example demonstrates the nested container tasks functionality in Compozy, showcasing how different task types can be nested within each other.

## What it tests

- **Composite tasks** containing multiple task types
- **Parallel tasks** with different strategies (wait_all, race)
- **Collection tasks** processing arrays of data (parallel and sequential modes)
- **Deep nesting** of container tasks within each other

## Task structure

```
root_composite (composite, strategy: fail_fast)
├── parallel_section (parallel, strategy: wait_all)
│   ├── task_a (basic)
│   │   └── tool: echo_tool → message: "Parallel Task A executed"
│   ├── task_b (basic)
│   │   └── tool: echo_tool → message: "Parallel Task B executed"
│   └── task_c (basic)
│       └── tool: counter_tool → count: 3
├── collection_section (collection, mode: parallel, strategy: best_effort)
│   └── process-{index} (basic, for each test_data item)
│       └── tool: echo_tool → message: "Processing item: {{ .item }} at index {{ .index }}"
└── nested_composite (composite)
    ├── nested_parallel (parallel, strategy: race)
    │   ├── race_task_1 (basic)
    │   │   └── tool: echo_tool → message: "Race task 1 - trying to win!"
    │   └── race_task_2 (basic)
    │       └── tool: echo_tool → message: "Race task 2 - trying to win!"
    └── final_collection (collection, mode: sequential, strategy: fail_fast)
        └── final-{index} (basic, for each static item)
            └── tool: echo_tool → message: "Final processing: {{ .item }}"
```

## Tools

- **echo_tool**: Simple tool that echoes a message with timestamp
- **counter_tool**: Tool that generates a sequence of numbers

## Running

```bash
cd examples/nested-tasks
../../compozy dev
```

Then trigger the workflow via the API or UI to see nested tasks execution in action.

### Trigger via API

See the `api.http` file in this directory for example API requests you can run directly in your editor or with tools like REST Client extensions.

## Expected behavior

1. The root composite executes its child tasks sequentially (fail_fast strategy)
2. The parallel section runs all three basic tasks simultaneously
3. The collection section processes test data items in parallel
4. The nested composite demonstrates deeper nesting
5. The nested parallel uses race strategy (first to complete wins)
6. The final collection processes items sequentially with fail_fast

This example validates that:

- Collection tasks use parent templates correctly
- Composite tasks load child configs from metadata
- Parallel tasks batch-load configs efficiently
- All nesting levels work without errors
