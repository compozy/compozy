# Task Memory: task_08.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Route CLI commands and legacy `internal/core/api.go` entry points through the typed kernel dispatcher, add the required event/reader docs, preserve external CLI contracts, and finish with clean `make verify`.

## Important Decisions
- Kept `core.Config` exported and marked transitional in Phase A, per ADR-001 and the tech spec, instead of deleting the type during task_08.
- Avoided a `core` ↔ `kernel` import cycle by registering dispatcher-backed legacy adapters from `internal/core/kernel` into `internal/core/api.go`.
- Added an unexported `newRootCommandWithDefaults` / `commandStateDefaults` seam so CLI parity tests can bypass bundled-skill preflight without changing production behavior.
- Persisted workflow `result.json` for text-mode runs as well as JSON-mode runs so the run artifact contract is consistent across workflow commands.

## Learnings
- `start` preflight validates title/H1 synchronization even for dry-run workflows, so CLI fixtures must keep frontmatter `title` aligned with the first H1.
- `fix-reviews` batch prompts do not inline the review title/body; the stable prompt contract is the required skill list plus the issue file and code-file scope.

## Files / Surfaces
- `internal/cli/root.go`
- `internal/cli/root_command_execution_test.go`
- `internal/cli/form_test.go`
- `internal/core/api.go`
- `internal/core/api_test.go`
- `internal/core/kernel/core_adapters.go`
- `internal/core/kernel/core_adapters_test.go`
- `internal/core/kernel/deps.go`
- `internal/core/kernel/handlers.go`
- `internal/core/run/result.go`
- `internal/core/run/result_test.go`
- `pkg/compozy/runs/examples_test.go`
- `pkg/compozy/events/docs_test.go`
- `docs/events.md`
- `docs/reader-library.md`
- `CLAUDE.md`
- `compozy.go`

## Errors / Corrections
- Initial attempt to rewrite `internal/core/api.go` by importing `internal/core/kernel` directly created an import cycle; replaced with kernel-registered adapters plus preserved direct implementations for handler use.
- `make verify` initially failed on lint because the new root-command seam left redundant wrappers and the public-package blank import lacked justification; removed the redundant wrapper, updated tests, and documented the adapter-registration import.

## Ready for Next Run
- Task implementation, docs, parity coverage, task tracking, and `make verify` are complete. Only the local commit remains, while keeping tracking-only files out of the automatic commit unless explicitly required.
