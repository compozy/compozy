# QA Run Report — 2026-08-26 — Child Loop config overrides Web rewalk

- **Scope:** Web authoring and public round-trip for typed, child-scoped `run-loop.params.config_overrides`.
- **Cadence tier:** Targeted.
- **Build:** Local `feat/run-loop-config-overrides` worktree based on `8753ad9e9`; exact PR-head CI remains a delivery gate.
- **Environment:** Fresh isolated Compozy home, workspace, daemon, Web server, and ephemeral Playwright Docker browser.
- **Started:** 2026-08-26 · **Completed:** 2026-08-27 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | Desktop / wifi-fast / en-US | CH-author-child-loop-config |

## Flows in Scope

- `J-recover-loop-node-failure` — author, publish, run, and independently re-read one child-only configuration (`../journeys/J-recover-loop-node-failure.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-author-child-loop-config | J-recover-loop-node-failure / LP-child-loop-config-web-authoring | Bruno | Feature Tour | Pass | — | — |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debrief

Bruno selected the isolated workspace from the real menubar, opened the parent Loop editor, selected
the `run-loop` node, and entered malformed JSON. The editor retained the draft for correction and did
not publish it. After correction, Web published version 2; a reload preserved numeric values and the
runtime-rule array. Public HTTP returned the same typed object.

Starting the parent through Web produced parent run `looprun-2c246ea3705ea9c2` and child run
`looprun-98061518e300c693`, both terminal `done`. Structured CLI reported that the child received cap
4, 250,000 tokens, and the Cursor/Grok runtime rule from the per-run source. A separate read of the
stored child definition remained cap 50, token budget zero, and without runtime rules, proving that
the override was child-scoped and non-persistent.

The adjacent omitted-field compatibility path was covered by the automated editor and Loop action
suites rather than repeated in this targeted browser walk.

## What Was Fixed

No product defect was found during the final walk. The first automation attempt compared serialized
JSON key order and produced a false negative; the scratch-only browser probe was corrected to compare
object values, and the entire Web journey was repeated from a fresh Loop definition.

## Paper Cuts

- The JSON field deliberately keeps an incomplete draft editable, but does not publish until the
  value parses. This was usable in the desktop editor and needs no product change for this PR.

## Runtime Errors Observed

None in the successful rewalk. Parent and child both reached `done`, and teardown found no surviving
daemon, Web, browser, or container process.

## Human Verifications Needed

None for the behavior covered here. Exact-head GitHub CI remains required before merge.

## Decisions for a Human

None.

## Impact Audit

- **Native tools:** Checked the existing generic Loop surfaces `compozy__loop_validate`,
  `compozy__loop_create`, `compozy__loop_edit`, and `compozy__loop_run`. This change adds no tool ID,
  toolset, descriptor, input schema, digest, or risk-class change; those tools receive the same Loop
  DSL and the closed `run-loop.params.config_overrides` validation is owned by the Loop runtime.
- **Extensibility and hooks:** The reserved `run-loop` DSL and its public documentation changed. No
  extension resource, hook event, registry projection, MCP contract, or configuration lifecycle was
  changed. CLI, HTTP, and UDS definition validation plus run behavior were checked through focused
  tests and the public Web/HTTP/CLI walk.
- **Workspace data isolation:** The parent carried the override only to its child in the same
  workspace. No stored Loop mutation, cross-workspace cache, event, or SSE contract changed. The
  independent child run and stored-definition reads proved per-run application without persistence.
- **Official Compozy skill:** `skills/compozy/references/loops.md` now lists the exact closed public
  fields, precedence, awaited/detached behavior, strict rejection, non-persistence, and a copyable
  YAML example. A fresh-context pressure test derived valid YAML and the child-only semantics without
  repository inspection or guessing.

## Evidence

- Web publish: `../evidence/2026-08-26-child-loop-config-overrides-web/editor-published.png`
- Web reload: `../evidence/2026-08-26-child-loop-config-overrides-web/editor-reloaded.png`
- Terminal parent: `../evidence/2026-08-26-child-loop-config-overrides-web/parent-run-done.png`
- Public and runtime truth: `../evidence/2026-08-26-child-loop-config-overrides-web/published-definition.json`,
  `../evidence/2026-08-26-child-loop-config-overrides-web/child-run-status.json`, and
  `../evidence/2026-08-26-child-loop-config-overrides-web/child-stored-config.json`
- Strict QA auditor: `../evidence/2026-08-26-child-loop-config-overrides-web/qa-audit-report.md` — PASS,
  zero blockers and zero warnings.
- Teardown: `../evidence/2026-08-26-child-loop-config-overrides-web/teardown.json` — exact
  `"clean": true`, survivors empty.

The evidence directory is generated and ignored from Git; Skeeper owns durable evidence transport.

## Final Status

- **Exit gate:** `make gate` PASS; Go race and the full affected Web lane passed (719 files / 6,360 tests).
- **Additional checks:** focused Web 66 tests, site 56 files / 322 tests, site build 2,436 pages, and
  strict QA evidence audit all passed.
- **Issues by user impact:** zero product findings; one scratch-probe false negative corrected outside
  product code.
- **Coverage:** Web authoring, malformed-input correction, publish, reload, HTTP round-trip, Web start,
  child runtime truth, stored non-persistence, and clean teardown.
- **Verdict:** local behavior is ready; merge readiness waits for the new exact-head PR CI round.
