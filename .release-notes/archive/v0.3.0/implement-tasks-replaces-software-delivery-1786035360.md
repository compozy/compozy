---
title: implement-tasks replaces the software-delivery Loop
type: breaking
---

The bundled dev-cycle Loop now does one job clearly: implement authored task files in dependency order. `software-delivery` is gone and `implement-tasks` takes its place, with a five-node graph — `slug_input → load_tasks → implement → execute_task → collect` — and only three inputs. The old second control layer for review, command verification, and human approval is removed from the bundled Loop; task-level validation, self-review, tracking updates, and optional per-task commits stay inside the implementation agent's own prompt. (#325)

- Inputs are now `slug`, `implementer`, and `auto_commit`. The `review`, `verify`, and `approve` nodes and their edges are deleted, along with the verification contract, stale hash fields, and target-branch handling.
- The separate bundled `review-and-fix` Loop is unchanged, and custom Loops can still declare their own command gates — `verify_command` remains part of the generic Loop DSL.
- The catalog, Loop overview, configuration examples, migration guide, web routes, and the official Compozy skill all name `implement-tasks`.

Migration notes: this is a hard cut with no alias. Any config, CLI or API call, automation binding, or documentation link that says `software-delivery` must say `implement-tasks`, and the `target_branch` and `verify_command` inputs must be dropped from `[loops.inputs.*]`.

```toml
# before
[loops.inputs.software-delivery]
target_branch = "main"
verify_command = "make gate"
auto_commit = false

# after
[loops.inputs.implement-tasks]
auto_commit = false
```
