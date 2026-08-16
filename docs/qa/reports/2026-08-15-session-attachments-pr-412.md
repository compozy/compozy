# QA Run Report — 2026-08-15 — Session attachments PR #412

- **Scope:** PR #412 session attachment upload, preview, dispatch, persistence, queueing, capability gates, and cleanup
- **Cadence tier:** targeted
- **Build:** historical behavioral walk `2603eed`; reviewed head `87c02571` · **Environment:** isolated local daemon and web app from `session-attachments-pr-412-20260815-103704-955265`
- **Started:** 2026-08-15T10:37:32Z · **Status:** HISTORICAL — superseded for the current capability correction

This report records the attachment walk completed before the unknown-versus-explicitly-unsupported
agent-capability correction. It is historical evidence only and cannot approve the current branch.

## Personas

| Persona | Base | Goal | Device / Network / Modality / Locale | Patience | Sessions |
|---|---|---|---|---|---|
| Théo | Returning Session User | Return to a long-lived background agent session and immediately see my persisted conversation, current and truthful, with the live run resuming. | desktop / wifi-fast / mouse-keyboard / en-US | 15 seconds | CH-session-attachments |

## Flows in Scope

- `J-session-attachments` — attach files once and trust their scope, order, retention, and cleanup (`../journeys/J-session-attachments.md`)

## Historical Session Matrix & Results

The matrix below records the pre-correction Feature Tour and retains the exact result of that historical
walk. The current charter uses Network Tour; a current run must record a new matrix and report after the
pending scenarios are walked.

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-attachments | J-session-attachments / ET-session-attachment-picker | Théo | Feature Tour | Fixed | BUG-20260815-session-attachment-store-unavailable | `2603eed` |
| 2 | CH-session-attachments | J-session-attachments / ET-session-attachment-paste-reload | Théo | Feature Tour | Pass | | `2603eed` |
| 3 | CH-session-attachments | J-session-attachments / ET-session-attachment-multiple-drop | Théo | Feature Tour | Pass | | `2603eed` |
| 4 | CH-session-attachments | J-session-attachments / ET-session-attachment-oversize | Théo | Feature Tour | Pass | | |
| 5 | CH-session-attachments | J-session-attachments / ET-session-attachment-unsupported-type | Théo | Feature Tour | Pass | | |
| 6 | CH-session-attachments | J-session-attachments / ET-session-attachment-model-gate | Théo | Feature Tour | Pass | | |
| 7 | CH-session-attachments | J-session-attachments / RT-session-queued-attachment-dispatch | Théo | Feature Tour | Pass | | |
| 8 | CH-session-attachments | J-session-attachments / RT-session-delete-attachment-files | Théo | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Théo attached files through picker, paste, and drop; each path produced truthful previews. A pasted image persisted through a provider turn and cold reload, while its scoped bytes route returned the expected 68-byte PNG and SHA-256. A busy-session prompt retained its attachment ref, dispatched once, and remained visible in history. Removing the stopped session deleted only its attachment tree.

## Current Follow-up

`ET-session-attachment-model-gate` is `untested` after the capability correction. The adjacent canary
`ET-web-session-composer-text-entry` is also `untested` because attachments now share its composer path.
Walk both scenarios through their owning charters in a fresh isolated QA run before recording a current
release verdict.

## What Was Fixed

- `BUG-20260815-session-attachment-store-unavailable`: reordered daemon boot so attachment storage exists before the session manager, HTTP server, and UDS server are constructed (`2603eed`).

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

- Initial supported uploads returned HTTP `503` before `2603eed`; the rerun returned `201` and completed the journey.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Content-based MIME detection refused disguised binary data despite its `.txt` extension.
- Unbound sessions keep image drafts visible but disable dispatch until ACP image capability is known.
- Content-addressed storage preserves scoped identity across composer input methods and reloads.

## Final Status

- **Historical behavioral evidence:** `docs/qa/evidence/2026-08-15-session-attachments-pr-412/`
- **Historical automated verify:** `/Users/pedronauck/dev/qa-labs/compozy-session-attachments-pr-412-20260815-103704-955265-lab/qa-artifacts/qa/final-make-verify.log`
- **Historical exit gate:** `make gate-full` for the pre-correction walk.
- **Historical React validation:** React Doctor 100/100 with no findings; root Bun lint, typecheck, and test gates passed.
- **Historical teardown:** `/Users/pedronauck/dev/qa-labs/compozy-session-attachments-pr-412-20260815-103704-955265-lab/qa-artifacts/qa/teardown.json` records `"clean": true` with no survivors.
- **Issues by user impact:** Blocks-Completion 1 fixed · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Historical coverage:** 8/8 attachment scenarios walked before the capability correction.
- **Current coverage:** 0/2 reset scenarios walked (`ET-session-attachment-model-gate`; `ET-web-session-composer-text-entry`).
- **Verdict:** NOT READY — the current capability correction requires a fresh targeted QA run.
