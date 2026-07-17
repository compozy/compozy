# Workflow Memory

## Shared Decisions

- Chose event-sourcing for orders (append-only stream + projected read model). Writers keyed by command
  id for idempotency after review issue_001.

## Shared Learnings

- Table prefix `ord_` used feature-locally.
