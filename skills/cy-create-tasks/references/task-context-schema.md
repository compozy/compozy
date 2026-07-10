# Task Metadata Schemas

Task metadata is parsed from YAML frontmatter by Compozy's `ParseTaskFile()` function in `internal/core/tasks/parser.go`.
Parallel task execution also reads the canonical `_tasks.md` graph manifest.

## `_tasks.md` Graph Manifest

The `_tasks.md` file owns dependency relationships for the whole task suite. It must start with frontmatter shaped like:

```yaml
schema_version: "compozy.tasks/v2"
workflow: feature-name
graph:
  nodes:
    - id: task_01
      file: task_01.md
  edges:
    - from: task_01
      to: task_02
```

Graph rules:

- `schema_version` MUST be `compozy.tasks/v2`.
- `workflow` MUST match the feature/task directory name.
- `graph.nodes` MUST include every generated task exactly once.
- Node `id` values MUST be canonical `task_NN` identities.
- Node `file` values MUST match the node id, e.g. `task_01.md`.
- `graph.edges` stores dependency relationships only. Each edge means `from` must finish before `to` can start.
- Use `edges: []` when there are no dependencies.
- The graph MUST be acyclic.

## Individual Task Frontmatter

Individual task files own task metadata only. They do not own graph topology.

### Required Fields

- `status`: Task lifecycle state.
- `title`: Human-readable task title. It must match the first H1 in the task body.
- `type`: Allowed work type slug. Use `[tasks].types` from `.compozy/config.toml` when configured; otherwise use the built-in defaults `frontend`, `backend`, `docs`, `test`, `infra`, `refactor`, `chore`, `bugfix`.
- `complexity`: Risk rating. Must be one of: `low`, `medium`, `high`, `critical`. Complexity rates implementation risk (regression surface, concurrency, cross-task coordination), not size — a large but well-specified task can be `low`, and a high rating is never a reason to split the task.

Do not include `dependencies` in individual task frontmatter for `compozy.tasks/v2` suites. Dependencies belong only in `_tasks.md` under `graph.edges`.

## Status Values

Valid `status` values:

- `pending` - task has not been started.
- `in_progress` - task is currently being worked on.
- `completed` - task is finished and verified.
- `done` - treated as completed.
- `finished` - treated as completed.

## File Naming

Task files must match the pattern `task_\d+\.md` with zero-padded numbers:
- `task_01.md`, `task_02.md`, `task_10.md`, `task_99.md`

The leading underscore prefix is reserved for meta documents:
- `_prd.md` - Product Requirements Document
- `_user_stories.md` - User-story catalog (companion to the PRD)
- `_techspec.md` - Technical Specification
- `_tests.md` - Test contract (companion to the TechSpec)
- `_tasks.md` - Task graph manifest

## Parser Compatibility

Compozy reads task files matching the regex `^task_\d+\.md$`. Files with the old `_task_` prefix are not recognized. The file MUST start with YAML frontmatter for `ParseTaskFile()` to read the metadata.
