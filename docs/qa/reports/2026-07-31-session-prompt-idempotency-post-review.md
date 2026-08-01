# QA Run Report — 2026-07-31 — Session prompt idempotency post-review

- **Scope:** Post-review verification of durable prompt identity, exact replay, conflict diagnostics, Goal response semantics, provider-echo suppression, live reconciliation, and cold reload.
- **Cadence tier:** targeted
- **Build:** current working tree · **Environment:** initial diagnostic lab `compozy-session-prompt-idempotency-post-review-20260801-072745-435459-lab`, followed by production-parity lab `compozy-session-prompt-idempotency-current-web-20260801-081126-391752-lab` using current `web/dist`.
- **Started:** 2026-08-01T07:27:45Z · **Finished:** 2026-08-01T08:25:50Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | `sess-5496ad3ba96f55b4` (invalid environment); `sess-64ed9d39e1b551fb` (production-parity pass) |

## Flows in Scope

- `J-11` — send one authored prompt, observe live settlement, and reload the exact permalink.
- `RT-session-prompt-idempotency` — replay the exact command and reject conflicting identity reuse.
- `RT-session-message-reload` — preserve one authored row in strict chronology through streaming and reload.

## Session Matrix & Results

| # | Journey / Scenario | Entry point | Persona | Status | Evidence |
|---|---|---|---|---|---|
| 1 | CH-session-prompt-identity / RT-session-message-reload | Web session thread · Interrupt Tour | Théo | Pass | `session-prompt-settled.png`; `session-prompt-reloaded-centered.png`; `session-prompt-reloaded-deterministic.png`; `cold-reload-dom-counts.json` |
| 2 | CH-session-prompt-identity / RT-session-prompt-idempotency | HTTP and CLI exact replay plus independent history | Théo | Pass | `structured-first.json`; `structured-replay.json`; `history-summary.json` |
| 3 | CH-session-prompt-identity / RT-session-prompt-idempotency | conflicting identity reuse | Théo | Pass | `identity-conflicts.json` |
| 4 | CH-session-prompt-identity / RT-session-prompt-idempotency | Goal replay status and CLI identity envelope | Théo | Pass | `goal-http-replay.json` |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debrief

- First isolated walk: onboarding selected Codex/GPT-5.6 Terra and the isolated workspace; session `sess-5496ad3ba96f55b4` attached successfully.
- Sending `Please reply with exactly ONE-ROW-OK.` rendered one optimistic row, then failed with `message_id is required` before provider dispatch. Investigation proved the daemon was serving its versioned embedded release bundle rather than the current `web/dist`; `BUG-20260801-web-prompt-missing-message-id` is invalid as a product finding. Teardown completed with `clean=true`; all matrix rows remain pending for a fresh production-parity lab.
- Fresh production-parity lab used the supported `COMPOZY_WEB_DIST_DIR` override. The Web prompt settled exactly once as `turn-3157dac135eba00d`, and a cold reload retained one authored row and one `ONE-ROW-OK` response.
- CLI command identity `3D07BC76-5AAE-4707-98D1-D111F1474EF7` / `A31CEA4B-7AAC-49CE-AFBB-287F6074C658` completed `turn-ae0a591ae48f3fef`. Its exact retry returned the stored turn with `replayed=true`; independent history contained one authored event and one response.
- Divergent reuse returned the two canonical conflict diagnostics. A Goal error returned 404 for both first admission and replay, with stable identities and `replayed` changing from false to true. A successful Goal replay preserved the original run and flat CLI identity fields.
- The fresh lab teardown killed the registered daemon and browser processes and recorded `teardown.json` with `clean=true`.

## Evidence Root

`/Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-current-web-20260801-081126-391752-lab/qa-artifacts/qa/`

## Runtime Errors Observed

- First lab only: stale embedded Web assets sent the legacy request to the current API. No current-build runtime error occurred in the production-parity rewalk.

## Human Verifications Needed

- None planned.

## Decisions for a Human

- None planned.

## Final Status

- Pass. Both targeted scenarios are `pass`; the stale-bundle finding is classified `invalid`, and the production-parity lab completed every matrix row.
