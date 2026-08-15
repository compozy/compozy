# QA Run Report — 2026-08-15 — Session attachments PR #412

- **Scope:** PR #412 session attachment upload, preview, dispatch, persistence, queueing, capability gates, and cleanup
- **Cadence tier:** targeted
- **Build:** `2603eed` · **Environment:** isolated local daemon and web app from `session-attachments-pr-412-20260815-103704-955265`
- **Started:** 2026-08-15T10:37:32Z · **Status:** PASS

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Returning Session User | desktop / wifi-fast / en-US | CH-session-attachments |

## Flows in Scope

- `J-session-attachments` — attach files once and trust their scope, order, retention, and cleanup (`../journeys/J-session-attachments.md`)

## Session Matrix & Results

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

## What Was Fixed

- `BUG-20260815-session-attachment-store-unavailable`: reordered daemon boot so attachment storage exists before the session manager, HTTP server, and UDS server are constructed (`2603eed`).

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

- Initial supported uploads returned HTTP `503` before `2603eed`; the rerun returned `201` and completed the journey.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

None identified yet.

## Learnings

- Content-based MIME detection refused disguised binary data despite its `.txt` extension.
- Unbound sessions keep image drafts visible but disable dispatch until ACP image capability is known.
- Content-addressed storage preserves scoped identity across composer input methods and reloads.

## Final Status

- **Behavioral evidence:** `docs/qa/evidence/2026-08-15-session-attachments-pr-412/`
- **Automated verify:** `/Users/pedronauck/dev/qa-labs/compozy-session-attachments-pr-412-20260815-103704-955265-lab/qa-artifacts/qa/intermediate-make-verify.log`
- **Exit gate (full automated suite):** pending final branch-close record; intermediate full gate passed at fingerprint `b531f503d9a098a46dd65d01faf0c7594c7343fa`.
- **Issues by user impact:** Blocks-Completion 1 fixed · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 8/8 scenarios walked
- **Verdict:** PASS — all attachment lifecycle scenarios passed after the daemon boot-order fix.
