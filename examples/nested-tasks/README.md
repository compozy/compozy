# Nested Tasks Example

This example demonstrates the nested container tasks functionality in Compozy, showcasing how
different task types can be nested within each other.

## What it tests

- **Composite tasks** containing multiple task types
- **Parallel tasks** with different strategies (wait_all, race)
- **Collection tasks** processing arrays of data (parallel and sequential modes)
- **Deep nesting** of container tasks within each other

## Task structure

```
root_composite (composite)
├── parallel_section (parallel, wait_all)
│   ├── task_a (basic - echo tool)
│   ├── task_b (basic - echo tool)
│   └── task_c (basic - counter tool)
├── collection_section (collection, parallel)
│   └── process-{index} (basic - echo tool for each item)
└── nested_composite (composite)
    ├── nested_parallel (parallel, race)
    │   ├── race_task_1 (basic - echo tool)
    │   └── race_task_2 (basic - echo tool)
    └── final_collection (collection, sequential)
        └── final-{index} (basic - echo tool for each item)
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

```bash
curl -X POST http://localhost:3001/api/v0/workflows/nested-tasks/executions \
    -H "Content-Type: application/json" \
    -d '{
"input": {
    "test_data": ["item1", "item2", "item3"],
      "parallel_count": 3
    }
  }'
```

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
