---
id: RT-provider-enable-toggle
area: RT
title: Turn off a provider the operator doesn't use
persona: Ada
journey: J-configure-providers
expected: Setting providers.<id>.enabled = false (CLI, native tool, Settings PATCH or a global/profile file edit) applies live; the provider stays listed in inventories with enabled:false, leaves every selection surface, and new starts, stopped-session resumes, runtime changes and loop/goal runs on it fail with provider_disabled (409) while running sessions and same-route recovery keep working. The default provider can't be turned off (provider_default_disable_rejected, 422), workspace layers reject the key, aliases toggle independently, and turning it back on restores the config file byte for byte.
entry_points: compozy config set providers.<id>.enabled false|true [-o json]; compozy provider list [-o json]; compozy session continue <id> --provider <id>; compozy config set providers.<id>.enabled false --workspace <ref>; PATCH /api/settings/providers/{name}/enabled; GET /api/providers; GET /api/workspaces/<id> (providers[]); compozy__config_set; compozy__provider_models_list; compozy__subagent_capabilities
qa_status: pass
bug_ids: BUG-20261107-provider-reenable-not-lossless; BUG-20261107-provider-disabled-diagnostic-title
fix_status: fixed
retest_status: pass
fix_commits:
evidence: lab qa-artifacts/evidence (see report)
last_report: docs/qa/reports/2026-11-07-provider-enable-toggle.md
overlaps: MS-web-settings-providers-redesign; MS-provider-detail-modal; ET-provider-usage-surfaces; APP-provider-usage-tray; MS-background-role-fallback; ET-web-session-prompt-runtime-and-create-navigation
---

Planning 2026-11-07 (issue #1326, spec `provider-enable-toggle`, task_06). Walk in an isolated lab
(eng-qa-bootstrap: own home, port and socket, COMPOZY_* unset) with a mock or real second provider.

1. **Config and CLI journey (E2E-001).** `compozy config set providers.gemini.enabled false -o json`
   reports `applied: true`, lifecycle `live`, no restart. `compozy provider list -o json` still lists
   gemini with `enabled: false`; the human table shows the `Enabled` column.
2. **Refused start.** `compozy session continue <id> --provider gemini` (and a new session pinned to
   gemini) fails with `provider_disabled` and the message naming
   `compozy config set providers.gemini.enabled true`. No session row, no process.
3. **Running work.** A session already running on gemini before the disable keeps taking prompts
   (same-route recovery is not gated); switching it to another disabled route is refused.
4. **Selection surfaces.** Workspace detail `providers[]`, `compozy__provider_models_list` and
   `compozy__subagent_capabilities` omit gemini; an explicit `provider_id=gemini` model list returns
   no models with a `provider_disabled` note.
5. **Aliases.** Disable an alias (`claude-mm2`) → its base `claude` and sibling aliases stay
   selectable.
6. **Default provider.** Disabling `defaults.provider` is refused: CLI/native write error and
   `PATCH .../enabled` 422 `provider_default_disable_rejected`, file unchanged.
7. **Layers.** `--workspace` scope (and a workspace `config.toml` that sets the key) is rejected with
   `providers.<id>.enabled can only be set in the global or profile config`; a profile overlay that
   disables gemini wins over global for that profile.
8. **Live apply split.** Stage a restart-required edit (`providers.gemini.command`) in the file, then
   disable gemini through the CLI: gemini is disabled live, the command edit stays pending
   (`restart_required: true`, restart-daemon next action). With a pending `defaults.provider` change
   that still names gemini active, the disable stays pending with a `warnings` entry in
   `config set -o json`.
9. **Re-enable.** `compozy config set providers.gemini.enabled true` → starts succeed again and the
   config file is byte-identical to the original (an `enabled`-only block is removed);
   `provider_usage.providers.gemini.*` never changes. A block authored as `[providers.OPENCODE]`
   is toggled in place for `opencode` (no sibling `[providers.opencode]`) through both the CLI and
   the Settings toggle (`PATCH /api/settings/providers/opencode/enabled`).
10. **Pending-only reload.** Add a new custom provider with `enabled = false` and reload: the
    reload stays pending (`applied: false`, restart-daemon) instead of recording a live apply.
    Disabling a custom provider whose definition is still pending succeeds and stays pending with
    it; removing a disabled custom provider from the file keeps it disabled until the removal
    applies on restart.
11. **Retained disabled value.** A session picker pinned to a disabled provider shows it marked
    Disabled, but neither its model row (pointer or keyboard) nor a custom model ID for that
    provider can be committed. An exact `provider/model` entry (loop runtime, `allowCustomProvider`)
    naming any provider the selector knows is disabled is refused, even when it is not the current
    value.

QA 2026-11-07 (issue #1326): walked in an isolated lab — see `docs/qa/reports/2026-11-07-provider-enable-toggle.md`.
