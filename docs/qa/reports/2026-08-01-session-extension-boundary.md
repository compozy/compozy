# QA Run Report — 2026-08-01 — session-extension-boundary

- **Scope:** PR #288 follow-up fixes for deferred session sandbox binding and workspace-scoped extension dev discovery
- **Cadence tier:** targeted
- **Build:** f28d7d1e + working tree · **Environment:** isolated local release-like binary at `http://127.0.0.1:64394`; public extension SDK publication is outside this run
- **Started:** 2026-08-01T14:04:04Z · **Completed:** 2026-08-01T14:17:43Z · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-session-sandbox-first-bind |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-extension-dev-link-invoke |

## Flows in Scope

- `J-15` — operate one logical session through public structured surfaces (`../journeys/J-15-operate-session-via-cli-api.md`)
- `J-extension-dev-lifecycle` — operate a workspace-scoped extension generation (`../journeys/J-extension-dev-lifecycle.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-sandbox-first-bind | J-15 / RT-session-sandbox-first-bind | Ada | Feature Tour | Pass | | working tree |
| 2 | CH-session-sandbox-first-bind | J-15 / RT-session-prompt-idempotency | Ada | Feature Tour | Pass | | a73b6587 |
| 3 | CH-extension-dev-link-invoke | J-extension-dev-lifecycle / ET-extension-dev-reload-loop | Bruno | Feature Tour | Pass | | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- Ada created `sess-da3796c17e1b24a8` as an unbound logical session in a workspace whose sandbox profile
  was `local`. A live Codex prompt bound it once, and an exact retry returned the stored turn with
  `replayed=true`. Fresh HTTP detail and transcript reads showed a prepared local sandbox, one authored
  message, and one `SANDBOX-READY` response.
- Bruno linked `boundary-search` using workspace registration `ws_fcd6ecd9076c58c6`. The daemon exposed
  the stable workspace identity `01KYYTH06HE0TR5WV2BTHQMEWA` consistently through dev, reload, invoke,
  HTTP list/status/logs, and remove. Reload changed both the generation hash and invoked behavior.

## What Was Fixed

- Deferred first-bind now derives sandbox intent from the immutable logical-session creation profile.
- The Host API session adapter preserves durable acceptance capabilities.
- Workspace-scoped extension operations normalize a public registration reference to the stable workspace
  identity at the daemon boundary instead of changing extension manager/provider identity semantics.
- E2E fixtures now bind logical sessions before reading runtime diagnostics and assert post-dispatch
  reasoning failures as indeterminate.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

- The lab initially lacked `defaults.provider`; the isolated config correctly rejected session creation
  before persistence. After setting `codex` and restarting the isolated daemon, the scenario passed.
- The strict real-scenario auditor reports six blockers because a generic targeted bootstrap does not
  manufacture the release-grade four-agent/Web startup scenario. This run is targeted QA, not
  `eng-real-scenario-qa` evidence; the audit report is retained at
  `qa-artifacts/qa/qa-audit-report.md` rather than weakened or satisfied with fictitious activity.

## Human Verifications Needed

- The published quickstart remains separately blocked on public Go/TypeScript SDK artifacts; this run does not claim `ET-extension-quickstart-verbatim` passed.

## Decisions for a Human

None.

## Learnings

- Public workspace registration IDs and stable workspace identities are deliberately different. The
  daemon service is the normalization boundary; extension manager/provider keys remain stable-identity
  contracts.
- Extension CLI `list` and `status` currently represent the global view; workspace-scoped read parity was
  independently exercised through the documented HTTP `workspace` query and through CLI operations that
  expose `--workspace`.

## Final Status

- **Exit gate (full automated suite):** Pending final repository gate after this report mutation.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** CLI, HTTP, runtime, real Codex ACP provider, real Go extension subprocess; no Web behavior
  changed in this remediation. Structured evidence:
  `/Users/pedronauck/dev/qa-labs/compozy-session-extension-boundary-20260801-140242-564687-lab/qa-artifacts/qa/boundary-verification.json`.
- **Teardown:** `/Users/pedronauck/dev/qa-labs/compozy-session-extension-boundary-20260801-140242-564687-lab/qa-artifacts/qa/teardown.json`
  reports `clean=true` with no survivors.
- **Verdict:** Targeted scenarios pass; public quickstart publication remains separately blocked-verify.
