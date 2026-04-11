# Task Memory: task_12.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implement the builtin `compozy ext` management surface (`list`, `inspect`, `install`, `uninstall`, `enable`, `disable`, `doctor`) on top of the existing extension discovery and enablement APIs.
- Keep the new CLI package within the task-12 seven-file grouping and land the required unit + round-trip integration coverage at 80%+.
- Finish only after clean targeted CLI tests and a clean `make verify`.

## Important Decisions
- The new command package lives in `internal/cli/extension` and exposes `NewExtCommand(dispatcher)` for root registration, but the command handlers stay metadata/local-state oriented and do not route through kernel handlers.
- `list` shows every raw discovered declaration across bundled, user, and workspace scopes, plus an `active` column derived from precedence + enablement so overridden or disabled entries are visible without ambiguity.
- `inspect`, `enable`, and `disable` resolve by the effective declaration name (highest-precedence discovered entry, even when disabled) so operator actions match ADR-007 precedence rules.
- `install` copies an extension directory into `~/.compozy/extensions/<name>/`, then immediately writes the user-scope enablement file as disabled so installs never auto-activate locally.
- `uninstall` removes any user-scope directory by name if present, even when a workspace declaration with the same name shadows it; bundled/workspace declarations are refused explicitly.
- `doctor` treats skill-pack and provider overlay drift as placeholder informational output for task 13, while still validating manifests, surfacing discovery failures, warning on active hook priority ties, and flagging manifest-only unused capability signals heuristically.

## Learnings
- The existing `internal/core/extension` package already provides everything task 12 needs for CLI state: manifest loading, three-level discovery, precedence override records, and home-backed user/workspace enablement persistence.
- `extensions.Discovery{IncludeDisabled:true}` returns the effective declaration set regardless of local enabled state, which makes it the right basis for `inspect` and enable/disable targeting.
- Package-local black-box CLI tests with injected home/workspace roots were enough to cover the real copy/install/toggle/list behavior without introducing test-only production seams.
- The final `internal/cli/extension` package reached `80.7%` statement coverage.

## Files / Surfaces
- `internal/cli/root.go`
- `internal/cli/extension/root.go`
- `internal/cli/extension/display.go`
- `internal/cli/extension/install.go`
- `internal/cli/extension/enablement.go`
- `internal/cli/extension/doctor.go`
- `internal/cli/extension/display_test.go`
- `internal/cli/extension/doctor_test.go`

## Errors / Corrections
- The first implementation pass failed lint on `gocritic` `rangeValCopy`, one unused install-prompt parameter, and two over-generic helper signatures (`unparam`). Those were fixed by switching the affected loops to index-based iteration, tightening helper signatures, and rerunning `make verify`.
- Early package coverage landed at `70.2%`; extra focused helper tests were added until the package reached the required 80%+ threshold.

## Ready for Next Run
- Code, tests, and full verification are complete. Remaining close-out work is limited to updating task tracking, reviewing the final diff, and creating the required local commit.
