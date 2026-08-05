# QA Run Report — 2026-08-04 — session-rewind

- **Scope:** Same-session conversation rewind across Web, CLI, HTTP/UDS, native tools, storage, and ACP replay
- **Cadence tier:** targeted
- **Build:** working tree · **Environment:** isolated bootstrap lab with live Codex ACP
- **Started:** 2026-08-04T23:49:38-03:00 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | CH-rewind-conversation |
| Ada | Power User | native-tool / wifi-fast / en-US | structured parity checks |

## Flows in Scope

- `J-rewind-conversation` — abandon a mistaken path in place and continue from the retained prefix (`../journeys/J-rewind-conversation.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-rewind-conversation | J-rewind-conversation / RT-conversation-rewind | Théo | data-tour | Fixed | BUG-20260805-rewind-reader-unavailable | pending branch commit |
| 2 | CH-rewind-conversation | J-rewind-conversation / RT-conversation-rewind | Ada | feature-tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- The same logical session `sess-9a7fa66954cad263` kept its retained prefix, advanced to transcript epoch and generation 1, and restarted on fresh ACP session `019fcfff-bf37-7012-8963-b9c39b533583`.
- Default HTTP event reads excluded `DISCARD-ME`; explicit archived reads returned it for audit. The next live Codex answer was `RETAINED-CONTEXT-AFTER-REWIND`, proving that replay used only the retained prefix.
- The Web exposed `Rewind to here`, supported cancellation without mutation, and stated that files, tools, network calls, and saved memory are not undone.
- CLI rewind succeeded without manual fence flags by reading the transcript boundary first. HTTP, UDS, native-tool, storage, and Web adapters passed their canonical automated suites.

## What Was Fixed

- `BUG-20260805-rewind-reader-unavailable`: the read-only session database pool lease now forwards the rewind reader contract. A session database integration regression owns the invariant.

## Paper Cuts

None recorded so far.

## Runtime Errors Observed

- The first rewind returned `session: event recorder does not support conversation rewind`. The fix was applied to production code and the complete public-surface walk passed on retry.

## Human Verifications Needed

None identified so far.

## Decisions for a Human

None identified so far.

## Learnings

- A capability implemented by the concrete session recorder must also be forwarded by every recorder lease used by public read paths.
- Saved workspace memory correctly remains outside the rewind boundary; only the ACP conversation context and active transcript suffix are rewound.

## Final Status

Pass. The targeted cross-surface walk and live provider proof completed. Visual evidence is at `docs/qa/evidence/session-rewind/rewind-confirmation.png`; structured evidence is in the isolated lab's `qa/verification-report.md`. Teardown completed with `qa/teardown.json` reporting `"clean": true` and no survivors. The workstream closes only after the repository full gate records a current passing fingerprint.
