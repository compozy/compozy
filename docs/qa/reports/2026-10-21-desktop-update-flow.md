# Desktop update implementation verification — 2026-10-21

Scope: branch `fix/desktop-update-flow`, baseline `b1008af26`, isolated worktree
`desktop-update-flow`. Targeted engineering verification of the update repair;
this is not a completed published-release or cross-platform persona cycle.

## Journey and coverage

The changed flow is mapped in [J-desktop-update-moment](../journeys/J-desktop-update-moment.md)
and [J-desktop-agent-headless](../journeys/J-desktop-agent-headless.md). Existing
`CH-desktop-update-rehearsal-macos`, `CH-electron-beta-update-macos`, and their
Linux/browser counterparts retain ownership of the full release journeys. Their
broader missions were not run or relabeled as passing here.

The six affected scenarios are flagged `untested` for their complete current
contracts: APP-app-auto-update, APP-runtime-update-app-owned,
APP-single-command-multi-target-update, APP-update-recovery-state,
APP-web-update-indicator and APP-web-update-two-track. The targeted evidence below
supports the changed branches, without inheriting an old full-scenario verdict.

## Native macOS rehearsal

Environment: macOS 26.6.2 / Darwin 25.6.0, arm64, Electron 44.5.1,
electron-updater 6.8.9. The existing Playwright desktop fixtures create separate
application copies, homes, update caches, loopback feeds and daemon ports. The
installed CompozyOS/Grok Bot applications and the operator's Compozy home are not
update targets. Fixture signatures are local ad-hoc signatures; notarization is
deliberately disabled in the existing E2E packaging path.

The fixture now builds baseline app/runtime **0.3.0** and candidate app/runtime
**0.3.1**, instead of packaging the same runtime in both apps. E2E-016 and E2E-017
require all of the following after actual native replacement and relaunch:

- The app record reports 0.3.1.
- The update history ends in `updated`.
- The shell reaches `product` with an owned 0.3.1 runtime and no startup error.
- `compozy status -o json` reports a running 0.3.1 daemon.

E2E-016 also observes the real Settings API and download progress in the product
window. E2E-017 first archives an expired handoff as failed, preserves the old app
version, and renders **Retry update** in the real product before the fresh attempt.
The existing staged-operation fixture supplies the verified plan; it does not
click through a production GitHub/Sigstore release authority.

Command, invoked from the repository root with pinned Bun 1.4.0 and the existing
machine verification lock:

```text
bun run --cwd desktop test:e2e --grep 'E2E-016|E2E-017|E2E-007|A staged app update'
```

Native result: **8 passed (4.4m)**, exit code 0. This includes both actual update
journeys, four adjacent shell shutdown/relaunch checks, consuming a staged update
while runtime bootstrap fails, and refusing an unverified bundled runtime.

The first run passed native replacement and runtime bootstrap but failed a new
test assertion because its author used the nonexistent `daemon status` command.
The assertion was corrected to the existing `status -o json` command and its
`daemon` object; the readiness/version requirement was retained. Both packages
were rebuilt from the final production source before rerunning.

A second run passed the update assertions but exceeded the existing five-second
teardown limit for the relaunched shell in E2E-016. macOS reported a clean exit
about eight seconds after its last window closed; the exact native delay was not
established. The limit was not relaxed. The existing fixture now preserves process
state and desktop logs if that cleanup fails.

Inspection also found that `updaterCacheDirName` isolates only the JavaScript
download cache: Squirrel still keyed its native helper/cache by the production
bundle identifier. Both fixture versions now share a worktree-specific E2E bundle
identifier, with the existing ad-hoc signature requirement derived from that ID.
The final eight-test run passed with this isolation. This is not claimed as proof
that the identifier collision caused the earlier native exit delay.

Teardown verification found no remaining lab shell, helper or daemon processes;
`launchctl getenv COMPOZY_HOME` returned to its prior unset state. The idle native
test helper registration and its isolated cache were removed. The earlier helper's
state explicitly referenced a test-only app path: that state was preserved in the
worktree cache before unregistering the helper and removing its identified test
staging directory. Other production-cache entries were left intact.

Local evidence: `.cache/update-research/desktop-e2e.log`,
`.tmp/playwright/desktop-results/`, `.tmp/playwright/desktop-report/` and
`desktop/.artifacts/e2e-update-fixture.json`. Screenshots capture downloading in
Settings and the Retry update menubar state, copied to
`docs/qa/evidence/2026-10-21-desktop-update-flow/`. Large binaries/logs remain ignored.

## Owning regression suites

| Layer | Observable invariant and evidence |
| --- | --- |
| Desktop consumer | Stalled I/O reaches failure, progress refreshes only the idle deadline, late completion cannot install, transient acquisition retries, terminal failure does not reinstall; existing updater suites: 24 passed after red reproductions |
| Native adapter | Staging succeeds before cleanup; native error/abort preserves the shell; abort reaches the real download cancellation token; Rosetta disables incompatible differential download |
| Main lifecycle | Real Electron BrowserWindow red/green probes reproduce ordinary quit winning before native readiness and verify repaired event order; packaged E2E owns the actual replacement |
| Go planner | Verified app-only plan for desktop-owned joint intent; no redundant runtime archive; original eligibility and order checks preserved; Rosetta app selection agrees with MacUpdater |
| CLI completion | Real local daemon HTTP client and discovery record; old, starting, incompatible stable/beta, unavailable and newer-compatible runtimes produce truthful outcomes |
| Web | Failed eligible app retains its error and exposes direct retry; existing projection, menubar and General Settings suites: 94 tests passed |

Completed scoped commands:

```text
bunx turbo run test typecheck lint --filter=./desktop --only
  PASS: 191 tests / 22 files, typecheck and lint
CGO_ENABLED=1 go test -race ./internal/update
  PASS
CGO_ENABLED=1 go test -race ./internal/cli -run 'TestUpdateCommandContract|TestUpdateTerminalProjection|TestSettingsRestartTimeout|TestLocalRestartStatusDuringBoot' -count=1
  PASS
COMPOZY_GO_LINT_SCOPES='./internal/update ./internal/cli' make go-lint
  PASS: zero issues, default and integration tags
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./internal/update
  PASS
```

Root Turborepo Web typecheck/build passed after the retry projection change.
React Doctor scored 100/100 with no issues. The build still prints existing CSS,
chunk-size and dynamic-import diagnostics; these are not counted as clean lint
evidence. An initial generator check under Bun 1.4.3 failed before tests; rerunning
with repository-pinned Bun 1.4.0 passed. No unrelated generated file was changed.

The full Web suite also emits React `act(...)` notices and expected failure-path
console errors in unchanged suites, plus Node's experimental SQLite warning.
Those messages are retained in the gate log; they are not a warning-free test log
or newly resolved defects. The lint tasks report zero warnings and zero errors.

The root affected gate covers `internal/cli`, `internal/update`, `skills`, desktop
and Web. Its current-tree receipt is `.cache/gate/`; `make gate-status` is the
authority for cache freshness. `make gate` passed all affected lanes: Go lint and
race tests, generated-artifact checks, desktop lint/typecheck plus 191 tests in
22 files, and Web lint/typecheck plus 7,262 tests in 687 files. Final editorial
changes are revalidated before the local commit; no CI or release receipt is implied.

The pre-commit task set also passed via `lint-staged --no-stash
--no-hide-partially-staged --no-revert`, preserving the operator's prohibition on
Git stash/restore operations. The first attempt selected the global Go 1.26
formatter despite the build using Go 1.27.1; selecting that toolchain's `gofmt`
fixed the check and returned its incidental formatting edits to their original
content. Only the intended update/research paths remain in the commit.

## PR #886 review remediation

The first PR head (`250e8a919`) completed 33 successful checks and 18 conditional
skips. Its bot reviews still identified actionable changes; that green check
rollup was not treated as merge approval. Current-head remote delivery remains
tracked in [PR #886](https://github.com/compozy/compozy/pull/886).

- **Windowless process after failed handoff (CodeRabbit):** the existing native
  update suite now replaces the fixture's control-token file with a directory,
  causing a real cleanup failure after native staging. Before the fix, the
  operation archived as failed and the product window closed, but the process
  remained alive past the ten-second assertion. The error handler now releases
  installer quit ownership and joins the existing cleanup before ordinary quit.
  This also covers a deadline error arriving while cleanup is still draining.
  The unchanged regression passed after the production change.
- **Restart observation (Greptile):** the CLI captures live daemon PID/start time
  before acquiring the update operation and compares that identity with the
  validated ready daemon afterward. Two red regressions returned false for a
  replaced PID and a reused PID with a different start time. Both pass after the
  fix; preserved daemons and an initially absent daemon keep a false restart flag.
- **Specific failure assertions (CodeRabbit):** the owning CLI projection table
  requires the snapshot error or the wrapped deadline error for its failure rows,
  while retaining the app/runtime status assertions. The four owning CLI suites
  passed with the race detector (3.825 seconds), and the test-shape checker passed.
  The broader gate then exposed a race when the deadline interrupted an in-flight
  HTTP snapshot request: it returned a raw transport error instead of the update
  deadline message. A deterministic blocked-request regression reproduced that
  failure. The production observer now returns deadline handling to its polling
  owner, preserving the specific error assertion and context cancellation cause.
- **Shared template claims (Greptile):** reusable prompts and empty checkboxes are
  restored. The user-requested complete implementation record remains in the same
  file inside an explicitly labeled, non-rendering historical comment for PR #886.
  The current PR description carries the complete visible report.
- **Unavailable pre-update identity (CodeRabbit follow-up):** reading the previous
  daemon identity is best effort because it only informs `daemon_restarted`.
  A real malformed discovery file reproduced an early abort in the existing CLI
  projection suite. Missing or unreadable identity now leaves the restart flag
  false, while final daemon readiness, version and snapshot validation remain
  mandatory. The original discovery file is not repaired or deleted by capture.
  The red reproduction is retained in `review-baseline-red.log`.

The local desktop check exposed an existing health-monitor test that stopped the
monitor after 30 ms of wall-clock time during concurrent packaging. Its canonical
suite now advances a controlled clock and checks zero notifications after one
failure, one notification after the second, and no duplicate afterward. The
production monitor and its failure threshold are unchanged. All 191 desktop tests,
lint and typecheck passed afterward via root Turborepo.

The post-review native run exercised nine cases with rebuilt baseline/candidate
packages: **7 passed, 2 failed**. Both native installation/recovery journeys and
the new failed-cleanup regression passed. The two failures were existing shell
teardown checks after cleanup had completed, in the healthy relaunch and
unverified-bundle scenarios. The macOS log for PID 6777 records cleanup ending at
22:01:46.918 UTC and process termination at 22:01:56.456 UTC; the exact native
exit-delay cause remains unestablished. The unverified runtime was rejected before
its update consumer started.

A focused rerun of exactly those two scenarios using the same packaged artifacts
passed **2/2 in 14.1 seconds**, without changing their assertions, timeouts or
production source. Thus all nine cases have passing evidence, but this is not
reported as a single clean nine-test run. The original errors and OS log remain
preserved in `.cache/update-research/review-native-first-contexts/` and
`.cache/update-research/review-shell-6777-os.log`.

Additional local receipts: `review-go-red.log`, `review-go-green.log`,
`review-native-red.log`, `review-native-green.log`, `review-native-rerun.log`, and
`review-desktop-final.log` under `.cache/update-research/`. `review-deadline-red.log`
captures the HTTP deadline reproduction; the final affected gate owns its green
verification. No production signing,
Rosetta or Linux runtime claim is added by these review fixes.

## Release acceptance still outside this local proof

- Install an actually published older app and update through the production-signed
  candidate's complete metadata/signature chain. Both local fixture apps contain
  repaired updater code, so their success cannot prove repair of beta.35's
  already-installed parser or other old outgoing updater code.
- Exercise a real Rosetta installation and Linux AppImage update. This run proves
  architecture selection at the Go I/O and pinned MacUpdater boundaries only.
- Walk the complete browser/CLI apply interaction and broader existing charter
  branches, including shutdown with active agent work, managed installs and install-directory
  permission failures. The local native tests start from a verified staged plan.

No release was published and no host app was replaced. Final-head CI and review
status are tracked in the PR; production success is not claimed. The [research summary](../../research/desktop-update-flow/README.md)
links causal reproductions, competitor evidence and the cross-surface audit.
