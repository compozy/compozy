---
provider: manual
pr:
round: 2
round_created_at: 2026-07-22T15:39:03Z
status: resolved
file: internal/core/tasks/validate.go
line: 21
severity: high
author: claude-code
provider_ref:
---

# Issue 001: Task validation omits requirement-to-test traceability

## Review Comment

`ValidateWithOptions` validates task metadata and graph shape, but it never loads the canonical requirements or test catalog and has no representation for requirement IDs, acceptance behaviors, test ownership, or diagnostic severity. `TestValidateRules/accepts clean v2 task` therefore treats a task containing only valid frontmatter, an H1, and `Body.` as valid. This allows generation to publish syntactically valid tasks whose requirements are incomplete or objectively unverifiable. The current `Issue` type also cannot distinguish blocking errors from non-blocking warnings; `Report.OK` treats every issue identically.

Add a stable traceability model and make it part of the pre-publication gate: `requirement ID -> acceptance behavior -> test ID -> owning task`. Require every requirement to reach at least one test, every test to have exactly one task owner, and every acceptance criterion to define an observable assertion. Missing product decisions and ambiguous criteria must produce blocking diagnostics rather than invented details; advisory quality findings should be warnings that do not make `Report.OK` false. Cover both ordinary and Task Group workflows, and add negative tests proving an H1-only task, an orphan requirement, a duplicate test owner, and an untestable criterion cannot publish.

## Triage

- Decision: `INVALID`
- The cited behavior is reproducible: the existing
  `TestValidateRules/accepts clean v2 task` case confirms that metadata plus a
  matching H1 passes `ValidateWithOptions`. That behavior matches this API's
  current contract: it validates task frontmatter, dependency/manifest shape,
  and Task Group workflow identity for both CLI validation and execution
  preflight.
- The proposed traceability graph has no source schema to validate. Individual
  task frontmatter is explicitly limited to `status`, `title`, `type`, and
  `complexity`; task `<requirements>` entries have no stable IDs; and the
  supported workflow permits `_tests.md` to be absent by placing concrete test
  cases inline in each task. The canonical test catalog maps stories, edge
  cases, and components to test IDs, but does not define the requested
  requirement ID -> acceptance behavior -> test ID representation.
- Publication does not rely on `ValidateWithOptions` alone. The
  `cy-create-tasks` generation contract separately requires an exactly-once
  `_tests.md` assignment audit after `compozy tasks validate`, with an inline
  test-case fallback when no catalog exists. Task Group generation applies that
  audit initiative-wide before publishing its staged artifacts.
- Adding the requested model in `internal/core/tasks/validate.go` would therefore
  invent a new artifact contract, make runtime preflight depend on optional
  planning documents, and require coordinated schema, parser, CLI/JSON, prompt,
  and Task Group ownership changes outside this batch. It is not a fix for the
  scoped `ExpectedWorkflow` validation change and would break supported task
  suites rather than restore an established invariant.
- Verification note: the initial full `make verify` attempts reached the Go
  race suite and hit the unrelated pre-existing timing failure
  `TestShutdownEscalatesFromSIGTERMToSIGKILL: timed out waiting for SIGTERM
  marker` in `internal/core/subprocess`. The unchanged focused race test passed
  10/10 repetitions. The test launches a shell and immediately requests
  shutdown without first synchronizing that its `TERM` trap is ready, so host
  contention can deliver the signal before the marker-producing trap exists;
  this is outside the documentation-only batch.
- The first E2E attempt also exposed an environment-only limit: Playwright
  nests its daemon home under this isolated review worktree, producing a Unix
  socket path longer than the host permits (`bind: invalid argument`). A direct
  foreground launch reproduced that exact bind failure. Final verification uses
  the supported `COMPOZY_HOME` override at a short temporary path while retaining
  the complete E2E suite.
- Resolution: no production or test change is warranted for this batch.
