# QA Run Report — 2026-07-31 — Session prompt idempotency

- **Scope:** Durable prompt identity, exact replay, provider-echo suppression, live transcript reconciliation, and cold reload across Web plus structured session-prompt surfaces.
- **Cadence tier:** targeted
- **Build:** current working tree · **Environment:** isolated lab `compozy-session-prompt-idempotency-20260801-040518-847041-lab`; daemon `http://127.0.0.1:53003`; Web proxy derives from the bootstrap manifest.
- **Started:** 2026-08-01T03:57:49Z · **Finished:** 2026-08-01T06:02:37Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | `sess-c260441ced7d7b51` |

## Flows in Scope

- `J-11` — send one authored prompt, observe live settlement, and reload the exact permalink.
- `RT-session-prompt-idempotency` — replay one command by its original identities and reject conflicting reuse.
- `RT-session-message-reload` — preserve one authored row in strict chronology through streaming and cold reload.

## Session Matrix & Results

| # | Journey / Scenario | Entry point | Persona | Status | Evidence |
|---|---|---|---|---|---|
| 1 | CH-session-prompt-identity / RT-session-message-reload | Web session thread · Interrupt Tour | Théo | Pass | `session-prompt-settled.png`; `session-prompt-reloaded-deterministic.png`; `browser-reload-assertion.json` |
| 2 | CH-session-prompt-identity / RT-session-prompt-idempotency | HTTP and CLI plus independent transcript read | Théo | Pass | `prompt-replay.json`; `transcript-summary.json` |
| 3 | CH-session-prompt-identity / RT-session-prompt-idempotency | conflicting identity reuse | Théo | Pass | `prompt-idempotency-conflict.json`; `prompt-message-identity-conflict.json` |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debrief

- The first Web prompt used `message_id=we0Y9M6vnuXWdXhv` and `idempotency_key=386891c0-fb0b-4531-bb46-54c148617984`. It completed as `turn-f93e70b2d3d9ee2d` with the exact provider response `QA-OK`.
- The settled thread and cold-reloaded permalink each contained exactly one authored row and one assistant row. The deterministic 1440×900 capture was produced through the project CDP screenshot flow after seeding only the active workspace selection in its disposable browser profile.
- Identical HTTP and CLI retries returned `202`, `replayed=true`, and the original turn without a second SSE stream. Reusing the key with another message returned `prompt_idempotency_conflict`; reusing the message id with another key returned `prompt_message_identity_conflict`.
- The public CLI history independently showed one durable `user_message` at sequence 3 with the exact authored text and original message id, followed by one `agent_message` (`QA-OK`) at sequence 6 and turn completion at sequence 10.

## Evidence Root

`/Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-20260801-040518-847041-lab/qa-artifacts/qa/`

The evidence root contains the settled/reloaded screenshots, exact replay receipt, both structured conflict receipts, independent transcript summary, browser row-count assertion, bootstrap manifest, registered process IDs, and teardown record.

## Runtime Errors Observed

- None during the isolated public-surface walk.

## Human Verifications Needed

- None planned.

## Decisions for a Human

- None planned.

## Final Status

- **QA exit gate:** the isolated walk and deterministic screenshot passed. `make gate` passed the full monorepo verification for fingerprint `80e08a55c8ae331557e69976b45252b8de7ecab7` before this report-only mutation; the workstream's single final `make gate-full` remains intentionally deferred until after review and PR feedback freeze.
- **Teardown:** `qa/teardown.json` completed at `2026-08-01T06:04:08Z` with `clean=true`, no survivors, and both registered daemon/browser PIDs stopped.
- **Verdict:** pass — 3/3 matrix rows passed, no new bug was found, and the reopened duplicate-message bug is fixed.
