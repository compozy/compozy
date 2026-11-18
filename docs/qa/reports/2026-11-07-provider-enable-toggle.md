# QA report — provider enable toggle (issue #1326)

Date: 2026-11-07. Spec `provider-enable-toggle`, tasks task_06 (plan) and task_07 (execution).
Branch `feat/disable-providers`.

## Lab

Isolated targeted lab (eng-qa-bootstrap, surfaces cli, api, web, runtime). Its home, port 51687,
socket and tmux socket were its own, and `COMPOZY_*` was unset from the managed session. Daemon and
`acpmock-driver` were built from the branch. The config added providers `acpmock` (the default),
`acpmock2` and `mock-alias` (`runtime_provider = "acpmock2"`), plus agents `alpha` (on `acpmock2`)
and `beta` (on `acpmock`). Web ran under Vite with the lab proxy target and was driven by
agent-browser.

- Strict audit (`audit-qa-evidence.py --strict`): exit 0.
- Teardown: `teardown.json` reports `clean: true` with no survivors.
- Lab evidence: `~/dev/qa-labs/compozy-provider-enable-toggle-20261107-213441-969645-lab/qa-artifacts/evidence/`.

## Results

| Check | Result |
| --- | --- |
| `config set providers.acpmock2.enabled false` | Applied live: lifecycle `live`, no restart |
| `provider list` (human and JSON) | Shows `acpmock2` with `enabled: false`; the alias stays enabled (BR-7) |
| Session already running on `acpmock2` | Keeps answering prompts after the disable (BR-5) |
| `session new` on `acpmock2` | `provider_disabled` with the re-enable hint; no session row created |
| `session continue --provider acpmock2` | Refused |
| `session continue --provider mock-alias` | Succeeds |
| Workspace detail `providers[]` | Omits `acpmock2`, keeps the alias |
| `PATCH .../enabled` | 422 `provider_default_disable_rejected` on the default provider; 400 when `enabled` is missing; 404 for an unknown provider |
| Disabling the default provider via CLI | Rejected with the BR-3 message; file unchanged |
| `--scope workspace` | Rejected: `providers.<id>.enabled can only be set in the global or profile config` |
| Profile layer | `eng` profile disable refuses starts in `eng`; the `default` profile starts normally |
| Live-apply split | `enabled` applies live while the pending `env_policy` edit stays pending (`restart_required`, `restart-daemon`) |
| Pending default | CLI keeps the change pending and returns a `warnings` entry; PATCH returns 422 with the pending-default message |
| Re-enable via CLI | Config file is byte-identical to the original (after BUG-1 fix) |
| `roles show auto_title` | No diagnostic while a fallback stays enabled; `provider_disabled` once every route is off |
| Web: Settings → Providers | Switch disables ACP Mock Two: toast, header "· 1 disabled", card moves to "Disabled (N)" |
| Web: re-enable | Re-enabling from the Disabled group returns it to the grid |
| Web: default provider | Switch locked on the card and in the detail dialog |
| Web: runtime selector | Omits disabled providers (ACP Mock Two, Codex) and keeps the alias; ACP Mock Two returns after re-enable |
| Web: Settings → Provider usage | Shows "Provider disabled" on the Codex row |

## Bugs found and fixed

- **BUG-1 (Blocks-Completion, BR-11).** `compozy config set providers.<id>.enabled true` and
  `compozy__config_set` wrote `enabled = true` instead of removing the key, so re-enabling was not
  lossless. Fix: `OverlayEditor.SetValueAtTarget` deletes the key, and an emptied block, for global
  and profile targets. Workspace targets keep the plain write so validation still rejects them.
  - Tests: `TestSetValueAtTargetProviderReEnable`.
  - Re-walk: config file byte-identical. Status: fixed, retest pass.
- **BUG-2 (Copy).** `provider_disabled` and `provider_default_disable_rejected` diagnostics were
  titled "Role operation failed" by the generic coded-error fallback. Fix: titles "Provider is
  disabled" and "Default provider can't be disabled".
  - Test: `TestErrorPayloadForError` subtest.
  - Re-walk: title correct. Status: fixed, retest pass.

## Not walked live (covered by tests)

- Usage tray row removal: provider usage collection is disabled in labs by contract. Covered by
  `TestProviderUsageResolverDisabledProviders` and the web usage tests.
- Native tools (`compozy__provider_models_list`, `compozy__subagent_capabilities`): covered by the
  task_02 daemon and session suites.
- Auto-title role invocation on the fallback route: covered by `TestRoleDisabledProviderRoutes`;
  role status was walked live.

## Known ceilings

- Resolved after QA: provider inventories (`GET /api/providers[/{id}]`, `compozy provider list
  [--profile]`) now report `enabled` for the selected profile (active global snapshot plus that
  profile's override), owned by `TestProviderInventoryEnabled` and the `provider list` profile case.
