# Examples Plan Template

## Conventions

- Folder prefix: `examples/usage-reporting/*`.
- Use minimal YAML workflows with fake provider IDs; no secrets.

## Example Matrix

1. examples/usage-reporting/workflow-summary

- Purpose: Demonstrate capturing usage for a workflow execution and retrieving via CLI.
- Files:
  - `compozy.yaml` – basic project wiring with LLM provider stub.
  - `workflows/usage_demo.yaml` – workflow invoking a single agent action.
  - `scripts/run.sh` – helper script to run workflow and fetch execution data.
- Demonstrates: Workflow execution, API fetch showing `usage` object.
- Walkthrough:
  - `./scripts/run.sh`
  - Shows `curl` or `compozy executions workflows get` output with usage tokens.

2. examples/usage-reporting/task-direct-exec

- Purpose: Highlight direct task execution usage logging when workflows skipped.
- Files:
  - `compozy.yaml` – referencing tasks configuration.
  - `tasks/direct_task.yaml` – task invoking agent with prompt.
  - `README.md` – commands to execute task via CLI and read usage.
- Demonstrates: Task execution endpoint returns usage summary.
- Walkthrough:
  - `compozy tasks exec direct_task`
  - `compozy executions tasks get <exec_id>` to inspect `usage`.

3. examples/usage-reporting/agent-sync

- Purpose: Show agent sync endpoint returning usage; includes failure scenario logging null usage.
- Files:
  - `agents/agent.yaml` – agent definition with actions.
  - `scripts/agent.sh` – script invoking `compozy agents exec`.
  - `README.md` – expected outputs and handling when provider omits usage metadata.
- Demonstrates: API fallback to `null` usage and warning logs.
- Walkthrough:
  - `./scripts/agent.sh`
  - Inspect JSON output for usage (present or null).

## Minimal YAML Shapes

```yaml
# workflows/usage_demo.yaml
workflows:
  usage_demo:
    description: Demo workflow for usage metrics
    steps:
      - id: ask
        run:
          agent: demo-agent
          with:
            prompt: "Summarize the meeting notes"
```

## Test & CI Coverage

- Add integration test referencing `examples/usage-reporting/workflow-summary` to validate documentation sample stays current.

## Runbooks per Example

- Prereqs: `COMPOZY_API_TOKEN`, staging backend URL, feature flag enabled.
- Commands: Provided in respective README/scripts.

## Acceptance Criteria

- Each example runnable with mocked provider; outputs show usage fields.
- README includes expected JSON snippet and troubleshooting tips.
