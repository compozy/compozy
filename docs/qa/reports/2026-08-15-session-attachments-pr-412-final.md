# QA Run Report — 2026-08-15 — Session attachments PR #412 final

- **Scope:** PR #412 attachment input, capability admission, dispatch, reload, cleanup, and adjacent composer text entry
- **Cadence tier:** targeted
- **Build:** b265dbaf plus the final QA remediation batch · **Environment:** fresh isolated daemon and Web lab session-attachments-pr-412-final-20260815-195219-431614
- **Started:** 2026-08-15T19:52:51Z · **Status:** passed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | CH-session-attachments |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-session-composer-text-entry |

## Flows in Scope

- `J-session-attachments` — attach files once and trust their scope, capability admission, persistence, and cleanup (`../journeys/J-session-attachments.md`)
- `J-17` — preserve exact composer input while selecting the next-prompt runtime (`../journeys/J-17-session-create-unified-selector.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-attachments | J-session-attachments / ET-session-attachment-model-gate | Théo | Network Tour | Fixed | BUG-20260815-session-window-runtime-selector-loop | QA remediation batch |
| 2 | CH-session-attachments | J-session-attachments / ET-session-attachment-picker | Théo | Network Tour | Pass | | |
| 3 | CH-session-attachments | J-session-attachments / ET-session-attachment-paste-reload | Théo | Network Tour | Pass | | |
| 4 | CH-session-attachments | J-session-attachments / ET-session-attachment-multiple-drop | Théo | Network Tour | Pass | | |
| 5 | CH-session-attachments | J-session-attachments / ET-session-attachment-oversize | Théo | Network Tour | Fixed | BUG-20260815-session-attachment-cli-closed-pipe | QA remediation batch |
| 6 | CH-session-attachments | J-session-attachments / ET-session-attachment-unsupported-type | Théo | Network Tour | Pass | | |
| 7 | CH-session-attachments | J-session-attachments / RT-session-queued-attachment-dispatch | Théo | Network Tour | Pass | | |
| 8 | CH-session-attachments | J-session-attachments / RT-session-delete-attachment-files | Théo | Network Tour | Pass | | |
| 9 | CH-session-composer-text-entry | J-17 / ET-web-session-composer-text-entry | Bruno | Feature Tour | Fixed | BUG-20260815-session-composer-draft-reload | QA remediation batch |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Théo:** A live Codex ACP binding on GPT-5.6 Terra negotiated image and embedded-context support. Picker, paste, and multi-file drop produced ready previews; the model answered from image content; a queued image dispatched once; cold reload preserved transcript media; wrong-workspace reads returned 404; session removal deleted owned files. Direct daemon delivery showed precise oversized and unsupported-file errors.
- **Bruno:** Sequential Unicode text and repeated spaces survived the runtime selector, full reload, and deep-link return after the session draft store gained a browser persistence owner.

## What Was Fixed

- The session window runtime selector now uses a stable shallow comparison, preventing a React maximum-update loop.
- CLI multipart uploads now publish exact length and preserve daemon rejections instead of replacing them with closed pipe.
- The browser persists only validated, non-empty session drafts; first-prompt handoffs and Goal feedback remain memory-only.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Théo | Oversized upload through the Vite proxy | The proxy converted an early daemon 413 into a generic 502. | Medium | Repeated against the direct local Web bundle; the product rendered the precise limit error. No product defect remained. |

## Runtime Errors Observed

- The original session render produced a maximum update depth error; fixed and covered by the controller suite.
- The original oversized CLI request produced closed pipe; fixed and covered by the UDS client suite.
- No unresolved browser console error, daemon panic, attachment leak, or cross-workspace byte read remained.

## Human Verifications Needed

None planned.

## Decisions for a Human

None.

## Learnings

- ACP capability presence must remain tri-state: unknown before a matching live binding, explicit true/false after negotiation.
- Browser QA for local Web changes must serve web/dist through COMPOZY_WEB_DIST_DIR; the normal binary intentionally embeds the pinned published web-assets module.
- Early streaming rejection needs exact request length and response-first error ownership across both HTTP and UDS clients.

## Final Status

- **Targeted QA verdict:** PASS
- **Exit gate (full automated suite):** workstream-close make gate-full runs after the final QA mutation.
- **Final make verify evidence:** qa-artifacts/qa/final-make-verify.log
- **Issues by user impact:** Blocks-Completion 1 fixed · Data-Loss 1 fixed · Trust-Damage 1 fixed · Friction 0 · Cosmetic 0
- **Coverage:** 2/2 journeys walked; 9/9 scenarios settled
- **Verdict:** ready for the workstream-close gate — targeted real-provider QA passed with every discovered issue fixed and re-walked.
