# QA Report — provider usage for runtime aliases (#1266), 2026-11-04

Branch `fix/provider-usage-alias-selector` (base v0.5.1), binary built from the integrated branch.
Isolated lab from `eng-qa-bootstrap` (targeted profile, surfaces cli/api/web): own `COMPOZY_HOME`,
port 43967, socket, and a synthetic daemon `HOME` holding a fake "default" Claude account. Synthetic
Claude config dirs for `claude-mm1`/`claude-mm2`; no operator account or real token was used.

The daemon cannot redirect the Claude collector to a fake usage server outside tests, so the lab
ran with `HTTPS_PROXY` pointing at a local stub that logs each `CONNECT` target and answers 502.
That proves whether a refresh leaves the daemon without sending anything upstream; real usage
numbers per account are covered by `TestProviderUsageAliasSelector` (real Claude collector against
`httptest`, IT-001).

## Matrix

| Row | Persona × journey | Surface | Verdict |
| --- | --- | --- | --- |
| QA-1 | Ada × aliases with their own selector | CLI, HTTP | Pass |
| QA-2 | Ada × alias without selector | CLI, HTTP, Web | Pass (after BUG-20261104-alias-selector-log-repeats fix) |
| QA-3 | Ada × Settings list and family logos | Web | Pass |

## Session Debriefs

### QA-1 — aliases with their own selector

- `claude-mm1` (leading `CLAUDE_CONFIG_DIR=…`) and `claude-mm2` (`env -u ARGV0 CLAUDE_CONFIG_DIR=…`)
  were not gated: refreshing `claude-mm1` alone produced exactly one `CONNECT api.anthropic.com:443`
  and the row ended `error` / `network_error` (the stub refused), never the selector message.
- `compozy provider usage list -o json` and `GET /api/provider-usage` returned identical rows at the
  same revision; alias rows carry `collector_id: "claude"`, built-in rows have no `collector_id`.
- Evidence: `artifacts/2026-11-04-provider-usage-runtime-alias/qa1-cli.json`, `qa1-http.json`,
  `qa1-refresh-mm1.txt`.

### QA-2 — alias without selector

- `claude-mm5` (no `CLAUDE_CONFIG_DIR`) refreshed to `signed_out` / `auth_missing` with
  "Set CLAUDE_CONFIG_DIR in this provider's command so CompozyOS reads its own sign-in" over CLI and
  HTTP, `collector_id: "claude"`, and zero `CONNECT`s during its refresh — the synthetic default
  account in the daemon `HOME` was never used. The Usage app row shows the same message.
- Finding: the WARN `provider_usage.alias_selector_missing` repeated on every resolution
  (BUG-20261104-alias-selector-log-repeats, Cosmetic). Fixed in the same branch and re-walked: one
  warning, key only, no path values.
- The empty-value and `env -u` variants were not re-walked live; `TestApplyLaunchEnvironment`
  (UT-002) and the resolver gate table (UT-005) cover them.
- Evidence: `qa2-refresh-mm5.json`, `qa2-http-mm5.json`, `qa2-usage-mm5-expanded.png`.

### QA-3 — Settings and logos

- Settings → Provider usage lists the built-ins, then Claude (mm1), Claude (mm2), claude-mm5 with
  the Claude mark (`qa3-settings-on.png`).
- With **Show provider usage** off, the cadence row hides and the Providers group stays editable;
  saving the mm1 switch wrote only `provider_usage.providers.claude-mm1.enabled = false`
  (`qa3-settings-off.png`).
- The Usage app shows the Claude mark on alias rows (`qa3-usage-rows.png`). A Codex alias whose
  `CODEX_HOME` uses the keyring store lands in the quiet group as `unsupported` with the Codex mark
  (`qa3-usage-quiet.png`). Disabled rows stay hidden from the panel, unchanged from v0.5.1, so the
  quiet-group check uses the unsupported alias; UT-014 was aligned to that reachable state.
- `data-provider` stays the alias ID on every row.

## Limitations

- The message was reworded after this walk from "so Compozy reads" to "so CompozyOS reads" (product
  language check); the JSON captures keep the wording observed at the time.

- No live upstream usage numbers (no reachable fake for the Claude collector in a daemon); covered
  by IT-001.
- Packaged desktop/menubar popover not exercised; it renders the same row components as the Usage
  app walked here.
- During the walk, the single-provider read with collection off returned a synthetic `disabled` row
  without `collector_id` (HTTP/UDS/CLI share that view). Peer review round 1 fixed it: the row now
  names the alias family through `config.ProviderUsageFamily` (covered by `TestProviderUsageAliasSelector`);
  not re-walked live.

## Final Status

Pass for QA-1..QA-3. One Cosmetic finding, fixed and re-walked. Lab torn down; see teardown evidence in
the workflow QA memory.
