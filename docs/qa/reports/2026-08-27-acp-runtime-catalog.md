# QA Run Report — 2026-08-27 — ACP runtime catalog

- **Scope:** Automatic ACP model discovery and refresh, logical runtime controls, provider strategies, agent defaults, prompt transitions, and the shared Web Runtime Selector.
- **Cadence tier:** targeted
- **Build:** `3deaa1718` plus uncommitted deep-review remediation · **Environment:** isolated lab `acp-runtime-catalog-20260828-004625-083662`
- **Started:** 2026-08-27T21:45:59-03:00 · **Finished:** 2026-08-28T00:37:50-03:00 · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-acp-runtime-catalog-refresh, CH-agent-runtime-default-options |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-prompt-runtime-option-order |
| Théo | Power User | desktop / wifi-fast / en-US | CH-provider-runtime-strategies |
| Dora | Power User | desktop / wifi-fast / en-US | CH-session-launch-composer-handoff |
| Lea | New User | laptop / wifi-fast / en-US | CH-030 |

## Flows in Scope

- `J-20` — Curate and refresh the model catalog through structured public surfaces (`../journeys/J-20-catalog-curation-agent-surfaces.md`).
- `J-18` — Author an agent with reusable runtime controls (`../journeys/J-18-agent-create-gains-reasoning.md`).
- `J-21` — Apply prompt runtime options truthfully and in order (`../journeys/J-21-claude-reasoning-end-to-end.md`).
- `J-17` — Launch a session and choose its next-prompt runtime (`../journeys/J-17-session-create-unified-selector.md`).
- `J-19` — Choose a default runtime during onboarding (`../journeys/J-19-onboarding-default-model.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-acp-runtime-catalog-refresh | J-20 / MS-live-model-release-refresh | Ada | Feature Tour | Pass | | |
| 2 | CH-acp-runtime-catalog-refresh | J-20 / MS-cursor-account-model-discovery | Ada | Feature Tour | Pass | | |
| 3 | CH-acp-runtime-catalog-refresh | J-20 / RT-hermes-live-model-readiness | Ada | Feature Tour | Pass | | |
| 4 | CH-acp-runtime-catalog-refresh | J-20 / MS-042 | Ada | Feature Tour | Pass | | |
| 5 | CH-acp-runtime-catalog-refresh | J-20 / RT-model-catalog-cold-open | Ada | Feature Tour | Pass | | |
| 6 | CH-agent-runtime-default-options | J-18 / MS-web-agent-create-simple-advanced | Ada | Feature Tour | Pass | | |
| 7 | CH-agent-runtime-default-options | J-18 / RT-070 | Ada | Feature Tour | Fixed | BUG-20260827-default-profile-agent-prompt-resolution; BUG-20260827-cursor-launch-model-negotiation | pending remediation commit |
| 8 | CH-prompt-runtime-option-order | J-21 / RT-061 | Bruno | Feature Tour | Pass | | |
| 9 | CH-prompt-runtime-option-order | J-21 / RT-session-prompt-runtime-transitions | Bruno | Feature Tour | Fixed | BUG-20260827-cursor-launch-model-negotiation; BUG-20260827-unbound-session-fast-inheritance | pending remediation commit |
| 10 | CH-provider-runtime-strategies | J-17 / RT-cursor-logical-runtime-options | Théo | Feature Tour | Fixed | BUG-20260827-cursor-launch-model-negotiation; BUG-20260827-live-uncurated-model-admission | pending remediation commit |
| 11 | CH-provider-runtime-strategies | J-17 / RT-openclaw-provider-managed-runtime | Théo | Feature Tour | Blocked (needs human verify) | OpenClaw CLI absent from isolated provider home | |
| 12 | CH-provider-runtime-strategies | J-17 / RT-session-runtime-selection-continuity | Théo | Feature Tour | Fixed | BUG-20260828-session-runtime-restart-projection | pending remediation commit |
| 13 | CH-provider-runtime-strategies | J-17 / ET-web-runtime-selector-minimal-slider | Théo | Feature Tour | Pass | | |
| 14 | CH-session-launch-composer-handoff | J-17 / MS-web-session-simple-advanced-launch | Dora | Feature Tour | Fixed | BUG-20260827-session-create-first-message-regression; BUG-20260827-unbound-session-fast-inheritance | pending remediation commit |
| 15 | CH-030 | J-19 / RT-071 | Lea | Feature Tour | Fixed | BUG-20260827-grok-xhigh-onboarding-rejected | pending remediation commit |
| 16 | CH-agent-runtime-default-options | J-18 / RT-029 | Ada | Feature Tour | Fixed | BUG-20260827-native-agent-create-profile | pending remediation commit |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-030 — Lea

- **Ran:** 2026-08-27T21:49:00-03:00 → 2026-08-27T21:55:00-03:00 (box respected: yes)
- **Findings:** Grok 4.6 with Extra high reasoning was visibly advertised but rejected on Continue, blocking the first-run journey.
- **Bugs filed/updated:** BUG-20260827-grok-xhigh-onboarding-rejected
- **Scenarios settled:** RT-071 → fixed/pass after a fresh browser retest
- **Paper cuts:** none; the failure was functional.
- **Surprises:** CLI, HTTP, native-tool, and Web catalog reads agreed on `xhigh`; only the settings commit disagreed.
- **Retest:** Cursor Grok 4.6 + Extra high advanced to Workspace and persisted the logical runtime; screenshots and fresh settings reads captured.
- **Suggested next charter:** Continue CH-provider-runtime-strategies.

### CH-agent-runtime-default-options — Ada (structured create parity)

- **Ran:** 2026-08-27T21:56:00-03:00 → 2026-08-27T22:11:00-03:00 (box respected: yes)
- **Findings:** Web, CLI, and HTTP authored the selected logical runtime, but `compozy__agent_create` rejected every workspace payload because it lost the active profile.
- **Bugs filed/updated:** BUG-20260827-native-agent-create-profile
- **Scenarios settled:** RT-029 → fixed/pass
- **Paper cuts:** none; the native surface was unusable for workspace authoring.
- **Surprises:** the tool's published input schema was correct; the failure happened later during profile-aware workspace resolution.
- **Suggested next charter:** Continue the same charter with session inheritance and prompt override precedence.

### CH-agent-runtime-default-options — Ada (first prompt inheritance)

- **Ran:** 2026-08-27T22:12:00-03:00 → 2026-08-27T22:31:00-03:00 (box respected: yes)
- **Findings:** the first prompt initially lost the default-profile agent during command discovery; after that fix, post-launch verification treated Cursor's shared ACP configuration selection as an authoritative echo of the process launch argument.
- **Bugs filed/updated:** BUG-20260827-default-profile-agent-prompt-resolution; BUG-20260827-cursor-launch-model-negotiation
- **Scenarios settled:** RT-070 → fixed/pass; RT-cursor-logical-runtime-options Grok canary fixed, full matrix pending
- **Paper cuts:** none; both failures blocked every prompt from the selected runtime.
- **Retest:** fresh Cursor sessions reached the provider with logical runtime state intact. The final Grok 4.5 High/Fast canary answered `QA_RUNTIME_FAST_OK` after the launch-verification and Web inheritance fixes.
- **Suggested next charter:** Continue CH-provider-runtime-strategies with the full transition matrix.

### CH-acp-runtime-catalog-refresh — Ada (automatic release, cold open, Hermes readiness)

- **Ran:** 2026-08-27T22:32:00-03:00 → 2026-08-27T22:44:00-03:00 (box respected: yes)
- **Findings:** no product defect; the controlled future-model, same-source failure, restart, and Hermes handshake paths matched the catalog contract.
- **Bugs filed/updated:** none.
- **Scenarios settled:** MS-live-model-release-refresh → pass; RT-model-catalog-cold-open → pass; RT-hermes-live-model-readiness → pass.
- **Paper cuts:** none.
- **Surprises:** Hermes rows are deliberately hidden from the default list while stale and become visible with `--include-stale`; status still reports the retained row count and failure.
- **Suggested next charter:** Finish catalog Web parity, then continue CH-provider-runtime-strategies.

### CH-acp-runtime-catalog-refresh — Ada (cross-surface parity)

- **Ran:** 2026-08-27T22:45:00-03:00 → 2026-08-27T22:50:00-03:00 (box respected: yes)
- **Findings:** no product defect; CLI, HTTP, native tool, and Web agreed on the new logical catalog contract.
- **Bugs filed/updated:** none.
- **Scenarios settled:** MS-cursor-account-model-discovery → pass; MS-042 → pass.
- **Paper cuts:** none.
- **Surprises:** Cursor currently advertises 33 logical models, including Opus 5 and both requested Grok generations; Web search made the formerly missing rows directly visible.
- **Suggested next charter:** Continue CH-provider-runtime-strategies.

### CH-session-launch-composer-handoff — Dora

- **Ran:** 2026-08-27T22:51:00-03:00 → 2026-08-27T23:29:00-03:00 (box respected: yes)
- **Findings:** Start session had regained a First message field. After removing it, the replay exposed a second defect: an unbound composer inherited provider, model, and reasoning from the agent but replaced Fast with Normal on the first prompt.
- **Bugs filed/updated:** BUG-20260827-session-create-first-message-regression; BUG-20260827-unbound-session-fast-inheritance; BUG-20260827-cursor-launch-model-negotiation
- **Scenarios settled:** MS-web-session-simple-advanced-launch → pass; ET-web-session-prompt-runtime-and-create-navigation → pass; RT-063 → pass. RT-new-session-fast-feedback remains untested because this walk did not measure its 100 ms and 250 ms budgets.
- **Paper cuts:** none; both findings changed runtime behavior.
- **Retest:** Simple and Advanced were prompt-free. A new composer showed Cursor Grok 4.5 High/Fast before bind, completed `QA_RUNTIME_FAST_OK`, and CLI readback retained `speed: fast` in ready state.
- **Suggested next charter:** Measure the explicit fast-feedback budgets after the runtime fixes.

### CH-provider-runtime-strategies — Théo

- **Ran:** 2026-08-27T23:30:00-03:00 → 2026-08-28T00:02:00-03:00 (box respected: yes)
- **Findings:** durable metadata survived restart, but reconciliation and list projection omitted the selected runtime, revision, generation, and recovery fields.
- **Bugs filed/updated:** BUG-20260828-session-runtime-restart-projection
- **Scenarios settled:** RT-cursor-logical-runtime-options → fixed/pass; RT-session-prompt-runtime-transitions → fixed/pass; RT-session-runtime-selection-continuity → fixed/pass; ET-web-runtime-selector-minimal-slider → pass; RT-openclaw-provider-managed-runtime → blocked-verify.
- **Paper cuts:** none; the restart gap changed public runtime truth.
- **Surprises:** OpenClaw strategy and disabled-control projections pass every public and focused check, but the provider executable is not installed in the isolated provider home.
- **Retest:** after a real daemon restart, stopped session `sess-106eb8fa23dfb9c0` retained Grok 4.6, xhigh, Fast, revision 2, and generation 1.
- **Suggested next charter:** Run the OpenClaw handshake on a machine with the CLI installed.

### CH-session-launch-composer-handoff — Dora (fast feedback)

- **Ran:** 2026-08-28T00:03:00-03:00 → 2026-08-28T00:35:00-03:00 (box respected: yes)
- **Findings:** logical acceptance refreshed Cursor's live catalog before any ACP process existed, and the Web waited for a later render before routing.
- **Bugs filed/updated:** BUG-20260828-unbound-session-start-latency
- **Scenarios settled:** RT-new-session-fast-feedback → fixed/pass
- **Paper cuts:** none; the 1.4-second acceptance violated the explicit launch budget.
- **Retest:** direct create fell to 148 ms. A click-listener browser replay measured 14.8 ms to feedback, 207.4 ms to navigation, and 392 ms to the separate composer, with exactly one successful create request.
- **Suggested next charter:** none; the launch charter is terminal.

## What Was Fixed

- Catalog-backed settings validation accepts launch-bound Cursor reasoning choices such as Grok 4.6 `xhigh`; CH-030 passed in a fresh browser retest.
- Native workspace agent creation now forwards the caller's active profile into workspace resolution; focused `-race` tests and a public `compozy__agent_create` retest pass.
- Default-profile command discovery now resolves workspace agents through the profile layer before the first prompt.
- Launch-bound Cursor runtimes are catalog-validated and compiled before process start; shared ACP configuration state no longer vetoes the private launch alias. Focused `-race` coverage and real Grok 4.6 `xhigh` + Fast and Grok 4.5 `high` + Fast prompts pass.
- Explicit model admission uses the complete live catalog, so newly discovered available models do not require curation before use.
- Start session is prompt-free again, and the destination composer inherits Fast from the agent before the first runtime bind.
- A model absent from every seed and source file appeared through live discovery across CLI, HTTP, and native-tool surfaces, survived a same-source failure and daemon restart as stale, then disappeared after the real source recovered.
- Hermes readiness now has real handshake, same-source ACP failure, stale-retention, and recovery evidence for all 11 discovered models.
- Cursor catalog arrays matched exactly across CLI, HTTP, and native tools; Web exposed Grok 4.5, Grok 4.6, and Opus 5 with logical IDs and runtime controls.
- Restart reconciliation and durable session listing now project the complete selected/effective runtime, typed options, revision, generation, and recovery state.
- Unbound logical creation defers live catalog validation to the first runtime bind, and the Web begins navigation directly from durable acceptance instead of waiting for a render-driven handoff.

## Paper Cuts

- `reasoning effort "xhigh" is unavailable for provider model "cursor"/"grok-4.6"` — onboarding Continue after selecting an advertised live combination; BUG-20260827-grok-xhigh-onboarding-rejected.
- `compozy__agent_create` returned `schema_invalid` for a schema-valid workspace payload; BUG-20260827-native-agent-create-profile.
- Default-profile workspace agents failed command discovery before their first prompt; BUG-20260827-default-profile-agent-prompt-resolution.
- Start session duplicated the destination First message composer; BUG-20260827-session-create-first-message-regression.
- Runtime admission rejected available live models outside the curated browsing view; BUG-20260827-live-uncurated-model-admission.
- Cursor launch verification treated shared ACP configuration state as proof of a launch-bound process selection; BUG-20260827-cursor-launch-model-negotiation.
- The unbound Web composer replaced the agent's Fast default with Normal; BUG-20260827-unbound-session-fast-inheritance.
- Restarted sessions lost their selected runtime in public list/read projections; BUG-20260828-session-runtime-restart-projection.
- Unbound creation spent about 1.4 seconds refreshing a catalog before any ACP process existed; BUG-20260828-unbound-session-start-latency.

## Runtime Errors Observed

- `skills: agent not found: "grok_runtime_writer"` during default-profile prompt command discovery; fixed and retested.
- `provider.negotiation.model_unavailable` after Cursor exposed a different shared ACP configuration selection from the launch-bound alias; fixed and retested.

## Human Verifications Needed

- Run one real OpenClaw Gateway bind on a machine whose isolated provider home contains the OpenClaw CLI. All non-executable provider-managed projections and rejection paths passed here.

## Decisions for a Human

None identified yet.

## Learnings

- The targeted cycle reuses the durable ACP charters added with the feature and the existing launch-to-composer canary charter; no duplicate charter was needed.

## Final Status

- **Exit gate (full automated suite):** deferred until every QA row is terminal and any QA finding is remediated, per the agreed sequencing.
- **Issues by user impact:** Blocks-Completion 7 found / 7 fixed · Data-Loss 0 · Trust-Damage 2 found / 2 fixed · Friction 0 · Cosmetic 0
- **Coverage:** 5 / 5 journeys walked; 15 matrix rows pass or fixed/pass and 1 is blocked-verify only because the OpenClaw executable is absent. The overlapping launch/composer scenarios also passed the fresh browser replay.
- **Teardown:** pass — `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/teardown.json` records `"clean": true` with no survivors.
- **Verdict:** QA complete; ready for the single final repository gate.
