---
id: APP-provider-usage-tray
area: APP
title: See provider limits from the topbar, popover, and Usage app, and grant Claude Keychain consent
persona: Bruno
journey: J-provider-usage
expected: The topbar Provider usage meter shows the tightest known window with an accessible label; the popover and Usage app show one truthful row per provider with expandable detail; Refresh and Allow run explicit refreshes while popover open and the scheduler stay background; Claude Keychain consent on macOS prompts only after Allow or a local CLI refresh, and denial, token expiry, and daemon restart each show their recovery; settings changes apply live; CLI, HTTP, UDS, and native tools return the same profile-scoped envelope.
entry_points: topbar Provider usage control; Provider usage popover; Usage app (/usage); Settings → Provider usage; compozy provider usage list|refresh; GET/POST /api/provider-usage*; compozy__provider_usage_list; compozy__provider_usage_refresh
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-provider-usage-surfaces
---

Added 2026-10-21 for the Provider Usage spec (`.compozy/tasks/provider-usage/`). Walk it in the
desktop app on macOS with real Claude Code and Codex logins, then repeat the popover branches in a
plain browser. `ET-provider-usage-surfaces` owns the transport details (two-profile credentials,
SSE reconnect, app-token middleware); this scenario owns the operator journey. Untested until walked.

Never run an explicit Claude refresh against a lab whose `USER` differs from the Mac's login user:
the Keychain lookup uses the local username and would prompt for the operator's real item.

Codex end to end (file credential store, `codex login` done):

- the meter sits between **All desktops** and the bell; its tooltip and accessible name read like
  `Codex · Weekly 72% used · resets in 2d 4h`; tones are neutral below 80, warning 80–99, danger at
  ≥ 100 with "limit reached";
- the popover lists providers in catalog order, with `unsupported`/`disabled` rows folded into one
  closed trailing group (`N providers without usage data`) that opens to each name and reason, in
  the popover and the Usage app alike; Codex shows plan, tightest window,
  percent, and countdown; expanding shows Session, Weekly, credits, account, source
  `Codex app-server`, and measured time; Esc closes and focus returns to the meter;
- opening the popover refreshes only rows older than 60 s (background); ⟳ refreshes explicitly,
  keeps numbers visible while collecting, and a second ⟳ within 30 s does not re-collect;
- **Open Usage** opens the Usage window with every row expanded and its own ⟳; it updates live
  while the popover refreshes;
- set `cli_auth_credentials_store = "keyring"` in `$CODEX_HOME/config.toml`: the Codex row turns
  `unsupported` with the keyring message and no keyring prompt appears; restore `"file"`.

Claude on macOS (no `.credentials.json`, login in the Keychain):

- with consent off the row reads `Needs Keychain access` with **Allow**, and the CLI table ends
  with the `keychain_access true` hint; no macOS prompt appears from opening the popover or waiting
  through a background interval;
- **Allow** saves the Settings switch; the daemon runs exactly one explicit collection and the web only invalidates its query (no follow-up refresh POST); macOS asks for
  `Claude Code-credentials`; **Allow** fills Session, Weekly, per-model weekly, and Extra usage;
- **Deny** shows `Keychain access denied` with **Try again**; nothing retries until clicked;
- with Claude Code signed out of the Keychain (no `Claude Code-credentials` item), **Allow** ends in
  `Signed out · Sign in with Claude to read usage`, not a denial;
- while the loaded token is valid, background refreshes keep the row fresh without a prompt; once
  it expires the row reads `Refresh to reload your Claude login` and ⟳ reloads it (silent after
  **Always Allow**);
- `compozy provider usage refresh claude` from the host terminal behaves like **Allow** once
  consent is on and prints only the Claude row; a gateway device sees `Approve on the host Mac: open CompozyOS there and refresh`
  and never triggers a prompt;
- restart the daemon: rows read `Loading…` until their first collection; Claude asks for **Refresh**
  with consent still on;
- sign in to Claude Code with another account or sign out: the old numbers disappear instead of
  turning stale;
- turn **Claude Keychain access** off: the loaded login and snapshot are dropped immediately.

Key-based providers and settings:

- with an OpenRouter or Vercel AI Gateway key bound to the profile, the row shows balance and cap
  amounts in native units; a z.ai key without a coding plan reads `No coding plan on this key`;
  a provider without a key reads `Signed out`;
- Settings → Provider usage: the interval select offers 1m–1h and applies live; a provider switch
  off removes its row from the tray, popover, Usage app, and unfiltered CLI list (`list <id>` shows
  `disabled`); **Show provider usage** off removes the meter, and `compozy provider usage list`
  prints the turned-off message while `refresh` exits 1 with `provider_usage_disabled`;
- on a Linux or Windows host (`keychain_supported: false` in the envelope and settings response)
  the Settings Keychain row and every **Allow** button are absent.

Parity and profile isolation:

- the CLI JSON, `GET /api/provider-usage` over HTTP and UDS, and `compozy__provider_usage_list`
  from an agent session return the same revision and rows for the same profile;
- `compozy__provider_usage_refresh` from an agent never prompts, even with consent on, and
  `compozy__config_set provider_usage.providers.claude.keychain_access true` is refused while
  `provider_usage.providers.<id>.enabled` is accepted;
- switch the shell to a second profile with different logins: the meter, popover, and
  `--profile <name>` CLI read only that profile's numbers; no surface combines profiles.
