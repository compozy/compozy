# File explorer S2 backend evidence

Scope: S2 of `.compozy/tasks/file-explorer/_slices.md`, branch `feat/file-explorer-s2`, based on S1 `c11456105`.
The owning impact audit remains the spec's `_impact-entry.md`; S10 owns consolidation, official skill/site documentation, and integrated UI QA. Affected scenario: `ET-session-review-changes`.

## Implementation

- `internal/projectfiles/changes`: stateless `Engine` with summary, patches, contents, and bases. It uses S1 `GitReadArgs` and only `StreamingGitRunner`, with literal pathspecs, ten-second Git timeouts, confined new-side reads, per-side sensitive gates, and bounded outputs.
- Summaries expose at most 3,000 rows, retain totals beyond that limit, and mark incomplete line/count budgets as partial. Raw Git metadata supplies status, rename identity, and old OIDs; numstat and porcelain parsing reuse `worktree` parsers.
- File identity includes status, both rename paths, old OID, resolved metadata, inode/change time on macOS/Linux and file ID/change time on Windows, and raw-byte hashes for small files. Summary snapshots retry once; patch/content operations fence individual files before and after production and recheck the revision.
- Shared core handlers resolve the same session root and profile fences as S1. Both HTTP and UDS expose all four operations. App-token `reveal=true` fails before engine access. Existing session PR projections decorate summaries without a forge refresh.
- Additive OpenAPI, contract samples, TypeScript SDK, and Go SDK are regenerated with the owner. CLI/native tool and frontend remain S7/S5 work.

## Canonical test ownership

The new package owns engine behavior in `internal/projectfiles/changes/engine_test.go`; no existing suite could own a new package. Core credential/root errors extend `internal/api/core/session_files_test.go`; transport profile fences extend `internal/api/testutil/profile_read_transport_parity_test.go`; real daemon parity extends `internal/daemon/session_files_integration_test.go`. The existing app policy oracle records the four new allowed operations.

| IDs | Test / subtest |
| --- | --- |
| UT-019, UT-020, UT-021, UT-022, UT-023 | `TestEngine/Should describe branch renames working edits and untracked files` |
| UT-024, UT-038 | `TestEngine/Should cap each patch independently without the runner buffer limit` |
| UT-025, UT-026, UT-027 | `TestEngine/Should gate sensitive sides and classify binary generated and deleted files` |
| UT-028, UT-030, UT-031, UT-039 | `TestEngine/Should reject unsafe inputs and enumerate bases` |
| UT-029 | `TestEngineLimitsAndRaces/Should resolve worktree remote and fallback bases` |
| UT-032 | `TestEngineLimitsAndRaces/Should cap summary rows while preserving totals` |
| UT-033 | `TestEngine/Should fence individual edits and moved HEAD revisions` |
| UT-034 | `TestEngine/Should reject unsafe inputs and enumerate bases` |
| UT-035 | `TestEngine/Should never execute configured git programs or mutate repository state`; `TestEngineLimitsAndRaces/Should discover drivers added between reads and preserve git state` |
| UT-036 | `TestEngine/Should confine changes to a subdirectory root`; `TestEngineLimitsAndRaces/Should classify conflicts within a subtree` |
| UT-037 | `TestEngine/Should fence individual edits and moved HEAD revisions`; `TestEngineLimitsAndRaces/Should detect edits during patch generation`; `Should follow target identity and sensitivity for symlinks` |
| IT-005, IT-006, IT-016, IT-018, IT-011 (successful changes responses) | `TestDaemonSearchSessionFilesWorktreeBoundIT005/Should expose Changes summary patches contents and bases identically over HTTP and UDS` |
| IT-007 | `TestProjectChangesUserScope/Should report user session roots as unavailable for every Changes operation` |
| IT-010 | `TestWorkspaceSessionProfileFenceHTTPUDSTransportParity` four changes cases |
| IT-018 (attributed core caller) | `TestProjectChangesReveal/Should reject app token reveal before running git` |

IT-016's bases shape is exercised by the daemon journey; nonrepository classification is covered by the owning engine suite. `TestEngineLimitsAndRaces/Should compute fingerprints only for requested targets` instruments the reader boundary and proves that a one-file detail request never reads other changed files.

## Consumer notes

S5: operation IDs are `getSessionChanges`, `getSessionChangePatches`, `getSessionChangeContents`, and `listSessionChangeBases`. Summary uses `root`, `head`, `base`, optional `pull_request`, `totals`, `files`, `truncated`, and `revision`. Detail requests require the summary revision and the same scope/base. `path` and `old_path` arrays align positionally; omit old paths for a wholly nonrename batch. A moved HEAD/base returns `409 changes_stale`. A working-file edit can retain the revision but changes its fingerprint: compare each item with the summary before rendering. Missing contents sides are omitted; gated sides have `sensitive: true` and no text. Patches have `too_large: true` without text when capped. `totals.partial` indicates lower bounds.

S7: inject or construct `changes.Engine` via `changes.NewEngine`, with optional `WithGitRunner`/`WithReader`; use `Summary`, `Patches`, `Contents`, and `Bases` directly. Resolve `projectfiles.Root` from registered workspace/worktree metadata, including `BaseRef`; never use resumed session CWD. Reuse the HTTP/core reveal and root authorization semantics at the native boundary.

## Implementation choices

- The engine uses Git's `--literal-pathspecs` global flag instead of changing process-global environment; it is the command-scoped equivalent of `GIT_LITERAL_PATHSPECS=1`.
- Raw diff metadata already contains status and old OIDs, so a redundant `--name-status` process is omitted.
- Batches are processed one target at a time, passing both rename paths. `go-gitdiff` selects/formats the matching per-file patch. This bounds parser memory and isolates oversized files without buffering the batch.
- Small-file byte hashing runs on every fingerprint, a stronger policy than only hashing in the racy timestamp window.
- No repository-wide tests/lint, race suites, or E2E were run. S10 owns integrated UI QA after all slices land.

## Validation

Successful checks so far (repository root unless stated otherwise):

```sh
go test ./internal/projectfiles/changes -count=1 -cover
# 83.6% before final snapshot-comparison refinement; 83.4% after it.
go test -ldflags=-w ./internal/projectfiles/changes -count=1 -cover
go test ./internal/projectfiles/changes ./internal/api/core ./internal/api/spec ./internal/api/testutil -run 'TestEngine|TestProject|TestAppAccessPolicy|TestProfileDetailReadHTTPUDSTransportParityIT070' -count=1 -cover
go test ./internal/api/testutil -run '^TestWorkspaceSessionProfileFenceHTTPUDSTransportParity$' -count=1
go test -ldflags=-w ./internal/api/core -run 'TestProjectChangesUserScope|TestProjectChangesReveal' -count=1
go test -tags integration ./internal/daemon -run '^TestDaemonSearchSessionFilesWorktreeBoundIT005$' -count=1
go run ./cmd/compozy-codegen all
go run ./cmd/compozy-codegen check
bun x turbo run typecheck --filter=@compozy/client
GOOS=windows GOARCH=amd64 go build ./internal/projectfiles/...
# sdk/go:
go test ./client
```

The initial real-daemon S2 journey passed (137.861 s), and engine/core/app-policy/profile checks passed independently. A later combined core/profile rerun and several lint/daemon reruns hit host disk exhaustion while compiling/linking, rather than assertions. The final daemon rerun propagates `GOFLAGS=-ldflags=-w` to its harness-built daemon to omit debug symbols and reduce temporary disk needs. The locked SDK tooling and its `make codegen-check` prerequisite passed; that prerequisite also verified module tidy, manifests, CLI docs, and migration guides.

The read-only test-conventions checker passed for the changed engine, core, profile, app-policy, and daemon suites. No assertions were weakened. Final receipts:

```sh
# Pinned binary: ~/.local/share/mise/installs/golangci-lint/2.15.0/golangci-lint-2.15.0-darwin-arm64/golangci-lint
golangci-lint run --allow-parallel-runners --concurrency 2 ./internal/projectfiles/changes ./internal/api/core ./internal/api/spec ./internal/api/contract ./internal/api/httpapi ./internal/api/udsapi ./internal/api/testutil ./internal/daemon
# PASS: 0 issues.
GOFLAGS=-ldflags=-w go test -tags integration ./internal/daemon -run '^TestDaemonSearchSessionFilesWorktreeBoundIT005$' -count=1
# PASS: 22.015 s, including final HEAD movement -> changes_stale assertion.
go test -ldflags=-w ./internal/projectfiles/changes -run 'TestEngineLimitsAndRaces/Should_compute_fingerprints_only_for_requested_targets' -count=1
# PASS: 0.725 s.
```

The last full engine run passed with 83.4% coverage; the subsequently added subset-instrumentation case passed independently. Final core user-scope/reveal tests passed in 0.843 s. Generated Go SDK tests passed in 36.438 s. Windows cross-build passed; Windows native runtime remains CI-owned. The TypeScript SDK's Turbo lane completed both tasks successfully, including its full codegen-check prerequisite.

Commit hooks run the repository checks through a temporary hook directory, with `lint-staged --no-stash --no-revert`, to honor the explicit prohibition on destructive Git operations. Commit-message validation remains enabled. No push or PR is part of S2.


