# Examples Plan Template

## Conventions

- Folder prefix: `examples/[feature]/*`.
- Provide small local sample content; avoid secrets; use env interpolation.

## Example Matrix

1. examples/[feature]/[example-name]

- Purpose: [what it demonstrates]
- Files:
  - `compozy.yaml` – [summary]
  - `workflows/*.yaml` – [summary]
  - Additional assets
- Demonstrates: [key aspects]
- Walkthrough:
  - [commands]

## Minimal YAML Shapes

```yaml
# insert minimal project/workflow snippets relevant to feature
```

## Test & CI Coverage

- Integration test paths to add (if applicable)

## Runbooks per Example

- Prereqs: [env vars]
- Commands: [exact commands]

## Acceptance Criteria

- P0 examples runnable locally; outputs match expectations.
- README in each folder with commands and expected outputs.
