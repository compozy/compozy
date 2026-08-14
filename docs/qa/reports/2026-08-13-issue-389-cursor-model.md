# QA Run Report — 2026-08-13 — issue-389-cursor-model

- **Scope:** Exact Cursor ACP model identity for catalog discovery, create, selected runtime, prompt admission, and web settings persistence.
- **Cadence tier:** targeted
- **Build:** `codex/issue-389-cursor-model` working tree · **Environment:** isolated local daemon and Vite web client using the operator's native Cursor login.
- **Started:** 2026-08-13T22:25:25-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Technical operator | desktop / wifi-fast / en-US | CH-cursor-account-models |
| Bruno | Developer | desktop / wifi-fast / en-US | CH-web-exact-model-id, CH-new-session-latency-title |
| Théo | Developer | desktop / wifi-fast / en-US | CH-prompt-bound-runtime-transition |

## Flows in Scope

- `J-20` — discover a signed-in provider's usable models before a session (`../journeys/J-20-catalog-curation-agent-surfaces.md`)
- `J-17` — select and create a session with a truthful runtime (`../journeys/J-17-agent-runtime-selection.md`)
- `J-13` — bind a runtime at the prompt boundary without losing retryability (`../journeys/J-13-session-prompt.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-cursor-account-models | J-20 / MS-cursor-account-model-discovery, MS-042, MS-055 | Ada | Feature Tour | Pass | | pending commit |
| 2 | CH-web-exact-model-id | J-17 / RT-web-exact-model-id-entry, RT-session-runtime-selection-continuity | Bruno | Feature Tour | Pass | | pending commit |
| 3 | CH-prompt-bound-runtime-transition | J-13 / RT-session-prompt-runtime-transitions | Théo | Multi-Tab Tour | Pass | | pending commit |
| 4 | CH-new-session-latency-title | J-17 / RT-new-session-fast-feedback | Bruno | Network Tour | Pass | | pending commit |

## Session Debriefs

### CH-cursor-account-models — Ada

- **Ran:** 2026-08-13T22:25:25-03:00 → 2026-08-13T23:10:00-03:00 (box respected: yes)
- **Findings:** A pre-session refresh used `cursor-agent acp` and returned the exact advertised value `grok-4.5[effort=high,fast=true]`. CLI, HTTP, UDS, and the native tool exposed the same persisted catalog; the rejected `cursor-grok-4.5-high` alias was absent.
- **Scenarios settled:** MS-cursor-account-model-discovery → pass; MS-042 → pass; MS-055 → pass.
- **Evidence:** `cursor-refresh.json`, `cursor-catalog.json`, `cursor-http.json`, and `cursor-native-tool.json` in the isolated lab.

### CH-web-exact-model-id — Bruno

- **Ran:** 2026-08-13T23:10:00-03:00 → 2026-08-13T23:25:00-03:00 (box respected: yes)
- **Findings:** The browser selected the Cursor catalog option and closed the popover without invoking the manual short-ID action. The provider settings PUT, `config.toml`, and public provider-settings readback all retained `grok-4.5[effort=high,fast=true]`; neither the short `grok-4.5` value nor the rejected alias was persisted.
- **Scenarios settled:** RT-web-exact-model-id-entry → pass; RT-session-runtime-selection-continuity → pass.
- **Evidence:** `cursor-web-network.json`, `cursor-web-config.toml`, `cursor-web-provider-settings.json`, and `screenshots/cursor-web-exact-selected.png` in the isolated lab.

### CH-prompt-bound-runtime-transition — Théo

- **Ran:** 2026-08-13T23:25:00-03:00 → 2026-08-13T23:40:00-03:00 (box respected: yes)
- **Findings:** A prompt selecting `cursor-grok-4.5-high` failed before the at-most-once admission commit. Retrying the same identity with `grok-4.5[effort=high,fast=true]` dispatched and retained that exact selected model.
- **Scenarios settled:** RT-session-prompt-runtime-transitions → pass.
- **Evidence:** `cursor-alias-prompt.json` and `cursor-exact-retry.json` in the isolated lab.

### CH-new-session-latency-title — Bruno

- **Ran:** 2026-08-13T23:40:00-03:00 → 2026-08-13T23:50:00-03:00 (box respected: yes)
- **Findings:** Exact Cursor create succeeded. Alias create failed with no durable session, and a blank Cursor model still created a native-default session after the configured default was removed.
- **Scenarios settled:** RT-new-session-fast-feedback → pass.
- **Evidence:** `cursor-live-create.json`, `cursor-alias-create.json`, `cursor-alias-sessions.json`, `cursor-native-default-unset.json`, and `cursor-native-create.json` in the isolated lab.

## What Was Fixed

### #389: Cursor ACP model identity

- **Symptom:** Cursor's human-readable CLI alias could be selected or bound where the ACP process requires its exact advertised model value.
- **Root cause:** Discovery parsed `cursor-agent models` rather than the ACP `model` configuration option, while lifecycle paths did not apply one exact-membership rule before create or prompt admission.
- **Fix:** pending commit — discover the ACP option, preserve its exact value, and reject non-members before durable admission.
- **Regression test:** `internal/acp/config_options_test.go`, `internal/session/manager_test.go`, and `internal/session/manager_busy_input_test.go`.
- **Retested:** all four targeted charter rows above, including a retry with the original message identity.

## Paper Cuts

None observed in the targeted paths. The first browser pass selected the catalog option and then used the manual short-ID action; that invalid procedure was discarded and the clean pass selected the catalog option, closed the popover with Escape, and verified the exact public readback.

## Runtime Errors Observed

None after the corrected browser path. Expected alias-rejection responses are retained as scenario evidence.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Cursor's `cursor-agent models` labels are discovery presentation, not ACP runtime identities; a short ACP inspection is the authoritative source.
- The provider settings public readback nests the persisted value under `.provider.settings.models.default`.

## Final Status

- **Exit gate (full automated suite):** final `make gate-full` runs after the required commit and rebase; its current gate-status record is cited in the completion handoff.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 3 journeys walked / 3 in scope; 4 targeted charter rows passed.
- **Verdict:** PASS / ready — exact ACP identity, alias rejection before admission, retry, native default, and web persistence were observed in one isolated environment; teardown completed cleanly.
