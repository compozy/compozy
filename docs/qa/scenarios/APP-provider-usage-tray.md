---
id: APP-provider-usage-tray
area: APP
title: See provider limits from the topbar, popover, and Usage app, and grant Claude Keychain consent
persona: Bruno
journey: J-provider-usage
expected: The topbar Provider usage meter shows the tightest known window with an accessible label; the popover and Usage app show one truthful row per provider, collapsed to a single line with a compact cue (percent used, or the status in a word or two) and detail only on expand, in an overlay scroll area whose bar appears only on hover or scroll; Refresh and Allow run explicit refreshes while popover open and the scheduler stay background; Claude Keychain consent on macOS prompts only after Allow or a local CLI refresh, and denial, token expiry, and daemon restart each show their recovery; settings changes apply live; CLI, HTTP, UDS, and native tools return the same profile-scoped envelope.
entry_points: topbar Provider usage control; Provider usage popover; Usage app (/usage); Settings → Provider usage; compozy provider usage list|refresh; GET/POST /api/provider-usage*; compozy__provider_usage_list; compozy__provider_usage_refresh
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: reports/2026-10-24-issue-955.md
overlaps: ET-provider-usage-surfaces
---

Added 2026-10-21 for the Provider Usage spec (`.compozy/tasks/provider-usage/`). Walk it in the
desktop app on macOS with real Claude Code and Codex logins, then repeat the popover branches in a
plain browser. `ET-provider-usage-surfaces` owns the transport details (two-profile credentials,
SSE reconnect, app-token middleware); this scenario owns the operator journey. Untested until walked.

Never run an explicit Claude refresh against a lab whose `USER` differs from the Mac's login user:
the Keychain lookup uses the local username and would prompt for the operator's real item.

Codex end to end (file credential store, `codex login` done):

- the meter leads the right-aligned tray, before the desktop pager and **All desktops**, so its arrival consumes spacer width without moving the existing controls; its tooltip and accessible name read like
  `Codex · Weekly 72% used · resets in 2d 4h`; tones are neutral below 80, warning 80–99, danger at
  ≥ 100 with "limit reached";
- the popover lists providers in catalog order, with `unsupported`/`disabled` rows folded into one
  closed trailing group (`N providers without usage data`) that opens to each name and reason, in
  the popover and the Usage app alike; every row starts collapsed to one line, and Codex reads
  its plan and percent used (the accessible name adds the tightest window and countdown);
  expanding shows Session, Weekly, credits, account, source `Codex app-server`, and measured
  time; with more rows than fit, the scroll bar stays hidden until hover or scroll and never
  covers a row; Esc closes and focus returns to the meter;
- opening the popover refreshes only rows older than 60 s (background); ⟳ refreshes explicitly,
  keeps numbers visible while collecting, and a second ⟳ within 30 s does not re-collect;
- **Open Usage** opens the Usage window with every row collapsed and its own ⟳; it updates live
  while the popover refreshes;
- set `cli_auth_credentials_store = "keyring"` in `$CODEX_HOME/config.toml`: the Codex row turns
  `unsupported` with the keyring message and no keyring prompt appears; restore `"file"`.

Claude on macOS (no `.credentials.json`, login in the Keychain):

- with consent off the collapsed row reads `Needs access`; expanding it shows
  `Needs Keychain access` with **Allow**, and the CLI table ends
  with the `keychain_access true` hint; no macOS prompt appears from opening the popover or waiting
  through a background interval;
- **Allow** saves the Settings switch; the daemon runs exactly one explicit collection and the web only invalidates its query (no follow-up refresh POST); macOS asks for
  `Claude Code-credentials`; **Allow** fills Session, Weekly, per-model weekly, and Extra usage;
- **Deny** shows `Keychain access denied` with **Try again**; background retries are silent and never
  prompt, including when OS access is later granted outside CompozyOS;
- with Claude Code signed out of the Keychain (no `Claude Code-credentials` item), **Allow** ends in
  `Signed out · Sign in with claude /login to read usage`, not a denial;
- while the loaded token is valid, background refreshes keep the row fresh without a prompt; once
  it expires the daemon silently rereads the exact routed item. If Claude Code has renewed it, usage
  recovers automatically; an actually expired credential directs the operator to `claude /login`;
- `compozy provider usage refresh claude` from the host terminal behaves like **Allow** once
  consent is on and prints only the Claude row; a gateway device sees `Approve on the host Mac: open CompozyOS there and refresh`
  and never triggers a prompt;
- restart only the isolated lab daemon: rows read `Loading…` until their first collection; Claude
  recovers automatically when consent is still on and OS access is available;
- sign in to Claude Code with another account or sign out: the old numbers disappear instead of
  turning stale;
- within 30 seconds of a successful collection, switch the native Claude login to another account
  and let a background refresh reread the replacement login. Click the row's **Refresh** immediately:
  the new account's usage and plan appear without waiting for the previous collection's freshness
  window. The next background interval refreshes that account without another Keychain read;
- repeat with a provider route selecting another `CLAUDE_CONFIG_DIR`,
  `CLAUDE_SECURESTORAGE_CONFIG_DIR`, or `HOME`. Usage follows the provider route, and other profiles
  retain their own login. Missing or unreadable account evidence still forbids background token reuse;
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

PR review regressions:
- Open the popover with a first-fetch Loading row and no active collection. Opening requests a
  background refresh; a row with an active collection is not requested again.
- Fail an explicit Refresh or Try again request, then let the envelope read succeed. One error toast
  explains the rejected action while cached numbers remain visible. Background failures do not toast.
- Fail Allow's settings save. One Keychain error toast appears, and no explicit refresh is sent.

2026-10-22 qa-impact (#900 shell UI polish): every provider row, signed-out ones included, now
starts collapsed to one line with a compact trailing cue (percent used, or a word such as
`Signed out`, backed by its status dot); the reason line and its action (**Allow**, **Retry**,
**Try again**, **Refresh**) and the per-window detail appear only after expanding the row. The
Usage app no longer opens every row expanded. The shared `ScrollArea` bar now overlays a reserved
gutter and shows only on hover, scroll, or keyboard focus. Re-walk the expand-then-act steps;
status stays `untested`.

### Automatic recovery and explicit feedback

- In two operator profiles with consent, open the tray and Usage app after a fresh lab startup.
  Each profile independently acquires its routed credentials without a prompt or a previous explicit
  refresh. Expire the cached credential and let Claude Code renew its native item; the next scheduled
  collection must recover. Never alter the operator's credential to manufacture expiry.
- Switch profiles and refresh a single row. Verify the POST names the displayed profile and provider,
  the busy indicator appears, and an unsuccessful collection ends with a precise visible result.
- From a gateway client, a cached-login refresh warning explains that OS authorization must happen
  on the host Mac. Refresh never opens an OS prompt remotely.

## Targeted verification — 2026-10-24

The issue-955 isolated macOS walk verified startup collection for two profiles, missing-login
guidance, and browser Refresh targeting with real OAuth usage and independent API readback.
The profile mismatch was reproduced through the real HTTP middleware and repaired, then re-walked.
See [the run report](../reports/2026-10-24-issue-955.md) for evidence and limits. The broader scenario
remains untested because packaged desktop, all transport/security variants, and forced native-token
expiry were not part of this targeted walk.
