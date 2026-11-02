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

Session catalog payload gains `linked_work: {total, needs_you, lead?}`. The full list uses the links resource. Existing branch, pull_request and pull_request_lookup fields retain their original own-branch meanings.

Native tools: `compozy__session_link`, `compozy__session_unlink`; mutating risk. Inputs: required branch, optional repository and positive pr_number; additional properties are refused. There is no session override. Scope supplies session/workspace identity. Agent link provenance is `agent`; HTTP/CLI is `user`; native worktree creation is `created`. An agent may unlink its own direct links.

CLI: `session link <id>` lists; `session link <id> [branch] --pr-number N` resolves/adds; `session link <id> <branch>` adds; `session unlink <id> <branch>` removes. Flags: `--repository`, `--pr-number`, and standard structured output including `-o json`.

Stable link error codes: `session_link_repository_mismatch`, `session_link_invalid`, `session_link_failed`. HTTP mutation failures return 400 with code/error. Native domain failures retain those codes. Other existing authorization/session lookup errors retain their transport behavior. Candidate provider failure returns 502. No new hooks or config keys.

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

Needs you wins first; pending queued or dispatching input means Working; existing Working and Waiting badge behavior wins over all PRs. Own-branch PR behavior is preserved, including Unknown with an own open PR. A direct linked open PR changes Done to In review only for Idle, Done or Stopped badges. Child-only work never contributes DirectOpen. Closed/unlinked direct work no longer contributes to that linked-PR predicate.

Active rows use BadgeForInfo and BoardColumnWithLinkedWork. Durable SQL independently derives the badge from durable fields and joins current repository cached facts plus direct unlinked=false links. It checks queue membership before the linked PR predicate. The new SQL test computes BadgeForInfo independently from runtime inputs rather than supplying SQL with a precomputed badge. The canonical board suite continues owning filtered counts/facets/pagination. Waiting, health and supervision remain runtime-only inputs routed through the active-row path; this change does not invent durable versions of them.

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

All three tasks succeeded (including the prerequisite codegen-check). The final standalone `make codegen-check` also exited 0. Final backend feature head: `5307d78ec2d9a669be0b2dccefdd6e36ab0c66ed`; the following commit only updates this report. The only remaining working-tree modification is the inherited `web/proto.config.ts`.
