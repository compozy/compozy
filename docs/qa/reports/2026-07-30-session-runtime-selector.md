# QA Run Report — 2026-07-30 — Session runtime selector

- **Scope:** Prompt-bound runtime selection across session creation, the session composer, structured prompt surfaces, runtime transitions, and truthful public documentation.
- **Cadence tier:** targeted
- **Build:** `5c564032` plus the current reviewed working tree · **Environment:** isolated local release lab at `http://127.0.0.1:57570`; normal browser profile; provider and parity details recorded below.
- **Started:** 2026-07-30T20:24:56Z · **Status:** behavioral PASS; automated closure is owned by the current gate record

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Casual User | desktop / wifi-fast / en-US | CH-site-docs-marketplace-truth, CH-session-launch-composer-handoff |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-untested-037-31-bruno, CH-new-session-latency-title, CH-cursor-agent-mode |
| Théo | Power User | desktop / wifi-fast / en-US | CH-016, CH-prompt-bound-runtime-transition |
| Sol | Accessibility-Reliant | desktop / wifi-fast / en-US | CH-untested-018-17-sol |
| Ada | Power User (structured surfaces) | desktop / wifi-fast / en-US | CH-prompt-runtime-fail-loud |

## Flows in Scope

- `J-evaluate-compozy-beta` — public documentation teaches the shipped first-session contract truthfully.
- `J-31` — agent detail remains visually and behaviorally coherent at the session entry point.
- `J-17` — launch one logical session, arrive at its composer, then choose the next prompt runtime (`../journeys/J-17-session-create-unified-selector.md`).
- `J-13` — follow a live session while prompt-bound runtime snapshots and queued work remain truthful.
- `J-21` — apply Claude reasoning at the prompt boundary or reject the request before dispatch (`../journeys/J-21-claude-reasoning-end-to-end.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-site-docs-marketplace-truth | J-evaluate-compozy-beta / ET-site-docs-first-session | Dora | Feature Tour | Pass | | |
| 2 | CH-untested-037-31-bruno | J-31 / ET-web-agent-detail-tab-parity | Bruno | Back-Button Tour | Pass | | |
| 3 | CH-new-session-latency-title | J-17 / RT-new-session-fast-feedback | Bruno | Network Tour | Fixed | BUG-20260730-session-create-window-intent | working tree |
| 4 | CH-cursor-agent-mode | J-17 / RT-cursor-agent-mode | Bruno | Feature Tour | Pass | | |
| 5 | CH-016 | J-13 / RT-018, RT-019, RT-059 | Théo | Multi-Tab Tour | Pass | | |
| 6 | CH-untested-018-17-sol | J-17 / ET-web-runtime-selector-minimal-slider, RT-068 | Sol | Feature Tour + accessibility lens | Pass | | |
| 7 | CH-session-launch-composer-handoff | J-17 / MS-web-session-simple-advanced-launch, RT-010, RT-063, ET-web-session-prompt-runtime-and-create-navigation | Dora | Feature Tour | Fixed | BUG-20260730-session-create-window-intent | working tree |
| 8 | CH-prompt-bound-runtime-transition | J-13 / RT-061, RT-064, RT-065, RT-066, RT-067, RT-070, RT-072, RT-session-prompt-runtime-transitions | Théo | Multi-Tab Tour | Fixed | BUG-20260730-session-provider-auth-availability | working tree |
| 9 | CH-prompt-runtime-fail-loud | J-21 / MS-057, RT-062 | Ada | Garbage Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

1. **Dora / docs:** rendered lifecycle, quick-start, and first-agent pages all separate creation from
   prompting; a real CLI-created session attached while live, exposed durable history, and became
   non-attachable after terminal stop.
2. **Bruno / agent detail:** Overview, Instructions, Configuration, and Sessions rendered their live
   contracts; the Sessions tab listed the provider-backed sessions from this lab.
3. **Bruno / fast launch:** the first post-onboarding create exposed a hydration race. After the fix,
   closing every session window and creating again materialized the composer in 1.2 seconds without
   reload.
4. **Bruno / Cursor:** Cursor Agent `2026.07.23-e383d2b` ran in agent mode and created the requested
   `cursor-runtime-proof.md` filesystem artifact.
5. **Théo / busy input:** a live Codex turn accepted queued Sol/low and Terra/high snapshots, exposed
   editable/removable queued rows, staged steer, and recorded interrupt with a new queue generation.
6. **Sol / keyboard:** search received focus on open, `End` selected Max, `Escape` restored trigger
   focus, and the favorite survived reload.
7. **Dora / launch handoff:** Simple showed agent only; Advanced owned workspace, name, path, and
   network; creation owned no prompt or runtime and navigated to the destination composer.
8. **Théo / transitions:** one logical session moved Sol/max → Terra/high through live
   configuration, then Claude Fable/max through process replacement. Earlier event snapshots did not
   change. Cursor selection removed unsupported reasoning. Signed-out providers were fixed to fail
   closed in the selector.
9. **Ada / fail loud:** the same unavailable Codex model returned `model_unavailable` over CLI,
   HTTP, and UDS; HTTP/UDS used 422, CLI used exit 71, no affected prompt dispatched, and the prior
   runtime remained usable.

## What Was Fixed

- `BUG-20260730-session-create-window-intent` — route reconciliation now retains a created-session
  intent until Window Manager hydration can materialize it, and retries an accepted-but-unapplied
  completion.
- `BUG-20260730-session-provider-auth-availability` — the session runtime hook now intersects the
  workspace provider allow-list with global auth availability and disables signed-out provider rows.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

- Expected disruption: `model_unavailable` with HTTP/UDS 422 and CLI exit 71.
- Expected operator cancellation: Codex returned JSON-RPC cancellation after interrupt; the transcript
  recorded prompt-cancel and prompt-interrupted markers.
- No unexpected browser, daemon, or provider error remained after the two fixes above.

## Human Verifications Needed

- None.

## Decisions for a Human

- None.

## Learnings

- Session launch navigation is a durable routing intent, not a one-shot side effect; hydration and
  accepted lifecycle completion can both temporarily decline to materialize a window.
- Workspace membership and provider authentication are separate server-owned projections. Runtime
  availability must intersect both before the selector exposes a model as actionable.
- A single persisted event history can prove live reconfiguration, process replacement, queued
  snapshot identity, and earlier-history immutability without using provider doubles.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` runs after this final repo mutation; the
  authoritative result is the current evidence record reported by `make gate-status`.
- **Issues by user impact:** Blocks-Completion 1 fixed · Data-Loss 0 · Trust-Damage 1 fixed · Friction 0 · Cosmetic 0
- **Coverage:** 9/9 charter walks; 23/23 changed scenarios settled (`pass`).
- **Verdict:** behavioral PASS; engineering completion additionally requires a current full-gate
  record, a strict evidence-audit pass, and clean lab teardown.
