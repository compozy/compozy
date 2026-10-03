# BUG-20261002-agent-context-stale-model: Agent context names the default model after another model runs

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-workspace-context, read the bound session's situation
- **Scenarios:** RT-031
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

An agent receives the wrong model in its situation context after the operator selects another
runtime. The session and identity endpoint agree on the effective model, while the context endpoint
and delivered prompt still name the agent default. Consumers cannot trust both public projections.

## Reproduction

- **Charter:** CH-agent-situation-context · **Tour:** Feature Tour
- **Environment:** isolated macOS daemon, local CLI/HTTP/UDS, real Codex provider, en-US.

1. Create a `general` session in Studio Operations under profile `studio`, whose default is
   Codex `gpt-6-luna`.
2. Send a project handoff request with `--provider codex --model gpt-5.6-sol --reasoning-effort low`.
3. Wait for the real turn to finish and read the created `project-handoff.md`.
4. Read the session resource and authenticated `/api/agent/me` and `/api/agent/context` over HTTP
   and UDS using that session's exact identity.

**Expected:** current session identity in both projections names `gpt-5.6-sol`, matching
`runtime.effective.model` and the delivered turn's runtime.

**Actual:** `/agent/me` names `gpt-5.6-sol`; `/agent/context.self.model` remains `gpt-6-luna` on
both transports. The persisted delivered situation also contains the stale default.

## Evidence

- `docs/qa/evidence/2026-10-02-untested/situation-after-handoff-context-http.json`
- `docs/qa/evidence/2026-10-02-untested/situation-after-handoff-context-uds.json`
- `docs/qa/evidence/2026-10-02-untested/situation-after-handoff-me-http.json`
- `docs/qa/evidence/2026-10-02-untested/situation-runtime-read.json`
- `docs/qa/evidence/2026-10-02-untested/situation-handoff-history.json`
- `docs/qa/evidence/2026-10-02-untested/situation-handoff-artifact-read.json`
- `docs/qa/evidence/2026-10-02-untested/situation-walk-ended.json`

## Fix

- **Root cause:** `Service.ContextForSession` takes `self.model` from the resolved agent
  definition and never reads `session.Info.Model`, although the session read model already
  carries the bound runtime model. Both the authenticated API and live prompt augmenter
  consume this projection, so both publish the configured default after an override.
- **Fix commit:** pending
- **Regression invariant:** current context identity uses the bound session model ahead of the
  agent default and pending next-prompt selection. The owning layer is the situation service;
  the canonical suite is `internal/situation/service_test.go`. Existing coverage already owns
  the no-runtime fallback. The startup snapshot is assembled before runtime binding; the fresh
  live context must supply the effective model after binding.

## Verification

The regression failed before the one-line precedence repair and passed afterward. The complete
situation race suite passes; package coverage is 63.5%, unchanged from the HEAD baseline and below
the repository's 80% target. The test-shape checker has eight unchanged legacy findings and no new
findings. Evidence: `situation-model-regression-red.log`, `situation-model-race-green.log`,
`situation-model-baseline-coverage.log`, and `situation-test-conventions-comparison.json` in this
cycle's evidence directory.

The required affected `make gate` passed code generation, Go lint, Go race suites, and the root
Turborepo Web lane. The first attempt found only formatting drift in the new test; the formatter
repair and successful rerun are recorded in `situation-model-gate-retry.log`.

Ada repeated the journey with fresh sessions on the rebuilt daemon. Codex `gpt-5.6-sol` completed
turn `turn-aa74e85930df5114` and wrote `release-checklist.md`, independently read from the project.
HTTP and UDS context, identity, and effective runtime agree on `gpt-5.6-sol`; the persisted fresh
turn context agrees too. Selecting `gpt-6-luna` for the next prompt leaves the current identity
unchanged, and clearing the selection succeeds. Invalid and stopped identities retain their 401
refusals; the healthy sibling and a fresh recovery session remain usable. All three new sessions
were stopped with verified receipts. The initial startup snapshot remains a historical snapshot
from logical session creation, separate from the corrected fresh turn context.

Replay evidence: `situation-fixed-context-*.json`, `situation-fixed-pending-*.json`,
`situation-fixed-history-read.json`, `situation-fixed-delivered-context-summary.json`,
`situation-fixed-usage.json`, `situation-fixed-artifact-read.json`, and
`situation-fixed-walk-ended.json` in `docs/qa/evidence/2026-10-02-untested/`.
