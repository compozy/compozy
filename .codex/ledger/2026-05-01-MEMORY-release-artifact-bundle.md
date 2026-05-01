Goal (incl. success criteria):

- Reproduce the reported `@compozy/cli@0.2.0` daemon startup failure against the real published artifact, then fix the release/package path so published artifacts include the embedded frontend bundle (`web/dist/index.html`) and daemon startup succeeds.
- Success means the real artifact failure is captured with commands/evidence, the root cause in build/release packaging is fixed without workarounds, regression coverage/guards are added, and `make verify` passes.

Constraints/Assumptions:

- Follow AGENTS/CLAUDE instructions, including required skills and `make verify` before claiming completion.
- No destructive git commands (`git restore`, `git checkout`, `git reset`, `git clean`, `git rm`) without explicit permission.
- Preserve unrelated dirty work already present in release workflow/test files unless directly needed for this artifact packaging bug.
- Use isolated temp npm prefix/HOME for artifact reproduction instead of mutating user global state.

Key decisions:

- Reproduce via real npm package first: `@compozy/cli@0.2.0` installed into an isolated npm prefix.

State:

- Complete pending final report.

Done:

- Read required `golang-pro`, `systematic-debugging`, `no-workarounds`, and `testing-anti-patterns` skills.
- Scanned existing ledgers for release/daemon context.
- Observed unrelated dirty files before starting this task.
- Installed real `@compozy/cli@0.2.0` into isolated prefix `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozy-npm-prefix-XXXXXX.fIizmtbyAe`.
- Verified real artifact version: `compozy version 0.2.0 (commit=5871639 date=2026-05-01T19:15:45Z)`.
- Reproduced normal startup failure with short temp home `/tmp/czhome.mASRXX`: missing `/tmp/czhome.mASRXX/.compozy/daemon/daemon.json`.
- Reproduced foreground root cause against real npm/GitHub binary: `daemon: prepare startup: httpapi: load embedded frontend bundle: embedded bundle missing index.html: open index.html: file does not exist`.
- Confirmed npm package downloads GitHub archive `compozy_0.2.0_darwin_arm64.tar.gz`; failure is in shipped binary artifact, not local npm wrapper.
- Confirmed local `web/embed.go` embeds `all:dist` and local `web/dist/index.html` currently exists.
- Checked GoReleaser docs via `ctx7`: top-level `before.hooks` run commands before release builds.
- Added GoReleaser `before.hooks: make frontend-build`.
- Added `setup-bun` to release dry-run and production release jobs before GoReleaser invocation.
- Added release-config regression coverage for the GoReleaser frontend build hook and release workflow Bun setup ordering.
- Focused release config tests passed: `go test ./test -run 'TestGoReleaserBuildsFrontendBundleBeforeBinaries|TestGoReleaserConfigKeepsHomebrewCaskArchivesUnwrapped' -count=1`.
- Full config test package passed: `go test ./test -count=1`.
- Local frontend-aware build passed: `make build`.
- Local fixed binary daemon smoke passed with `COMPOZY_DAEMON_HTTP_PORT=0 HOME=/tmp/czfixed2.1EHUiJ ./bin/compozy daemon start --format json`; daemon ready on HTTP port `62634`.
- Verified embedded UI served from fixed binary: `curl -fsS http://127.0.0.1:62634/` returned `Compozy Web UI` `index.html`.
- Stopped the local smoke daemon successfully.
- Local `goreleaser` / `goreleaser-pro` binaries are not installed, so direct local GoReleaser snapshot validation is unavailable.
- Full verification passed: `make verify` completed with frontend lint/typecheck/tests/build, Go fmt/lint/race tests/build, and Playwright daemon UI E2E (`5 passed`, `All verification checks passed`).

Now:

- Report final evidence.

Next:

- Publish a new release artifact so users can install a fixed version; existing `0.2.0` artifact remains broken.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-05-01-MEMORY-release-artifact-bundle.md`
- `.goreleaser.yml`
- `.github/workflows/release.yml`
- `test/release_config_test.go`
- Real artifact temp prefix: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozy-npm-prefix-XXXXXX.fIizmtbyAe`
- Real artifact temp home: `/tmp/czhome.mASRXX`
