# File explorer S1 backend evidence

Scope: S1 of `.compozy/tasks/file-explorer/_slices.md` on `feat/file-explorer-s1`.
Public contracts are additive. No schema, config, hook, native tool, CLI, or frontend behavior changes.
The owning cross-surface audit is the spec's `_impact-entry.md`; S10 owns its consolidation,
site/official-skill documentation, and integrated QA scenarios. Affected scenarios:
`APP-open-transcript-paths-in-editor` and `ET-session-review-files`.

## Implementation

- `internal/projectfiles`: root canonicalization, read/list interfaces, confined descriptor reads,
  protected and sensitive target checks, natural ordering, caps, image/binary/text classification,
  status decorations, and the exported shared Git policy.
- Shared core handlers authorize the workspace, session, and worktree profile before reading.
  Session roots use the same workspace/worktree metadata underlying `WorkingDir`, excluding its
  resumed CWD. HTTP and UDS register the same four handlers.
- OpenAPI, TypeScript client, Go client, contracts and samples are generated together.
- `workspacefiles.ClassifySymlink` is the existing classifier exposed for reuse.
- `testutil.ConfigureGitReadMarkers(t, root)` is the shared executable-marker fixture for S2/S8.

## Test ownership and mapping

Reader invariants belong to `internal/projectfiles/reader_test.go`, `TestReader` (a new package
requires a new canonical suite). Handler credential/redaction behavior extends
`internal/api/core/session_files_test.go`. Profile transport fencing extends
`internal/api/testutil/profile_read_transport_parity_test.go`. Real-daemon routes extend
`internal/daemon/session_files_integration_test.go`, reusing its existing runtime/worktree fixture.
Route inventory and app policy assertions remain in their existing suites.

| ID | TestReader subtest |
| --- | --- |
| UT-001 | Should sort directories first in natural order |
| UT-002 | Should decorate status within a subdirectory without running configured programs |
| UT-003 | Should cap listings only beyond five thousand entries |
| UT-004 | Should classify symlinks and protect resolved targets |
| UT-005 | Should reject malformed paths for list and read; Should classify symlinks and protect resolved targets |
| UT-006 | Should return entries when git times out |
| UT-007 | Should classify missing files directories and unavailable roots |
| UT-008 | Should reject malformed paths for list and read |
| UT-009 | Should reject an alias swapped outside the root at open time |
| UT-010 | Should read text with language size and revision |
| UT-011 | Should classify missing files directories and unavailable roots |
| UT-012 | Should truncate text at a UTF8 boundary |
| UT-013 | Should classify images binary and sensitive contents |
| UT-014 | Should classify images binary and sensitive contents |
| UT-015 | Should classify images binary and sensitive contents |
| UT-016 | Should classify symlinks and protect resolved targets |
| UT-017 | Should classify symlinks and protect resolved targets |
| UT-018 | Should decorate status within a subdirectory without running configured programs; shared marker fixture |

Integration cases below extend `TestDaemonSearchSessionFilesWorktreeBoundIT005`; that existing
suite name retains its earlier composer-spec ID, distinct from the new spec's test IDs.

| ID | Existing suite / new case |
| --- | --- |
| IT-001 | Should serve project files with worktree roots and HTTP UDS parity |
| IT-002 | Should report a removed worktree as root unavailable |
| IT-003 | Should serve project files with worktree roots and HTTP UDS parity; raw byte parity in the sensitive-content case |
| IT-004 | Should fence sensitive content for app tokens on both root forms |
| IT-010 | TestWorkspaceSessionProfileFenceHTTPUDSTransportParity: workspace file entries/content foreign read, file entries/content foreign read |
| IT-011 | Should fence sensitive content for app tokens on both root forms: both content routes and entries error responses; core TestProjectFileContentReveal also rejects host-root leakage |
| IT-017 | Should fence sensitive content for app tokens on both root forms; core TestProjectFileContentReveal |

## Verification

Commands run from repository root unless stated otherwise. Cache paths isolate this worktree
from sandbox-protected user caches.

```sh
export GOCACHE=/private/tmp/file-explorer-s1-go-cache
export GOLANGCI_LINT_CACHE=/private/tmp/file-explorer-s1-lint-cache

go test ./internal/projectfiles ./internal/workspacefiles ./internal/testutil ./internal/api/core ./internal/api/contract ./internal/api/spec ./internal/api/httpapi ./internal/api/udsapi ./internal/api/testutil ./internal/daemon

go test ./internal/projectfiles -count=1 -cover

go test -tags integration ./internal/daemon -run '^TestDaemonSearchSessionFilesWorktreeBoundIT005$' -count=1

go test ./internal/projectfiles ./internal/api/core ./internal/api/spec ./internal/api/httpapi ./internal/api/udsapi ./internal/api/testutil -run 'TestReader|TestProjectFileContentReveal|TestWorkspaceSessionProfileFence|TestAppAccessPolicy|TestRegisterRoutesCoversTechSpecEndpoints' -count=1

golangci-lint run --concurrency 3 ./internal/projectfiles/... ./internal/workspacefiles/... ./internal/testutil ./internal/api/core/... ./internal/api/contract/... ./internal/api/spec/... ./internal/api/httpapi/... ./internal/api/udsapi/... ./internal/api/testutil/... ./internal/daemon/...

go run ./cmd/compozy-codegen all
go run ./cmd/compozy-codegen check
bun x turbo run typecheck --filter=@compozy/client

# From sdk/go:
go test ./client

# From repository root:
GOOS=windows GOARCH=amd64 go build ./internal/projectfiles
```

Use the pinned golangci-lint 2.15.0 executable (the ambient shim selected a Go 1.26 binary
incompatible with this module). The full focused package tests passed. Reader coverage: 85.4%.
Real daemon integration and generated Go client tests passed with local-listener sandbox approval.
Generator `all` and `check` passed. The Turborepo SDK typecheck and its full `make codegen-check` prerequisite passed. Focused lint finished with zero issues. No repository-wide lint/test gates, race suites or E2E ran.
Test-convention heuristics passed for the changed reader, core, integration and profile suites.

The package test suite also passed after exposing the existing symlink classifier and shared marker helper.

Initial sandbox runs could not open listeners or use user caches. An initial generator run resolved
an unpatched ancestor dependency; installing the worktree's frozen dependencies restored the
required generator patch. Neither failure required relaxing a product assertion.

## Integration notes and residuals

```go
projectfiles.GitReadArgs(ctx context.Context, runner worktree.StreamingGitRunner,
    root, command string, args ...string) ([]string, error)
```

Location: `internal/projectfiles/git.go`. Every caller must execute the resulting argv using
`RunStreaming`, not the runner's bounded `Run`. The real-runner timeout unit test uses a nanosecond budget to deterministically exercise failure,
rather than relying on a platform-dependent 1 ms scheduling race. Filter discovery happens per call for status,
diff and ls-files. Grep gets `--no-textconv`, never `--no-ext-diff`. S2 must add its literal-pathspec
environment policy; S8 must retain explicit glob validation and path confinement.

The config-rewrite race documented by the spec remains outside the threat model. Status is read
fresh per listing rather than cached; no invalidation protocol is introduced in this slice.
Root authorization stays in core, with filesystem canonicalization in projectfiles, so the domain
package does not import session or API packages. Downstream core routes can reuse projectFileRoot.

App-token authentication is an HTTP middleware boundary. UDS remains a trusted local transport
and does not authenticate a supplied bearer token; raw parity checks use local operator requests.
HTTP app-token tests cover both workspace and session forms, and the shared core handler independently
proves that attributed app callers cannot reveal. User session auth/local operator reveal succeeds.

Windows cross-compilation passes; native Windows runtime tests remain CI-owned. S10 still owns
integrated UI QA after the frontend slices land.
