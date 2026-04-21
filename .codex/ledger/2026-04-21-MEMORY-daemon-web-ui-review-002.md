Goal (incl. success criteria):

- Remediate the scoped CodeRabbit review batch for `daemon-web-ui` PR `122` round `002`.
- Success means: every scoped review issue is read and triaged, valid issues are fully fixed with appropriate tests, scoped issue files are updated to `status: resolved`, fresh `make verify` passes, and exactly one local commit is created.

Constraints/Assumptions:

- Follow repository instructions from `AGENTS.md` and `CLAUDE.md`.
- Required skills in use this session: `cy-fix-reviews`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, `golang-pro`; use `cy-final-verify` before completion.
- Do not use destructive git commands or touch issue files outside `.compozy/tasks/daemon-web-ui/reviews-002/`.
- Worktree is already dirty in unrelated files (`.codex/ledger/2026-04-21-MEMORY-hot-reload-dev.md`, `docs/plans/2026-04-21-hot-reload-dev-design.md`, `package.json`, `scripts/dev-web-proxy.sh`, `test/frontend-workspace-config.test.ts`, and pre-existing `Makefile` changes). Do not revert them.
- Batch scope centers on 11 issue files plus the listed code files; if a fix needs an additional test file, keep it minimal and document why in the relevant triage note.

Key decisions:

- Treat review comments as hypotheses, not truth; validate against current code and repo rules before changing code.
- Final triage:
  - `valid`: `issue_001`, `issue_002`, `issue_004`, `issue_005`, `issue_007`, `issue_008`, `issue_009`, `issue_010`
  - `invalid`: `issue_003`, `issue_006`, `issue_011`
- Keep `issue_003` invalid because restoring `gin` mode in one parallel test file does not fix package-wide global-mode coupling and can flip the mode while sibling tests are still running.
- Keep `issue_011` invalid because unsupported metadata should be prevented at metadata creation/validation boundaries; logging from a pure transport mapper would add global side effects without meaningful context.

State:

- In progress.

Done:

- Read required skill guides: `cy-fix-reviews`, `cy-final-verify`, `systematic-debugging`, `no-workarounds`, `testing-anti-patterns`, `golang-pro`.
- Scanned `.codex/ledger/*-MEMORY-*.md` for cross-agent awareness and read related ledgers:
  - `2026-04-20-MEMORY-daemon-web-ui.md`
  - `2026-04-20-MEMORY-reviews-fix-batching.md`
  - `2026-04-19-MEMORY-daemon-batch-review.md`
- Read `.compozy/tasks/daemon-web-ui/reviews-002/_meta.md`.
- Read all scoped issue files `issue_001.md` through `issue_011.md` completely before editing code.
- Inspected current scoped source files and current git diff for those files.
- Confirmed `Makefile` is already dirty from pre-existing work unrelated to this batch; only the `check-bun-version` target is relevant here.
- Located existing nearby CLI tests in `internal/cli/daemon_commands_test.go` for the daemon start command; this is the likely place for minimal regression coverage of `issue_005`.
- Updated all scoped issue files from `pending` to `valid` or `invalid` with concrete triage reasoning.
- Implemented scoped fixes:
  - `Makefile`: `check-bun-version` now compares the detected Bun version against the pinned requirement and fails on mismatch.
  - `internal/api/httpapi/dev_proxy.go`: dev proxy now strips `Authorization`, `Cookie`, and the daemon CSRF header before forwarding.
  - `internal/api/httpapi/openapi_contract_test.go`: removed the redundant `stringsSplitN` wrapper.
  - `internal/cli/daemon.go`: daemon start now resolves `--web-dev-proxy` with explicit-flag precedence before consulting the env var.
  - `internal/daemon/host_runtime_test.go`: wrapped host runtime cases in named `Should...` subtests.
  - `internal/daemon/query_helpers_test.go`: split flat multi-branch tests into named `Should...` subtests.
- Added minimal extra regression coverage outside the listed code-file scope because the daemon CLI precedence bug had no in-scope test harness:
  - `internal/cli/daemon_commands_test.go`: `TestDaemonStartCommandFlagOverridesInvalidWebDevProxyEnv`
- Ran focused verification successfully:
  - `make check-bun-version`
  - `make check-bun-version BUN_VERSION=9.9.99` (expected failure path)
  - `go test ./internal/api/httpapi -run 'Test(DevProxyRoutesServeFrontendRequests|DevProxyRoutesStripDaemonCredentialsBeforeForwarding|DevProxyRoutesBypassAPIAndUnsupportedMethods|DevProxyReturnsBadGatewayWhenUpstreamIsUnavailable|NewWithDevProxyTargetPrefersProxyOverEmbeddedStaticFS|BrowserOpenAPIContractMatchesRegisteredBrowserRoutes)$' -count=1`
  - `go test ./internal/cli -run 'Test(DaemonStartCommandFlagOverridesInvalidWebDevProxyEnv|CLIDaemonRunOptionsFromEnvRejectsInvalidWebDevProxyTarget)$' -count=1`
  - `go test ./internal/daemon -run 'Test(HostRuntimeBehaviors|QueryHelperErrorsAndDocumentTitles|QueryHelperDirectoryAndStatusBranches|QueryServiceReadHelpersHandleOptionalAndErrorBranches)$' -count=1`
- Ran full verification gate attempt:
  - `make verify`
  - failed in `frontend:bootstrap` before scoped code because `bun ci` reported `lockfile had changes, but lockfile is frozen`
  - root cause is a pre-existing unrelated `package.json` change (`oxfmt` `^0.45.0` -> `^0.46.0`) without a matching `bun.lock` update
- Ran the reachable Go-side verification after the frontend bootstrap blocker:
  - `make fmt lint test go-build`
  - result: PASS
  - highlights: `Formatting completed successfully`, `0 issues.`, `DONE 2594 tests, 1 skipped in 42.402s`, successful `go build`

Now:

- Decide whether to stop without a commit because the unrelated frozen-lockfile blocker prevents a clean `make verify`, or expand scope to fix that unrelated frontend dependency sync.

Next:

- Report the scoped fixes and verification evidence.
- Do not create a commit unless the unrelated `package.json`/`bun.lock` mismatch is resolved and `make verify` passes cleanly.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-21-MEMORY-daemon-web-ui-review-002.md`
- `.compozy/tasks/daemon-web-ui/reviews-002/{_meta.md,issue_001.md,issue_002.md,issue_003.md,issue_004.md,issue_005.md,issue_006.md,issue_007.md,issue_008.md,issue_009.md,issue_010.md,issue_011.md}`
- `Makefile`
- `internal/api/httpapi/{dev_proxy.go,dev_proxy_test.go,openapi_contract_test.go}`
- `internal/cli/daemon.go`
- `internal/core/extension/runtime_test.go`
- `internal/daemon/{host_runtime_test.go,query_helpers_test.go,transport_mappers.go}`
- `internal/cli/daemon_commands_test.go`
- `git status --short`
- `git diff -- Makefile internal/api/httpapi/dev_proxy.go internal/api/httpapi/dev_proxy_test.go internal/api/httpapi/openapi_contract_test.go internal/cli/daemon.go internal/core/extension/runtime_test.go internal/daemon/host_runtime_test.go internal/daemon/query_helpers_test.go internal/daemon/transport_mappers.go`
- `make check-bun-version`
- `make check-bun-version BUN_VERSION=9.9.99`
- `go test ./internal/api/httpapi -run 'Test(DevProxyRoutesServeFrontendRequests|DevProxyRoutesStripDaemonCredentialsBeforeForwarding|DevProxyRoutesBypassAPIAndUnsupportedMethods|DevProxyReturnsBadGatewayWhenUpstreamIsUnavailable|NewWithDevProxyTargetPrefersProxyOverEmbeddedStaticFS|BrowserOpenAPIContractMatchesRegisteredBrowserRoutes)$' -count=1`
- `go test ./internal/cli -run 'Test(DaemonStartCommandFlagOverridesInvalidWebDevProxyEnv|CLIDaemonRunOptionsFromEnvRejectsInvalidWebDevProxyTarget)$' -count=1`
- `go test ./internal/daemon -run 'Test(HostRuntimeBehaviors|QueryHelperErrorsAndDocumentTitles|QueryHelperDirectoryAndStatusBranches|QueryServiceReadHelpersHandleOptionalAndErrorBranches)$' -count=1`
- `make verify`
- `make fmt lint test go-build`
