Goal (incl. success criteria):

- Validate the real published Compozy `v0.2.1` artifact from npm/GitHub release against the previous daemon startup/frontend embed regression.
- Success means installing or downloading the real `0.2.1` artifact in isolation, verifying its version metadata, starting the daemon from a clean short HOME, confirming readiness, confirming the daemon-served web UI returns `index.html`, stopping the daemon cleanly, and reporting exact evidence.

Constraints/Assumptions:

- Follow AGENTS/CLAUDE instructions and avoid destructive git commands.
- Do not mutate the user's global npm install or home-scoped daemon state.
- Use short temp HOME paths under `/tmp` to avoid unrelated macOS UDS path-length failures.
- Existing dirty file `.codex/ledger/2026-05-01-MEMORY-release-notes-contract.md` is unrelated and must remain untouched.

Key decisions:

- Prefer isolated npm install of `@compozy/cli@0.2.1` because that exercises the real npm wrapper and the GitHub release archive it downloads.
- Use `COMPOZY_DAEMON_HTTP_PORT=0` so daemon HTTP binds a free local port instead of conflicting with any existing daemon on `127.0.0.1:2323`.

State:

- Complete pending final report.

Done:

- Read prior release-artifact bundle ledger.
- Read `qa-execution` and `cy-final-verify` skills.
- Checked git status and identified only unrelated dirty ledger file before starting.
- Inspected GitHub release page: `v0.2.1` is latest, release title `Release 0.2.1`, published on `2026-05-01`.
- Inspected GitHub release API: `v0.2.1` published at `2026-05-01T22:51:43Z` with `17` assets.
- Inspected npm metadata: `@compozy/cli@0.2.1` points darwin-arm64 to `compozy_0.2.1_darwin_arm64.tar.gz` with SHA-256 `04e6cad9b5fcaf0c815b81ac89049ba35dce604759d6234d72bc6399a8a39af0`.
- Installed `@compozy/cli@0.2.1` into isolated prefix `/tmp/compozy021-prefix.ljPiR7`.
- Verified installed CLI version: `compozy version 0.2.1 (commit=4bdb945 date=2026-05-01T22:46:52Z)`.
- Verified downloaded archive checksum matched published SHA-256.
- Started daemon from clean short HOME `/tmp/compozy021-home.kSCGWb` with `COMPOZY_DAEMON_HTTP_PORT=0`; startup returned `state=ready`, pid `98867`, HTTP port `60941`.
- `daemon status --format json` returned `state=ready` and `health.ready=true`.
- Confirmed `/tmp/compozy021-home.kSCGWb/.compozy/daemon/daemon.json` exists.
- `curl -fsS http://127.0.0.1:60941/` returned daemon-served `index.html` with `<title>Compozy Web UI</title>`.
- `curl -fsSI http://127.0.0.1:60941/assets/index-DUS-maGb.css` returned `HTTP/1.1 200 OK`.
- Searched daemon log for `embedded bundle`, `missing index`, `error`, `panic`, and `fatal`; no matches.
- Stopped daemon successfully; follow-up `daemon status --format json` returned `state=stopped`.
- Wrote QA report at `.codex/release-021-qa/qa/verification-report.md`.
- Local repository gate passed after artifact validation: `make verify` finished with `DONE 3009 tests, 3 skipped`, Playwright `5 passed`, and `All verification checks passed`.

Now:

- Report result to user.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-05-02-MEMORY-release-021-qa.md`
- `.codex/release-021-qa/qa/verification-report.md`
- Temp npm prefix: `/tmp/compozy021-prefix.ljPiR7`
- Temp daemon HOME: `/tmp/compozy021-home.kSCGWb`
