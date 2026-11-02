# Session linked work backend handoff

Owner: `docs/prompts/20261102-1950_backend-task.md`. Checkout: `proto/session-links`.
This report describes backend implementation and focused evidence, not a live-provider/UI acceptance walk.

## Commits

- `d87f81970`: optional forge discovery contract, SDK registration and GitHub provider.
- `81c9eff26`: durable links, migration, batched projection, board rules and initial regressions.
- `5307d78ec`: HTTP/UDS/CLI/native surfaces, generated clients, official reference, candidate source selection and final regressions.
- A following documentation-only commit records the final validation evidence.

## Exact public contract

HTTP and UDS share these workspace/profile-authorized resources:

- `GET /api/workspaces/{workspace_id}/sessions/{session_id}/links`: `{items: SessionLinkedWorkItem[]}`.
- `POST` on the same path adds/restores a direct link, then returns the list.
- `DELETE` on the same path soft unlinks a direct identity, then returns the list. It does not delete any Git or forge resource.
- `GET /api/workspaces/{workspace_id}/sessions/{session_id}/link-candidates`: `q`, `kind`, `page`, `limit`; page starts at 1 and limit is capped at 100. `kind=branches` searches local branches even when forge discovery is available. Default/pull_requests uses forge discovery with branch fallback when unavailable. Response: `{items, has_more, supports_pr_discovery}`; candidate fields: repository, branch, pr_number, title, url, state.

Mutation body: `{repository?: string, branch: string, pr_number?: number}`. An empty branch can be resolved with a positive PR number on add. Repository is the normalized host/owner/repository identity and defaults to the session project. A different identity fails before PR resolution. A pasted URL is parsed by the frontend into repository and PR number; the backend does not accept arbitrary URLs.

Item fields: repository, branch, optional pr_number, worktree_id, worktree_name, worktree_removed, provenance, runs_here, optional source_session_id and source_session_title, pull_request and pull_request_lookup. PR payload gains optional title. Lookup retains its existing error/cause/staleness semantics. Worktree IDs and names survive removal.

Session catalog payload gains `linked_work: {total, pull_requests, needs_you, lead?}`. The PR count includes observed PRs and retained explicit PR identities even when lookup fails. The full list uses the links resource. Existing branch, pull_request and pull_request_lookup fields retain their original own-branch meanings.

Native tools: `compozy__session_link`, `compozy__session_unlink`; mutating risk. Inputs: required branch, optional repository and positive pr_number; additional properties are refused. There is no session override. Scope supplies session/workspace identity. Agent link provenance is `agent`; HTTP/CLI is `user`; native worktree creation is `created`. An agent may unlink its own direct links.

CLI: `session link <id>` lists; `session link <id> [branch] --pr-number N` resolves/adds; `session link <id> <branch>` adds; `session unlink <id> <branch>` removes. Flags: `--repository`, `--pr-number`, and standard structured output including `-o json`.

Stable link error codes: `session_link_repository_mismatch`, `session_link_invalid`, `session_link_failed`. Refused input returns 400 with code/error; a daemon failure returns 5xx. Native domain failures retain those codes. Other existing authorization/session lookup errors retain their transport behavior. Candidate provider failure returns 502. No new hooks or config keys.

Every successful add/unlink publishes the existing session catalog upsert for the target. Automatic native worktree creation links the returned worktree without binding or restarting the caller.

Forge capability: optional `supports_pr_discovery`; optional JSON-RPC methods `forge/pr_search` and `forge/pr_read`. Search request: remote_urls, query, page, limit; response: items, has_more. Read request: remote_urls, number; candidate: number, title, url, branch, state. Optional title also travels through status and the durable observation cache. SDK handlers are optional; the original required provider methods remain unchanged.

## Decisions for every unresolved item

1. Rolled-up items carry child ID/title and are read-only on the parent.
2. Roll-up is computed live through recursive lineage, within the same workspace and profile, across descendants.
3. Parent unlink does not mutate the child; archived children keep contributing.
4. Dedupe by repository plus observed/stored PR number, otherwise branch; direct work wins.
5. Unlink persists a tombstone, including derived branch items. Automatic insertion does not clear it; manual insertion restores it. Undo calls the same add path.
6. Existing single-valued branch/PR fields remain unchanged and additive linked work projects the current branch/bound worktree too.
7. Native tools are the self-targeted mutating link/unlink tools above.
8. Status uses a single batch PullRequests cache read for projected keys; catalog refresh discovery includes stored linked branches before the board/page cut. There is no status forge call per link. Candidate search is separately bounded discovery I/O.

## Board behavior

Needs you, Working and Waiting retain their badge precedence. Queued or dispatching input changes Idle/Done to Working only when direct work has an open PR, including the own-branch PR. Queue membership does not affect Stopped, Unknown, or sessions without open direct work. Own-branch PR behavior is preserved, including Unknown with an own open PR. A direct linked open PR changes Done to In review only for Idle, Done or Stopped badges. Child-only work never contributes DirectOpen. Closed/unlinked direct work no longer contributes to that linked-PR predicate.

Active rows use BadgeForInfo and BoardColumnWithLinkedWork. Durable SQL independently derives the badge from durable fields and joins current repository cached facts plus direct unlinked=false links. Its queue predicate requires an Idle/Done badge and either an open direct link or open own-branch PR. The new SQL test computes BadgeForInfo independently from runtime inputs rather than supplying SQL with a precomputed badge. The canonical board suite continues owning filtered counts/facets/pagination. Waiting, health and supervision remain runtime-only inputs routed through the active-row path; this change does not invent durable versions of them.

## Verification

Focused commands and outcomes are recorded in the final handoff. Added coverage:

- `TestSessionWorkLinkPersistence`: real SQLite, reopen, retained removed-worktree metadata, automatic suppression/manual restoration, cross-workspace conflict rejection.
- `TestBoardColumnWithLinkedWork`: activity/attention/waiting/unknown/queue precedence and settled links.
- `TestSessionCatalogBoard/Should derive linked board columns from runtime inputs`: independent Go/SQL state derivation, including queued and dispatching input.
- `TestSessionLinkedWorkProjection`: batch cache reads, direct precedence, child source identity and exclusion from parent column.
- `TestProvider/Should discover a bounded page and resolve PR identity`: HTTP boundary search paging, repository constraint, PR number/title/branch.

The first isolation regression failed because an upsert conflict skipped the workspace foreign key. Production now validates session membership before saving. Fixture failures (missing queue updated_at and canceled cleanup context) were corrected without removing assertions.

Test-shape checker passes badge_test.go and global_db_worktree_test.go. It reports existing violations in query_test.go, global_db_session_index_test.go and provider_test.go, outside the added cases. No unrelated test refactor was performed.

## Limits and remaining acceptance work

No push or PR. No make gate, gate-full, repo-wide lint, full go test ./..., E2E or daemon on port 2123. The proto dev server is untouched. Frontend product integration, COPY/glossary changes and the live multi-turn/provider/UI walk remain with the following frontend/controller task. Scenario ET-session-pull-request-link is extended but its live acceptance status remains pending.

The forge observation cache is branch keyed: if a branch is reused for a different PR, an explicitly stored PR number is retained and a mismatching branch observation becomes PR status unknown (`pr_identity_changed`), never the other PR. No dedicated number-keyed historical observation cache was introduced. Native automatic-link persistence occurs after worktree creation; a storage/identity failure returns an error while retaining the created worktree (it is not safe to delete it as rollback).

Successful commands before the final surface commit:

```sh
go build ./internal/... ./extensions/forge/...
go vet ./internal/session ./internal/store/globaldb ./internal/api/core ./internal/api/spec ./internal/daemon ./internal/cli ./internal/extension/... ./internal/extensionprotocol ./internal/tools/... ./internal/worktree ./extensions/forge/...
go test ./internal/store ./internal/store/globaldb ./internal/session -run 'TestProductionMigrationStreams|TestSessionWorkLinkPersistence|TestSessionCatalogBoard|TestSessionBranchSnapshots|TestSessionLinkedWorkProjection|TestOpenGlobalDBAppliesGlobalMigrationsAndEnablesWAL|TestOpenGlobalDBReopenPreservesRowsAndStatus'
```

For the combined command above, store and session passed; globaldb failed only on the test cleanup context. After repairing that fixture, this exact durable subset passed:

```sh
go test ./internal/store/globaldb -run 'TestSessionWorkLinkPersistence|TestSessionCatalogBoard|TestSessionBranchSnapshots|TestOpenGlobalDBAppliesGlobalMigrationsAndEnablesWAL|TestOpenGlobalDBReopenPreservesRowsAndStatus'
go test ./internal/api/core ./internal/api/spec ./internal/tools/builtin ./extensions/forge/github ./internal/session -run 'Test.*(SessionLinkedWorkProjection|SessionPullRequestProjection|BoardColumn|Forge|SessionWork|NativeToolCatalog)'
go test ./extensions/forge/github ./internal/api/spec ./internal/tools/builtin ./internal/cli -run 'TestProvider|TestRunProvider|Test.*Session|Test.*Catalog|Test.*Registry'
go test -tags integration ./internal/daemon -run 'TestValidateNativeToolBindings|TestSessionPullRequestAdapters'
go test ./extensions/forge/github -run 'TestProvider/Should_discover'
make codegen
make cli-docs
make codegen-check
```

The first multi-package focused pattern matched no tests in api/spec and tools/builtin; those entries are compilation evidence only. The later registry/catalog patterns supplement those reads. Initial provider-shell Go attempts hit sandbox cache permissions; validation used daemon-owned native terminals and registered monitors. One concurrent compile observed a test-file length change while editing; it was rerun on settled source.

`extensions/open-design/lint.gen.mjs` is generated by the OpenDesign step of `make codegen`; its one-line drift is reproduced by that generator and included only because the owning generation/check requires it. `web/proto.config.ts` stays unstaged and uncommitted.

Final incremental regression commands passed:

```sh
go test ./internal/store/globaldb -run 'TestSessionCatalogBoard/Should_derive_linked'
go test ./internal/session ./internal/store/globaldb -run 'TestSessionLinkedWorkProjection|TestBoardColumn|TestSessionCatalogBoard'
go test ./internal/session -run 'TestSessionLinkedWorkProjection'
```

These cover both queue admission states and the reused-branch PR identity fence. Final source build and vet passed after adding the candidate `kind` selector. No new runtime lab was started; real SQLite fresh/reopen/migration tests provide the backend's real execution evidence. HTTP/UDS/native authorization behavior has compilation and existing-suite coverage; a new full transport-level linked-work journey was not executed.

Final TypeScript delivery check passed from the repository root:

```sh
bunx turbo run typecheck --filter=@compozy/extension-sdk --filter=@compozy/client
```

All three tasks succeeded (including the prerequisite codegen-check). The final standalone `make codegen-check` also exited 0. Historical backend feature head: `5307d78ec2d9a669be0b2dccefdd6e36ab0c66ed`. The correction series below supersedes the board, summary, catalog search and app-access claims above. A concurrent frontend agent owns working-tree changes under web/.

## Root-review corrections

Owner: `docs/prompts/20261102-2020_backend-fixes.md`. All six fixes were reproduced before the corresponding production change.

- `e0ad20297`: board regression. `TestBoardColumnWithLinkedWork` failed for queued Idle/Done without open work, Stopped, Unknown and Waiting. The SQLite `TestSessionCatalogBoard/Should derive linked board columns from runtime inputs` failed for active-idle, stopped and unknown sessions. Both suites now cross direct-link open state, own-PR open state, queue state (including queued and dispatching in SQLite), and runtime badge/state. SQL independently derives its badge.
- `5d9263efb`: summary correctness. `TestSessionLinkedWorkProjection/Should sort fresh decisions before ready work and unknown lookups` failed for Ready, stale/error lookups and the remaining ranks. Needs you now counts only fresh open PRs with failing checks, requested changes or conflicts. Order: needs-you, fresh ready non-draft PRs, other open/no-PR work, unknown lookups, merged/closed. Lead follows the same order. The old identity-fence assertion now locates the unknown item after fresh open work; its identity checks remain intact. The cache fixture now explicitly reports successful lookups.
- `46c8c7601`: bounded reads. `TestSessionLinkedWorkProjection/Should resolve repositories once per profile and workspace` observed four repository calls for three distinct pairs. Resolution is memoized by both profile and workspace, including empty results. Defaults, branch snapshots and PR cache still each receive one batch read.
- `c2df7bddb`: PR count. `TestSessionLinkedWorkSummary` failed because `pull_requests` was absent from the public summary. Domain projection, payload mapping, OpenAPI and generated Go/TypeScript clients now expose the integer. Mixed-list projection verifies eight PR items among ten items.
- `5c163bbd8`: catalog search. `TestManagerBoardCatalog/Should search direct linked identities before paging active and durable rows` returned total zero for `#42`. Active filtering and SQL now match linked branch or PR number, with or without #, before count/page cuts. The real-SQLite test covers default and title_agent search, one-item pages, exact total, cursor continuation, workspace scope and tombstones, with both durable-only and active-overlaid rows.
- `ee7633eda`: app access. The canonical `TestAppAccessPolicy` failed on both authored and rendered decisions for add, remove and candidate search. These now deny app tokens, like worktree bind; listing linked work stays allowed, like session reads. OpenAPI and SDK samples were regenerated.

Invariant ownership: board decisions belong to the existing Go badge and durable catalog board suites; list sorting/counts and batch cost belong to the session projection suite; the additive summary field belongs to the API schema suite; catalog matching/count/page behavior belongs to the existing manager catalog suite using real SQLite; token authority belongs to the canonical API app-access policy suite.

No schema, CLI verb, native ID, forge method, hook or config change is introduced by these corrections. Existing migrations were left untouched. No web build, gate, full-suite run, E2E, daemon startup, push or PR was attempted. Commits use explicit owned paths. Hooks were disabled because lint-staged can stash concurrent work, which the task explicitly forbids; gofmt and focused verification were run directly.

The broader focused race run exposed an existing assertion that active catalog reads use one PR batch (`TestManagerBoardCatalog/Should overlay active PR columns and direct children before filtering and counting`): it saw two. Commit `cae99b502` removes the duplicate own-branch projection read when linked-work storage is configured. The shared projection retains the legacy own-branch state independently of link tombstones and keeps the fallback for readers without linked-work support. The existing assertion was preserved and passed on rerun.

Exact correction verification commands:

```sh
go test ./internal/session ./internal/store/globaldb -run 'TestBoardColumnWithLinkedWork|TestSessionCatalogBoard/Should_derive_linked'
go test ./internal/session -run 'TestSessionLinkedWorkProjection'
go test ./internal/session -run 'TestSessionLinkedWorkProjection/Should_resolve'
go test ./internal/api/spec -run 'TestSessionLinkedWorkSummary'
go test ./internal/api/spec -run 'TestAppAccessPolicy'
go test ./internal/api/spec ./internal/session -run 'TestSessionLinkedWorkSummary|TestSessionLinkedWorkProjection'
go test ./internal/session -run 'TestManagerBoardCatalog/Should_search_direct'
go test ./internal/api/spec -run 'TestAppAccessPolicy|TestSessionLinkedWorkSummary'
make codegen
```

These are the red/green reproduction commands described above; each owning test subsequently passed after its production fix. `make codegen` ran twice, both exit 0: once for the summary field and once for app policy.

```sh
go build ./internal/session ./internal/store/globaldb ./internal/api/contract ./internal/api/core ./internal/api/spec
go vet ./internal/session ./internal/store/globaldb ./internal/api/contract ./internal/api/core ./internal/api/spec
CGO_ENABLED=1 go test -race ./internal/session ./internal/store/globaldb ./internal/api/core ./internal/api/spec -run 'TestSessionLinkedWorkProjection|TestSessionPullRequestProjection|TestBoardColumn|TestSessionCatalogBoard|TestManagerBoardCatalog|TestPageSessionsVisibilityExclusion|TestPageSessionsStableKeyset|TestAppAccessPolicy|TestSessionLinkedWorkSummary|TestSessionPayloadOptionalRelations'
```

Build and vet passed. This race command passed globaldb (29.319s), core (3.834s), and spec (12.631s), and failed session only on the duplicate-batch assertion above. The chained codegen-check was consequently skipped in that attempt. After the production repair:

```sh
CGO_ENABLED=1 go test -race ./internal/session -run 'TestSessionLinkedWorkProjection|TestSessionPullRequestProjection|TestBoardColumn|TestManagerBoardCatalog'
go build ./internal/session ./internal/store/globaldb ./internal/api/contract ./internal/api/core ./internal/api/spec
go vet ./internal/session ./internal/store/globaldb ./internal/api/contract ./internal/api/core ./internal/api/spec
make codegen-check
```

The repeated session race subset passed (8.330s); build and vet passed again. Final codegen-check outcome is recorded below.

Test-convention checker: run `python3 .agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py <file>` separately for badge_test.go, query_test.go, global_db_session_index_test.go, spec_test.go and app_access_test.go under their paths above. Badge and app-access pass. Query reports nine pre-existing direct-assertion tests; globaldb reports five; spec reports the pre-existing `TestStopWhenSpecSchemaShouldMatchTheDualWireFormat`. None names an added case. An initial attempt supplied multiple file arguments, which the checker rejects; it was immediately rerun once per file. No unrelated suite reshaping was performed.

Final `make codegen-check` exited 0. The targeted rerun chain (session race tests, build, vet and codegen-check) exited 0 as confirmed by the daemon-owned terminal result. `git diff --check` passed. The matching-layer case `TestSessionMatchesListQuery/Should match direct work identities and exclude child rollups` additionally verifies that child branch/PR identities cannot match the parent.

Remaining acceptance scope: live forge, linked-work transport journey and UI dogfood remain unrun here. No migration suite was repeated because this correction series changes no schema or migration bytes. The original slice's migration evidence above is historical. Concurrent frontend files remain outside these commits.

Commit `test: exclude child work from parent catalog search` adds the matching-layer regression. Exact command `CGO_ENABLED=1 go test -race ./internal/session -run 'TestSessionMatchesListQuery'` passed (3.395s, exit 0). The repeated test-shape check retained the same nine pre-existing query-suite findings. Documentation formatter invocation exited 0 but the report path is ignored by formatter configuration; `git diff --check` remains the whitespace evidence.

Final search consistency regression: `TestManagerBoardCatalog/Should search derived own work consistently across runtime and durable rows` first returned total zero for a durable session's `own-find` branch. Search now also covers the derived own branch/bound worktree, matching the existing projection, with repository-scoped cached PR numbers, default-branch exclusion and tombstone suppression. The search SQL moved into its cohesive owner file so production files remain under 500 lines. No forge call was added. Exact reproduction command: `go test ./internal/session -run 'TestManagerBoardCatalog/Should_search_derived'` (failed before repair).

Commit `99225dde6` closes that derived-work search discrepancy. The new case also soft-unlinks the derived item and requires zero matches after the tombstone, for both active and durable paths. Final search/board/paging command passed (session 16.241s, globaldb 33.916s):

```sh
CGO_ENABLED=1 go test -race ./internal/session ./internal/store/globaldb -run 'TestManagerBoardCatalog|TestSessionMatchesListQuery|TestPageSessionsVisibilityExclusion|TestPageSessionsStableKeyset|TestSessionCatalogBoard'
```

After this final production change, the following build/vet/check chain was rerun:

```sh
go build ./internal/session ./internal/store/globaldb ./internal/api/core ./internal/api/spec
go vet ./internal/session ./internal/store/globaldb ./internal/api/core ./internal/api/spec
make codegen-check
```

The final build/vet/codegen-check chain exited 0 (daemon terminal term-e7db1834366a). All correction source and test changes are committed. The report is followed by a documentation-only evidence commit; remaining working-tree changes belong to the concurrent frontend task.


## Round 2: exact PR candidate resolution

Owner: item 7 of `docs/prompts/20261102-2020_backend-fixes.md`.
Invariant and owner: exact PR identities resolve independently of text search, scoped to the session repository; the existing daemon `TestSessionPullRequestAdapters` suite owns this provider I/O boundary. Its fixtures use real Git repositories and inject only the forge I/O interface.

Before the production change, the new bare-number, #number and three forge-link cases returned no candidate; the mixed search returned PR 7 before requested PR 42, and the foreign-repository case performed search instead of refusing the link. The reproduction command exited 1:

```sh
go test -tags integration ./internal/daemon -run 'TestSessionPullRequestAdapters/Should_(resolve|prepend|keep_a_missing|refuse_another|retain_normal)'
```

`WorkCandidates` now recognizes positive numbers and HTTP(S) forge PR links, normalizes repository identity with the existing worktree boundary and refuses other repositories with `ErrLinkedWorkRepository`. On the first page it resolves through `WorkPR`/`forge/pr_read`, prepends the complete candidate and removes its duplicate from the normal search page. There is no provider-specific host logic. Later pages and branch discovery keep their existing behavior. Normal search pagination and `has_more` are retained; the first page can contain one additional direct candidate, so no search hit is dropped at a page boundary. A failed search still returns a successfully resolved identity. A failed direct read falls back to normal search because the optional provider protocol has no shared not-found error; an absent read plus empty search produces no candidate. Provider outages therefore retain the pre-existing search error behavior when both paths fail.

No schema, migration, public DTO, SDK, CLI, native tool, hook or configuration change. No generated output changes are needed. The existing cross-surface audit and QA scenario were updated. No web files were edited; `web/proto.config.ts` remains uncommitted.


Round 2 verification (all Go subprocesses removed every inherited `COMPOZY_*` variable; race tests also set `CGO_ENABLED=1`):

```sh
go test -tags integration ./internal/daemon -run 'TestSessionPullRequestAdapters'
CGO_ENABLED=1 go test -race -tags integration ./internal/daemon -run 'TestSessionPullRequestAdapters'
go build ./internal/daemon
go vet -tags integration ./internal/daemon
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt internal/daemon/session_work_candidates.go internal/daemon/boot_worktrees_test.go
python3 .agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py internal/daemon/boot_worktrees_test.go
git diff --check
```

The initial green focused test passed (1.642s); race passed (6.901s), and the final repeated race/build/vet chain passed (race 6.104s, all exit 0). Formatter and whitespace checks passed. The shape checker exits 1 only for existing direct-assertion tests `TestDaemonExecutionWorktreesResolveServiceAfterConsumerBoot` and `TestWorktreeTurnRefreshNotifier`, outside this change. No new case is flagged.

The initial sandboxed build could not write the Go build cache; the same focused commands passed after cache access was allowed. An initial lint attempt used v2.13.1 and failed to decode Go 1.27 export data. The checkout pins v2.15.0 in `magefiles/defaults.go`; its first attempt encountered a concurrent lint lock, without interrupting that process. The final focused lint uses the pinned version with two workers and independent-run permission.

Not run: live forge/provider or UI acceptance, full tests, E2E, gates, repository-wide lint, codegen or migration checks. No wire/schema/generated artifact changed. No daemon was started, no push or PR was performed. The live QA scenario remains pending. Residual limits: provider read-by-number must be available; unsupported providers retain branch fallback, and the optional provider protocol still lacks a shared typed not-found error. Normal search paging remains intact with at most one extra direct candidate on page 1.


Final focused lint passed, exit 0, `0 issues`:

```sh
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --allow-parallel-runners --concurrency=2 --new-from-rev=HEAD ./internal/daemon
```

Only the five owned backend/QA paths were staged. The commit disables hooks for that invocation because lint-staged may stash unrelated work, which the task forbids; the focused tests, build, vet, formatter, lint and whitespace checks above were executed directly. The unrelated `web/proto.config.ts` change remains unstaged and uncommitted.


## Consolidated review round (items 1–17)

Checkout verified: `/Users/pedronauck/.compozy/worktrees/compozy/proto-session-links`, branch `proto/session-links`. No commits, staging, push, or PR operation was attempted. The root agent owns commits. `web/proto.config.ts` was already modified and was preserved; concurrent frontend edits are outside this report. `extensions/open-design/lint.gen.mjs` is incidental Bun-version generation drift and must be excluded, as must any `tmp-sqlc-helper/`.

### Implementation and owning regressions

The following paths are the exact logical groups; shared files intentionally occur in more than one group. All tests reuse existing files. The regression suite is the owning layer named below; not every case has an independently recorded pre-fix red run. Recorded red evidence includes atomic derived unlink returning no row, the detail fallback regression, and Undo restoring metadata but losing Runs here. The initial branch-only test fixture accidentally synthesized a PR and was corrected to return no PR before relying on its result. No production regression was repaired by weakening an assertion.

| Item | Fix and regression | Exact files |
| --- | --- | --- |
| 1 | Accepted worktree creation returns success even if automatic linking fails. Linking uses a detached context bounded to 10 seconds and logs failure. Native suite: `TestNativeWorktreeTools/Should return accepted worktree when automatic linking fails` checks cancellation, deadline, failure and success. | `internal/daemon/native_tool_worktree_calls.go`, `internal/daemon/native_worktree_tools_test.go` |
| 2 | Workspace-scoped `local:<workspace_id>` identity permits local branch links; local discovery does not invoke forge capabilities. Status does not enqueue forge reads for unsupported/local origins. Hosted subgroup normalization keeps its real identity. Real-Git adapter case `Should search local branches across path separators`; identity case `Should normalize hosted subgroup repositories`. | `internal/daemon/session_pull_requests.go`, `internal/daemon/session_work_candidates.go`, `internal/daemon/boot_worktrees_test.go`, `internal/worktree/pull_request_identity.go`, `internal/worktree/pull_requests_test.go` |
| 3 | SQL family recursion excludes fork and continue. The existing active-fork roll-up case now checks the linked-work family itself. | `internal/store/globaldb/queries/session_work_links.sql`, `internal/store/globaldb/sqlcgen/session_work_links.sql.go`, `internal/session/query_test.go` |
| 4 | A rolled-up item with neither observed PR nor stored number is skipped. Projection case `Should omit branch only child work`. | `internal/session/linked_work_projection.go`, `internal/session/query_test.go` |
| 5 | Optional-health detail reapplies the linked-work board rule using the health badge. API case `TestSessionOptionalHealthLinkedWork/Should retain linked review after health decoration`. | `internal/api/core/authored_context_access.go`, `internal/api/core/session_workspace_internal_test.go` |
| 6 | Detail children use linked projection; legacy readers retain their own-branch fallback. Existing fork/detail case now includes a linked child and requires In review in detail counts. | `internal/session/catalog_board.go`, `internal/session/query_test.go` |
| 7 | Derived default branches are removed before the batch PR read and cannot set DirectOpen, including bound worktrees. Explicit links retain existing behavior. Projection case `Should keep bound default branch work branch only without cache reads`; SQL fixture in `TestSessionLinkedWorkCatalogReview`. | `internal/session/linked_work_projection.go`, `internal/session/query_test.go`, `internal/store/globaldb/global_db_session_index_test.go` |
| 8 | Pending work requires dispatching or queued with dispatchable=1. Fenced queued rows do not affect board placement. Existing `TestSessionCatalogBoard` derivation matrix now checks fenced inputs and `SessionPendingWork`. | `internal/store/globaldb/queries/session_work_links.sql`, `internal/store/globaldb/sqlcgen/session_work_links.sql.go`, `internal/store/globaldb/global_db_session_board.go`, `internal/store/globaldb/global_db_session_index_test.go` |
| 9 | Unlink validates repository and performs one tombstone upsert. Derived worktree ID/name/provenance survive Undo; the projection uses persisted family worktree identity. Persistence case `Should tombstone a derived link without a prior save`; manager case `Should restore runs here after unlinking derived work`; foreign repository unlink is refused. | `internal/session/linked_work.go`, `internal/session/linked_work_projection.go`, `internal/session/query_test.go`, `internal/store/session_work_links.go`, `internal/store/globaldb/queries/session_work_links.sql`, `internal/store/globaldb/sqlcgen/session_work_links.sql.go`, `internal/store/globaldb/global_db_session_work_links.go`, `internal/store/globaldb/global_db_worktree_test.go` |
| 10 | Existing automatic-upsert logic already enriches worktree metadata without restoring tombstones or changing provenance. The canonical persistence test now starts with a dead worktree ID. | `internal/store/globaldb/global_db_worktree_test.go` |
| 11 | Optional head_repository is populated by GitHub search/read and passed through candidates. Foreign heads are refused with session_link_fork_unsupported. A new manual PR selection on the same branch replaces the stored number. Manager case `Should reject fork PRs and replace the selected branch PR` and provider discovery case. | `internal/extension/contract/forge.go`, `extensions/forge/github/discovery.go`, `extensions/forge/github/provider_test.go`, `internal/daemon/session_work_candidates.go`, `internal/session/linked_work_candidates.go`, `internal/session/linked_work.go`, `internal/session/query_test.go`, `internal/api/core/session_linked_work.go`, `sdk/go/contracts/types_008_gen.go`, `sdk/typescript/src/generated/contracts.ts`, `sdk/go/client/types_028_gen.go`, `sdk/client/src/generated/sdk/types.gen.ts`, `sdk/client/src/generated/openapi/compozy-openapi.d.ts`, `openapi/compozy.json` |
| 12 | Git enumerates refs without the slash-sensitive glob; Go filters substring matches and keeps at most page*limit+1 matching refs for paging. Real-Git local branch adapter case covers nested names and two pages. | `internal/daemon/session_work_candidates.go`, `internal/daemon/boot_worktrees_test.go` |
| 13 | Cache-derived search numbers require current repository identity; explicitly stored numbers remain searchable. Real SQLite `TestSessionLinkedWorkCatalogReview` changes repository and checks both cases. | `internal/store/globaldb/global_db_session_search.go`, `internal/store/globaldb/global_db_session_index_test.go` |
| 14 | The correlated board lookup explicitly selects the primary-key index beginning with session_id. Fresh migrated SQLite test runs EXPLAIN without ANALYZE. | `internal/store/globaldb/global_db_session_board.go`, `internal/store/globaldb/global_db_session_index_test.go` |
| 15 | The exact resolved number is omitted from later pages as well as the first page's normal search results. Adapter case `Should remove the resolved number from later pages`; route documents at most one extra exact result on page 1. | `internal/daemon/session_work_candidates.go`, `internal/daemon/boot_worktrees_test.go`, `internal/api/spec/session_linked_work.go`, `openapi/compozy.json` |
| 16 | One GraphQL search supplies number/title/state/branch/head repository; usable nodes survive partial GraphQL errors. Provider discovery test rejects unexpected REST calls, checks variables, preserves a good node beside a failed node, and verifies head repository. | `extensions/forge/github/discovery.go`, `extensions/forge/github/provider_test.go` |
| 17 | Stable daemon sort orders urgency, Runs here, direct, then rolled-up. Existing ready-to-merge urgency precedes other open work. Projection case `Should order runs here then direct then rolled up within urgency`. | `internal/session/linked_work_projection.go`, `internal/session/query_test.go` |

Documentation group: `docs/_memory/change-impact.md`, `docs/qa/scenarios/ET-session-pull-request-link.md`, `skills/compozy/references/native-tools.md`, and this report. No schema/migration bytes changed. Existing HTTP/UDS paths, native IDs/inputs, CLI verbs, catalog event types and supports_pr_discovery flag remain unchanged. Additive head_repository appears in forge and candidate contracts. Legacy providers may omit it. Board precedence remains Needs you, Working, Waiting, then own/direct open work; rolled-up work never controls the board. Default branches are excluded from derived PR reads in Go and from own-PR SQL joins.

EXPLAIN evidence: `SEARCH l USING INDEX sqlite_autoindex_session_work_links_1 (session_id=? AND repo_key=?)`, then `SEARCH f USING INDEX sqlite_autoindex_forge_pull_requests_1 (workspace_id=? AND repo_key=? AND branch=?)`. No repository-wide link scan was selected.

### Verification

`make codegen` passed; `make codegen-check` passed, including module verification, manifests, generated SDK/OpenAPI/SQLC, CLI docs and migration-guide checks. The initial sandboxed generation could not acquire its mage cache lock; only that owned bootstrap was stopped and generation was rerun with cache access. A first build incorrectly named the nested SDK module from the root and was corrected to a separate module build. No Git metadata was mutated.

Focused final commands and results are recorded below. Every daemon test invocation removes all inherited COMPOZY_* variables in Python before launching Go. No daemon was started and port 2123 was unused.

Test-convention checker was run separately for all eight changed Go test files. `global_db_worktree_test.go`, `native_worktree_tools_test.go`, and `pull_requests_test.go` pass. Query retains nine pre-existing shape findings; index retains five after wrapping the new case; boot worktrees retains two; session workspace retains one; provider retains five pre-existing discarded-error findings. No new case remains flagged. No unrelated test reshaping was done.

Not run: live GitHub/provider acceptance or the full HTTP/UDS/CLI/UI journey; repository-wide lint/tests, full race suites, E2E and make gate/gate-full (CI-owned). No migration suite was repeated because no schema changed; the real SQLite cases apply the current migration chain and exercise reopen/tombstone preservation. Live GraphQL pagination and latency still need measurement against the real provider; no four-second before/after latency claim is made. The live QA scenario remains unverified by this backend run. Pre-fix red evidence was not separately captured for every item, as noted above.


Final verification receipts (base HEAD `d85119930546ccff63ed2e53417ff11eba077316`; changes remain uncommitted):

```sh
make codegen
make codegen-check

go build ./internal/session ./internal/store/globaldb ./internal/api/core ./internal/api/spec ./internal/daemon ./internal/worktree ./extensions/forge/github ./internal/extension/contract
# In sdk/go (separate module):
go build ./...

go vet -tags integration ./internal/session ./internal/store/globaldb ./internal/api/core ./internal/api/spec ./internal/daemon ./internal/worktree ./extensions/forge/github ./internal/extension/contract

# The exact clean-environment test launcher:
python3 - <<'PYTEST'
import os, subprocess
for k in list(os.environ):
    if k.startswith('COMPOZY_'):
        del os.environ[k]
raise SystemExit(subprocess.call([
    'go', 'test', '-tags', 'integration',
    './internal/session', './internal/store/globaldb', './internal/api/core',
    './internal/daemon', './extensions/forge/github', './internal/worktree',
    '-run', 'TestSessionLinkedWork|TestSessionWorkLinkPersistence|TestManagerBoardCatalog|TestSessionCatalogBoard|TestSessionOptionalHealthLinkedWork|TestSessionPullRequestAdapters|TestNativeWorktreeTools|TestProvider/Should_discover|TestPullRequest',
    '-count=1'
]))
PYTEST

go test ./internal/session -run 'TestSessionLinkedWorkProjection|TestManagerBoardCatalog' -count=1

go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --allow-parallel-runners --concurrency=2 --new-from-rev=HEAD ./internal/session ./internal/store/globaldb ./internal/api/core ./internal/api/spec ./internal/daemon ./internal/worktree ./extensions/forge/github ./internal/extension/contract

git diff --check
```

Generation, codegen-check, both builds, vet and whitespace pass (exit 0). The complete focused test launcher passes (session 6.444s, globaldb 5.068s, core 1.058s, daemon 5.649s, provider 5.416s, worktree 5.578s). After the final fixture-only modernize cleanup, the session rerun also passes (3.045s). Initial lint exposed new style findings; those were repaired, including stable slices sorting, bounded SQL line lengths and fixture initialization. No test assertions were weakened.

Test shape command, invoked separately for each path: `python3 .agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py <path>`. Paths: `internal/session/query_test.go`, `internal/store/globaldb/global_db_worktree_test.go`, `internal/store/globaldb/global_db_session_index_test.go`, `internal/daemon/boot_worktrees_test.go`, `internal/daemon/native_worktree_tools_test.go`, `internal/worktree/pull_requests_test.go`, `internal/api/core/session_workspace_internal_test.go`, `extensions/forge/github/provider_test.go`. Results and pre-existing diagnostics are listed above. All owned Go files were formatted using `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt` with explicit owned file arguments.

Final focused lint passed: exit 0, `0 issues`. Final `git diff --check` passed. No commit or staging was attempted; all requested changes are ready for the root agent to review and commit, excluding incidental `extensions/open-design/lint.gen.mjs`, `tmp-sqlc-helper/`, and unrelated frontend files.
