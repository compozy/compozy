---
title: Time travel — compare, rerun, and fork a Loop run
type: feature
---

Durable Loop history became something you can act on. `diff` reads what changed, `rerun` opens a new generation from a settled node in the same run, and `fork` starts a linked run from a historical generation without changing its source. (#427)

- `compozy loop diff --run-id <id> --generation 1 --against-generation 2` compares two generations; `--against-run` compares two runs of the same Loop and marks different pinned definitions. Large values return their byte size and SHA-256 content hash instead of an oversized inline payload.
- `compozy loop rerun --from-node verify` re-runs the selected node and its transitive dependents while unrelated settled cells carry forward; `--item` addresses one fan-out lane. The new generation has origin `operator_rerun`.
- `compozy loop fork --generation 2` pins the source run's executed definition: generation 1 is a settled `fork_seed` baseline and generation 2 executes the body with the source inputs plus any validated overrides. Lineage is two-way — the child carries `forked_from`, the source lists `forks`.
- In the web UI, **Compare…** on an Inspect generation row opens a deep-linkable comparison page whose node rows group by the same `changed / rerun / skipped / carried / verdict` vocabulary the CLI prints, and **Fork from here** pre-fills the source run's declared inputs. Two identical generations render an explicit "nothing changed" state.
- Pass `--request-id` to retry a rerun or fork after a transport failure: the same key with identical inputs returns the committed result, and a changed request under a reused key returns `timetravel_key_reuse`.
- Agents need the `loops.timetravel` capability and get `compozy__loop_diff`, `compozy__loop_rerun`, and `compozy__loop_fork`. Diff is an ordinary workspace-scoped read. An agent cannot rerun its own executing run, but it may rerun its own terminal run.

```bash
compozy loop diff  --run-id <run-id> --generation 1 --against-generation 2
compozy loop rerun --run-id <run-id> --from-node verify --reason "retry verification"
compozy loop fork  --run-id <run-id> --generation 2 --input service=payments
```
