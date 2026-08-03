# QA Run Report — 2026-08-03 — PR 291 remediation

- **Scope:** Cross-platform shell shortcuts and stable window dragging; assistant text continuity across a permission decision and reload.
- **Cadence tier:** targeted
- **Build:** working tree after `b2536792` · **Environment:** isolated lab `compozy-pr-291-remediation-20260803-072136-467784-lab`, daemon `:53075`, current Web `:3001`
- **Started:** 2026-08-03T04:20:55-03:00 · **Status:** walks complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | isolated `pr-291-remediation` workspace | desktop / wifi-fast / en-US | CH-window-tabs-keyboard-flow |
| Théo | isolated `pr-291-remediation` workspace | desktop / wifi-fast / en-US | CH-session-prompt-identity · `sess-349f0185279f8457` |
| Cora | isolated `pr-291-remediation` workspace | laptop / wifi-fast / en-US | CH-window-tabs-home-canary |

## Flows in Scope

- `J-organize-tabbed-work` — group and recover related work through keyboard and pointer paths.
- `J-11` — return to a durable session without losing transcript content.
- `J-operate-home-dashboard` — adjacent shell canary after session and tab activity.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-window-tabs-keyboard-flow | J-organize-tabbed-work / ET-window-tab-deck-lifecycle | Bruno | Feature Tour | Pass | | this remediation batch |
| 2 | CH-session-prompt-identity | J-11 / RT-session-message-reload | Théo | Interrupt Tour | Pass | | this remediation batch |
| 3 | CH-window-tabs-home-canary | J-operate-home-dashboard / RT-home-usage-window-persistence | Cora | Feature Tour | Pass | | not applicable |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Bruno:** Command-T opened the destination picker, Settings joined Tasks in one deck, and the deck survived a full reload. Pointer drag moved Home through the stable head surface. The current Web E2E lane independently passed grouping preview/commit, persistence, the sole-pinned survivor, and the continuous-drag budget.
- **Théo:** A real Codex session under `approve-reads` rendered assistant text at sequence 8, requested permission at 37, recorded `allow-once` at 38, rendered completion text at 48, and ended at 52. Reload preserved both text segments around the permission boundary; structured CLI history returned the same chronology.
- **Visual capture:** the deterministic `SessionThread/FoldedTurns` Storybook capture shows every settled answer visible while reasoning/tool work remains collapsed. Capture: `docs/qa/evidence/2026-08-03-pr-291-remediation/session-thread-folded-turns.png` (1440×900, 42.5 KB); the Impeccable detector returned zero findings for the changed production files.
- **Cora:** After the session and reload, Home remained connected, retained the 30-day view, and rendered truthful real usage (`330K`, cost unavailable) plus the live session count.

## What Was Fixed

- Canonical primary shortcuts now map to Command on Apple platforms and Control elsewhere without weakening explicit physical Control chords.
- Every assistant text row remains transcript content across permission decisions; only reasoning and tool work fold.
- Window drag automation uses the stable empty topbar surface, sole-pinned close-others expectations match the product contract, and drag performance measures only continuous movement.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

No product failure remained in the verdict runs. The first setup attempt inherited the repository's `approve-all` overlay because the daemon started from the repository root; restarting from the isolated lab root applied `approve-reads`. An earlier `deny-all` attempt correctly rejected writes without an interactive prompt and did not contribute to the pass verdict.

The optional strict `eng-real-scenario-qa` audit returned its expected release-contract failure because this targeted lab has no four-actor/three-channel autonomy charter, task roots, disruption probe, or release verification report. Those checks are outside this targeted QA contract; the browser, provider, CLI history, and scenario-tracker evidence above own this verdict.

## Human Verifications Needed

None recorded yet.

## Decisions for a Human

None recorded yet.

## Learnings

- An isolated `COMPOZY_HOME` does not override a repository `config.toml` discovered from the daemon's current directory. Permission QA must start the daemon from the lab workspace when testing the isolated global config.
- Structured CLI history is the strongest independent proof for text/permission ordering: this run exposed sequences 3, 8, 37, 38, 48, and 52 without reading SQLite directly.

## Compozy Impact Audit

- **Native tools:** no impact; checked `compozy__*` IDs, descriptors, schemas, risk flags, and capability gates. The changes are Web shortcut, timeline rendering, and E2E behavior only.
- **Extensibility and hooks:** no impact; extension, hook, skill/capability, resource, registry, MCP sidecar, and config-lifecycle surfaces are unchanged.
- **Workspace data isolation:** no ownership change. Session transcript and window-manager reads remained bound to isolated workspace `ws_4e86a39ccfce6227`; no global/workspace/session/agent datum or propagation path changed.
- **Official Compozy skill:** no impact; public tools, CLI paths, hook events, capabilities, resources, and task semantics are unchanged.

Web/Docs impact: `web/` behavior and its canonical unit/E2E suites changed. `packages/site` has no impact because this restores the existing shortcut, permission-timeline, and window lifecycle contracts; the QA tracker is updated here.

## Final Status

- **Exit gate:** pass — affected `make gate` escalated to the full repository gate after the final tracked mutation; the current fingerprint and log are recorded by `make gate-status`. The one explicitly requested final `make gate-full` already ran before PR creation. All eight former PR-CI failures passed in the Web E2E lane; the corrected onboarding assertion also passed in its project runner.
- **Teardown:** pass — `qa-artifacts/qa/teardown.json` reports `clean: true`, killed registered Web PID `39895` and daemon PID `41772`, and found zero survivors.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 3/3 journeys walked.
- **Verdict:** QA and teardown pass — ready to commit and push.
