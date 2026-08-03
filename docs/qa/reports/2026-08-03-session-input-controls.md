# QA Run Report — 2026-08-03 — Session Input Controls

- **Scope:** Durable Queue, Steer, and Interrupt across daemon/public surfaces plus calm web feedback, one working indicator, and removal of the visible “Next prompt” label.
- **Cadence tier:** targeted
- **Build:** `0385983a` + final session-input worktree · **Environment:** fresh isolated Compozy QA lab at `/Users/pedronauck/dev/qa-labs/compozy-session-input-controls-20260803-222842-220527-lab`
- **Started:** 2026-08-03T22:27:38Z · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | CH-016, CH-session-calm-transcript |
| Sol | Accessibility-Reliant | desktop / wifi-fast / en-US | CH-020 |

## Flows in Scope

- `J-13` — Follow a live run and trust durable busy-input controls (`../journeys/J-13-follow-a-live-run.md`)
- `J-14` — Read a calm, truthful finished transcript (`../journeys/J-14-read-a-finished-transcript.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-016 | J-13 / RT-019 | Théo | Multi-Tab Tour | Pass | | final batch |
| 2 | CH-020 | J-13 / RT-059 | Sol | Back-Button Tour | Pass | | final batch |
| 3 | CH-session-calm-transcript | J-14 / ET-web-session-transcript-calm-grammar | Théo | Feature Tour | Pass | | final batch |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-016 — Durable busy input

- Queued two inputs during a deterministic 90-second turn, canceled one, replaced the other, and confirmed the daemon returned the same authoritative row after a browser refresh.
- Promoted a queued row to Steer in the web UI. The fenced active turn was canceled, the selected replacement was dispatched once, and the remaining queued item followed in FIFO order.
- Repeated the critical path against a real Codex provider session (`sess-8a3ded8e5a329240`). The live follow-up was admitted through the CLI, promoted through the web, and completed with a healthy idle session.
- Public parity evidence includes CLI/UDS mutations, HTTP queue reads, browser controls, transcript history, native-tool/extension contract tests, and the generated OpenAPI/SDK contract.

### CH-020 — Composer semantics and accessibility

- The active composer kept Queue, Steer, Interrupt, and Stop visible while the durable row remained above it.
- The visible “Next prompt” label is gone; the runtime selector retains its accessible name for screen readers.
- Text cleared only after daemon acknowledgement. Refresh preserved pending rows, and edit/promote/cancel targeted the durable row ID rather than a client-only copy.

### CH-session-calm-transcript — Calm lifecycle feedback

- Normal queue, steer, interrupt, accepted, canceled, and dropped lifecycle markers remained in durable history but did not render as warning rows.
- Exactly one working indicator appeared above the composer in live and deterministic active turns.
- Settled transcripts showed the replacement and remaining queued turn without duplicate success notices or warning noise.

## What Was Fixed

The first interrupt walk exposed one remaining trust defect: the expected ACP cancellation was still projected as `transcript_marker.provider_failure` even though the replacement prompt succeeded. The projection now suppresses expected `store.FailureCanceled` failures while retaining the technical error event, and `TestPromptTranscriptMarkerSuppressesExpectedCancellation` owns the regression. The post-fix interrupt walk passed without a warning row.

## Paper Cuts

None recorded.

## Runtime Errors Observed

The deterministic provider emits a cancellation error event when an active turn is interrupted. This is expected transport truth and is no longer misrepresented as a provider-failure transcript marker.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Durable admission must be separate from visual acknowledgement: the daemon owns the row and the web only projects it.
- An expected cancellation remains observable for diagnostics but is not a user-facing provider failure.
- A real provider pass is necessary because deterministic timing fixtures cannot prove provider protocol behavior.

## Final Status

- **Behavioral evidence:** `docs/qa/evidence/2026-08-03-session-input-controls/01-durable-queue-working.png`, `02-steer-settled-calm.png`, `04-interrupt-retest-calm.png`, and `05-live-codex-queue-working.png`
- **Deterministic visual evidence:** `docs/qa/evidence/2026-08-03-session-input-controls/storybook/busy-input-controls.png` and `queued-composer.png`
- **Lab evidence:** `qa/bootstrap-manifest.json`, `qa/journey-log.jsonl`, `qa/provider-attempt.json`, and the generated strict audit under the lab `qa-artifacts` directory
- **Exit verification:** The final `make gate-full` attempt reached `BunLint` and exposed one `SessionComposer` complexity error. That error was fixed before publication, then `make bun-lint`, the web typecheck, and the canonical 42-test session-thread suite passed. The user explicitly waived another full-gate rerun because the PR will enter a review/remediation cycle.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 2/2 journeys and 3/3 changed scenarios walked
- **Verdict:** PASS — behavioral QA is complete, and the known gate failure was corrected and verified by its owning scoped checks before PR publication.
