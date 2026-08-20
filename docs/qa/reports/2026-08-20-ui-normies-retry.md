# QA Run Report — 2026-08-20 — Normie-friendly UI retry

- **Scope:** Targeted retry for the normie-friendly UI foundation pass: first-run Home, populated Home, desktop typography and navigation, default window fit, settings aliases, onboarding, transcript, decision dock, and the empty-catalog canary.
- **Cadence tier:** targeted
- **Build:** working tree over `origin/main` · **Environment:** first isolated lab `/Users/pedronauck/dev/qa-labs/compozy-ui-normies-20260820-retry-20260820-183850-874109-lab/qa-artifacts/qa/bootstrap-manifest.json` reproduced `BUG-20260820-global-home-deleted-onboarding` and closed with `teardown.json` clean and no survivors; all verdict walks use the genuinely fresh replacement manifest `/Users/pedronauck/dev/qa-labs/compozy-ui-normies-20260820-retry-20260820-191554-013424-lab/qa-artifacts/qa/bootstrap-manifest.json`, runtime API `http://127.0.0.1:52760`, manifest-derived Web proxy target, and a fresh browser session. Safari, Firefox, physical mobile devices, and screen-reader speech output remain parity boundaries unless exercised during the run.
- **Started:** 2026-08-20T15:38:50-03:00 · **Closed:** 2026-08-20T19:17:32Z · **Status:** Skipped by explicit user instruction
- **Automated precondition:** PASS — `rtk make gate` escalated to `make verify` and passed for fingerprint `1c68d35828f3c063c576edd046cce8f807bdf4bf`; evidence `.cache/gate/logs/full-1787248788.log` and lab copy `/Users/pedronauck/dev/qa-labs/compozy-ui-normies-20260820-retry-20260820-183850-874109-lab/qa-artifacts/qa/make-verify.log`.
- **Post-fix scoped evidence:** PASS — canonical onboarding workspace suite 11/11, Web lint with zero warnings/errors, and Web typecheck. A post-fix `make gate` was started but explicitly waived by the user in favor of CI; its subprocess tree was confirmed absent before replacing the lab. No post-fix full-gate claim is made.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | New User | laptop / wifi-fast / en-US | CH-home-zero-inventory-first-start, CH-untested-019-19-lea |
| Cora | Casual User | laptop / wifi-fast / en-US | CH-untested-069-operate-home-dashboard-cora |
| Sol | Accessibility-Reliant | desktop / wifi-fast / en-US | CH-plain-scale-legibility |
| Théo | Session Operator | desktop / wifi-fast / en-US | CH-session-permission-dock, CH-session-calm-transcript |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-untested-068-operate-desktop-shell-bruno, CH-empty-catalog-first-use |
| Dora | Runtime Administrator | desktop / wifi-fast / en-US | CH-untested-041-administer-runtime-settings-dora |

## Flows in Scope

- `J-operate-home-dashboard` — start honestly from zero, then read real work in seven truthful zones (`../journeys/J-operate-home-dashboard.md`)
- `J-operate-desktop-shell` — reach and operate apps through one coherent desktop shell (`../journeys/J-operate-desktop-shell.md`)
- `J-19` — choose a default runtime while first-run setup owns the shell (`../journeys/J-19-onboarding-default-model.md`)
- `J-administer-runtime-settings` — find and apply runtime settings safely (`../journeys/J-administer-runtime-settings.md`)
- `J-14` — audit a calm, truthful finished transcript (`../journeys/J-14-read-a-finished-transcript.md`)
- `J-answer-agent-requests` — decide permissions and clarifications at the composer (`../journeys/J-answer-agent-requests.md`)
- `J-start-from-empty-catalogs` — reach real next actions from empty catalogs (`../journeys/J-start-from-empty-catalogs.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-home-zero-inventory-first-start | J-operate-home-dashboard / RT-home-zero-inventory-first-run | Lea | Feature Tour | Skipped | `BUG-20260820-global-home-deleted-onboarding` fixed; same-persona rewalk skipped by explicit user instruction | working tree |
| 2 | CH-untested-019-19-lea | J-19 / RT-onboarding-setup-panel-over-shell | Lea | Feature Tour | Skipped | `BUG-20260820-global-home-deleted-onboarding` fixed; same-persona rewalk skipped by explicit user instruction | working tree |
| 3 | CH-plain-scale-legibility | J-operate-desktop-shell / ET-web-geist-wght-medium-510 | Sol | Feature Tour | Skipped | Explicit user instruction; no behavioral verdict | |
| 4 | CH-untested-068-operate-desktop-shell-bruno | J-operate-desktop-shell / ET-web-catalog-navigation; ET-web-dock-default-window-size | Bruno | Feature Tour | Skipped | Explicit user instruction; no behavioral verdict | |
| 5 | CH-untested-041-administer-runtime-settings-dora | J-administer-runtime-settings / MS-web-settings-takeover-redesign | Dora | Back-Button Tour | Skipped | Explicit user instruction; no behavioral verdict | |
| 6 | CH-session-calm-transcript | J-14 / ET-web-session-transcript-calm-grammar | Théo | Feature Tour | Skipped | Explicit user instruction; no behavioral verdict | |
| 7 | CH-session-permission-dock | J-answer-agent-requests / ET-web-session-permission-dock | Théo | Feature Tour | Skipped | Explicit user instruction; no behavioral verdict | |
| 8 | CH-untested-069-operate-home-dashboard-cora | J-operate-home-dashboard / RT-home-dashboard-zones | Cora | Feature Tour | Skipped | Explicit user instruction; no behavioral verdict | |
| 9 | CH-empty-catalog-first-use | J-start-from-empty-catalogs / TA-web-tasks-zero-inventory-templates; TA-web-jobs-zero-inventory-suggestions; TA-web-triggers-zero-inventory-intro | Bruno | Feature Tour | Skipped | Adjacent canary skipped by explicit user instruction; its unrelated tracker rows were not changed | |

Status legend: `Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Initial lab — Lea onboarding and Home entry

- The setup panel owned focus, resisted Escape dismissal, preserved the shell underneath, and fit at 1280×800 and 768×800.
- Removing the suggested Home folder and finishing in Global exposed `BUG-20260820-global-home-deleted-onboarding`: Home focused but did not open after a clean reload and retry.
- Root cause was fixed in the onboarding workspace hook and its canonical suite. The complete Lea replay will run from zero in a replacement lab; no verdict is carried forward from the affected state.

### Closure

- The replacement lab was allocated fresh and started only its isolated daemon. Before any Web session or scenario walk, the user explicitly directed the worker to stop all remaining QA and close.
- All nine flagged scenarios, the adjacent canary, tours, edges, six-lens passes, screenshots, real-provider legs, refresh/deep-link checks, and strict audit were skipped. No pass or parity claim is inferred from the partial affected-lab observations.

## Experiential Lens Pass

Skipped by explicit user instruction. No lens verdict was recorded.

## Edge and Tour Results

Skipped by explicit user instruction. No tour or edge verdict was recorded.

## What Was Fixed

- `BUG-20260820-global-home-deleted-onboarding` — the onboarding draft now excludes the operator-home registration, rejects attempts to add it as a project, and never deletes it when cleaning a stale draft.
- Invariant: the internal Global registration never becomes a selectable/removable onboarding project. Owning layer: React hook. Canonical suite: `web/src/systems/onboarding/hooks/__tests__/use-onboarding-workspaces.test.tsx` (11/11).

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

- Agent-browser's instrumentation process relaunched once during the first runtime-selector attempt; a clean retry worked. This is a tooling parity issue, not product evidence.
- Product stall: Home Dock selection produced no window after Global's internal workspace was deleted by onboarding (`BUG-20260820-global-home-deleted-onboarding`).

## Human Verifications Needed

None assigned. The remaining legs were skipped by instruction rather than classified as human-only blockers.

## Decisions for a Human

None at run start.

## Parity Gaps

- Browser policy is the manifest-provided browser-use/agent-browser path on the local production-like Web surface.
- Safari, Firefox, physical mobile devices, and spoken screen-reader output are not yet exercised; keyboard, accessibility-tree, viewport, reduced-motion, and computed-style checks remain in scope.
- Provider behavior follows the manifest home policy; any unavailable real-provider leg will be recorded as blocked rather than simulated.

## Learnings

- The retry entered behavior-first QA with a current full verification record; the earlier formatting blocker is closed.
- Onboarding must reuse the workspace system's operator-home partition. Treating all daemon registrations as selectable projects let a first-run action delete Global's internal owner and silently disabled the desktop.

## Compozy Impact Audit

- **Native tools:** no impact; checked the onboarding workspace fix and it changes no `compozy__*` ids, descriptors, schemas, digests, gates, or CLI/API fallbacks.
- **Extensibility and hooks:** no impact; the fix only filters the existing operator-home registration from the onboarding project draft and changes no extensions, hooks, capabilities, resources, registries, bridge SDKs, MCP sidecars, or config keys.
- **Workspace data isolation:** the changed datum is the existing global-scoped operator-home registration. The fix reuses `partitionProjectWorkspaces` so the internal row remains the Global runtime binding and cannot be deleted as a project; it changes no workspace id propagation, SSE, cache, or event contract.
- **Official Compozy skill:** no impact; checked `skills/compozy/` ownership and no public command, tool id, hook event, capability, resource, memory, network, or task behavior changed.

## Final Status

- **Exit gate:** pre-fix full verification PASS; post-fix canonical suite, Web lint, and Web typecheck PASS. The user explicitly assigned all further gates to CI, so the interrupted post-fix `make gate` and `make gate-full` are skipped without a completion claim.
- **Strict evidence audit:** Skipped by explicit user instruction; no audit pass is claimed. Current full-gate C14 evidence was also waived in favor of CI.
- **Teardown:** PASS for both labs. Affected lab: `/Users/pedronauck/dev/qa-labs/compozy-ui-normies-20260820-retry-20260820-183850-874109-lab/qa-artifacts/qa/teardown.json`, `clean: true`, `survivors: []`. Replacement lab: `/Users/pedronauck/dev/qa-labs/compozy-ui-normies-20260820-retry-20260820-191554-013424-lab/qa-artifacts/qa/teardown.json`, `clean: true`, `survivors: []`.
- **Issues by user impact:** Blocks-Completion 1 fixed but not behaviorally reverified · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0.
- **Coverage:** 0/9 sessions completed; all nine flagged scenarios recorded `skipped`; adjacent canary skipped without changing its unrelated tracker rows.
- **Verdict:** Skipped — the defect was fixed and scoped checks passed, but all remaining behavioral QA and audit were stopped by explicit user instruction. No passing behavioral verdict is claimed.
