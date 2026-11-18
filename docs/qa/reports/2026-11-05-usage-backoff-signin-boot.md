# QA Report — usage backoff, one collection per account, sign-in after restart (#1302)

Date: 2026-11-05. Branch: `fix/usage-backoff-signin-boot`. Workflow:
`.compozy/tasks/usage-backoff-signin-boot` (task_07). Verdict: **PASS** for QA-1, QA-2, and QA-3.

## Lab

- Isolated `eng-qa-bootstrap` lab (targeted profile, surfaces cli/api/web). Lab daemon is the
  worktree build (`go build ./cmd/compozy`) with its own `COMPOZY_HOME`, port 56847, socket, and
  tmux socket. All `COMPOZY_*` variables from the managed session were removed by a wrapper.
- `PATH` was limited to the lab's fake CLIs and system directories, and `HOME` pointed into the lab.
  The startup probe therefore never ran the operator's real `claude` or `codex` CLIs or touched real
  accounts.
- Providers: `acmein` and `acmeout` are `native_cli` providers with fake `auth_status_command`
  scripts (signed in and signed out). `claude-mm9` is an alias with `runtime_provider = "claude"`.
  The built-in `codex` provider points `CODEX_HOME` at a synthetic auth file whose
  `chatgpt_base_url` is a loopback fixture. The fixture answers `429` + `Retry-After: 0` (or 200)
  and logs each request.
- Two profiles (`default`, `open-design`) resolve the same Codex account.
- Teardown: `teardown.json` reports `clean: true`.

## QA-1 — 429 floor, backoff, one collection per account (ET-provider-usage-surfaces)

| Step | Observation | Result |
| --- | --- | --- |
| Startup tick, fixture `429` | One fixture request for both profiles; one `provider_usage.cooldown` entry with `strikes: 1` and `until` 5 minutes later | PASS |
| CLI `provider usage list codex` and HTTP for both profiles | Both show `rate_limited` with the same `next_refresh_at` and `error.retry_after` (+5 min) | PASS |
| Explicit `provider usage refresh codex` during the cooldown | One request; `strikes: 2`; `next_refresh_at` +10 min | PASS |
| Second explicit refresh, and background refresh from `open-design` | No request; both show the +10 min cooldown | PASS |
| Restart with the fixture answering 200 | One request served both profiles (windows 40/72 in each) | PASS |

The 20/40/60-minute steps and the reset after success are owned by `TestUsageRateLimitBackoff`, so
they were not re-walked in real time.

## QA-2 — sign-in after restart, CLI agrees with Settings (MS-web-provider-auth-gate)

| Step | Observation | Result |
| --- | --- | --- |
| Daemon start, no manual probe | `provider_auth.startup_probe_done probed=2 skipped=16`; `provider list` shows `acmein` authenticated and `acmeout` needs_login, both with `last_probe_at` | PASS |
| `provider auth status acmein` / `acmeout` with the daemon up | `probe_source: daemon`, with the daemon's state and exit code | PASS |
| `--local` | `probe_source: local` | PASS |
| Daemon stopped, default invocation | `probe_source: local`, authenticated | PASS |
| `--local --remote` | Exit 1: `cli: --local and --remote cannot be used together` | PASS |
| `--remote` | Unchanged daemon payload (`provider`, `auth_status`, `probe`) | PASS |
| Second restart | Startup probe ran again (`probed=2`) | PASS |

## QA-3 — Settings → Providers (web)

Screenshot `qa3-settings-providers.png` (1440×900, `agent-browser`, daemon serving the worktree's
`web/dist`). It was taken after a restart with no manual check: Acme In shows **Ready** with a green
dot and a muted label, and Acme Out shows **Needs sign-in** with an amber dot. Claude MM9
(`runtime_provider = "claude"`) shows the Claude logo, and Pi keeps its own logo. Verdict: PASS.

## Observations (not regressions of this change)

- `compozy provider usage list --profile open-design` returned the default profile's envelope. The
  HTTP `?profile=` selector returned the right profile, so QA used HTTP for that read. This branch does
  not change that CLI path. It is recorded here only as a follow-up candidate.
- Local gate: `TestExtensionDevBindsResolvedCurrentWorkspace` fails only on this host. An empty
  `/tmp/.git` directory makes `go build` in a temporary directory try VCS stamping (exit 128). This
  is unrelated to the change.

## Evidence

`docs/qa/evidence/2026-11-05-issue-1302-usage-backoff/`: fixture request log, daemon log excerpt,
provider list, auth-status outputs (daemon, local, daemon stopped), explicit refresh envelope,
open-design envelope, and the Settings screenshot.
