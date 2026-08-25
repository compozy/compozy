# QA Run Report — 2026-08-24 — ENG-135 session groups

- **Scope:** Sessions timeline grouped tool calls, live tail disclosure, stable expansion identity, and disclosure accessibility.
- **Cadence tier:** targeted
- **Build:** working tree for ENG-135 · **Environment:** isolated web lab; daemon seed blocked before transcript access
- **Started:** 2026-08-25T00:21:22Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Existing calm-transcript charter | desktop / wifi-fast / en-US | CH-session-calm-transcript |

## Flows in Scope

- `J-14` — Read a finished transcript and audit grouped work (`../journeys/J-14-read-a-finished-transcript.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-calm-transcript | J-14 / ET-web-session-transcript-calm-grammar | Théo | Feature Tour | Blocked (needs human verify) | Isolated daemon seed fails at `internal/demoseed/seed.go:79` because `GlobalDB.ListPresets` is unavailable; screenshot is in the lab evidence path below. | |

## Session Debriefs

### CH-session-calm-transcript — Théo

- **Ran:** 2026-08-25T00:21:22Z → 2026-08-25T00:25:00Z (box respected: yes)
- **Findings:** The web shell opened, but setup prevented entry to a finished session. The daemon seed compile error is outside ENG-135 and was not changed.
- **Bugs filed/updated:** none; this is a precondition/tooling blocker, not a transcript behavior finding.
- **Scenarios settled:** `ET-web-session-transcript-calm-grammar` → `blocked-verify`
- **Paper cuts:** none assessed because the transcript was unreachable.
- **Surprises:** the isolated seed path fails before daemon startup with an unavailable `GlobalDB.ListPresets` method.
- **Suggested next charter:** re-run this same charter after the seed/compiler blocker is repaired.

## What Was Fixed

- Settled consecutive tool runs now render truthful semantic summaries while preserving
  failures as individual rows and keeping the live tail within the existing four-row budget.
- Local work-group anchors survive streaming re-derivation, reorder, duplicate lifecycle
  events, lower-sorting arrivals, and live-to-settled failure splits.
- Summary, live-tail, turn-fold, and changed-file disclosures expose stable ARIA targets and
  keep hidden detail containers out of the layout until expanded.

## Paper Cuts

None assessed.

## Runtime Errors Observed

- `db.ListPresets undefined (type *globaldb.GlobalDB has no field or method ListPresets)` — isolated `go run ./scripts/demo-seed`; retained as a blocker for a future QA pass, not filed against ENG-135.

## Human Verifications Needed

- [ ] Repair or provide the isolated daemon seed path, then walk `CH-session-calm-transcript` from the finished-session entry point and verify grouping, failures, live tail, keyboard disclosure, refresh, and anchor preservation.

## Decisions for a Human

None.

## Learnings

- The focused web contract is covered by the canonical unit/component suites, but a production-parity transcript walk depends on the daemon seed compiling.

## Focused Verification

- `bunx turbo run test --filter=./web --force -- --run src/components/assistant-ui/__tests__/session-timeline.logic.test.ts src/components/assistant-ui/__tests__/timeline-scroll-anchoring.test.ts src/components/assistant-ui/__tests__/session-thread.test.tsx` — PASS (131 tests after the review remediation).
- `bunx turbo run typecheck --filter=./web` — PASS.
- Targeted `oxfmt --check` and `oxlint --deny-warnings` — PASS (0 warnings, 0 errors).
- First `make gate` — FAIL in the unrelated `use-window-manager-stream.test.tsx` test (`expected null to be "workspace:home"`); 713/714 web test files passed.
- Re-running that exact test through Turbo reproduced the same failure (10/11 tests passed); no out-of-scope changes were made.
- Second `make gate` — FAIL in the unrelated `web-storybook-visual-contract.test.ts` test after its 20s timeout; lint, typecheck, and codegen passed in that run.

## Final Status

- **Exit gate:** `make gate` was run twice; both runs were blocked by unrelated web tests recorded above. Full `make verify`/`make gate-full` was not run by task instruction and is intentionally deferred to CI.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 0/1 journeys walked; the only journey is explicitly blocked by the isolated daemon seed/compiler precondition.
- **Verdict:** ready with blocked items — merge the implementation only with the transcript QA re-walk scheduled after the seed blocker and unrelated local gate failures are repaired or cleared in CI.
