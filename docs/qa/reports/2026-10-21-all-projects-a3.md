# All-projects A3 backend evidence

Scope: loop runs, human requests, definition catalog, their CLI reads, and the Loop leg of the
operator notification bell. Branch: `ap/a3`. This is slice evidence, not an integrated Web QA verdict.

## Contract coverage

- BR-2/14; T-01/04/06–09/16/17/29: three new app-denied operator HTTP/UDS routes require
  `all_workspaces=true`. Registered projects plus User are selected within the profile lens.
  Runs use `(created_at DESC, id DESC)`; workspace operational ordering stays available.
  Cursors bind profile, breadth, filters, and order. `origin=session` retains the existing Goal filter.
- BR-14/15; T-30/31: requests preserve expiry/opened-at/rowid order, distinguish tied generations/items,
  carry workspace ownership, and join the run's profile before count/page. Cursor reuse across breadth
  or profiles is rejected. Pending and resolved lifecycle suites remain passing.
- BR-15; T-10/32: definitions return `{entries, errors, truncated}` with `source.layer`, up to
  200 per workspace and 2,000 total. User definitions resolve independently of project overrides;
  project-inherited rows require recent project runs. Summary ownership is exact, including zero projects.
  A broken project's definition does not blank healthy projects.
- BR-2/3: the bell issues aggregate requests/node-inventory reads under explicit all-profiles scope,
  de-duplicates request-backed waits using each row's workspace, and labels empty ownership `User`.
- CLI list/runs/requests accept mutually exclusive `--all-workspaces` and `--workspace`. Human tables
  identify projects using the workspace catalog; JSON keeps ownership and aggregate error metadata.
  Native list tools retain ordinary workspace pinning.

## Schema and query plans

`make codegen` generated `internal/store/globaldb/schema/migrations/00146_schema.sql` and refreshed
`atlas.sum` and sqlc. No hand-written data migration or existing migration edit. The three additions are:

- `idx_loop_runs_profile_created(profile_id, created_at DESC, id DESC)`;
- `idx_loop_runs_created(created_at DESC, id DESC)` for the all-profiles recency walk;
- `idx_loop_requests_state_expiry(state, expires_at, opened_at)`.

`TestGlobalDBLoopAggregateQueryPlans` ran `EXPLAIN QUERY PLAN` against the real migrated SQLite schema:

| Read | Observed plan |
| --- | --- |
| Runs, one profile | `SEARCH loop_runs USING INDEX idx_loop_runs_profile_created (profile_id=?)` |
| Runs, all profiles | `SCAN loop_runs USING INDEX idx_loop_runs_created` in recency order; no temporary sort |
| Requests, one profile | `SEARCH request USING INDEX idx_loop_requests_state_expiry (state=?)`; indexed run ID join; profile predicate before page |
| Requests, all profiles | Same request index; covering run ID join |

All reads check registered workspace membership through the workspace primary-key index. Requests
still use a temporary B-tree for the specified null-expiry-first expression ordering; the new index
reduces the candidate scan by state without changing that order. The runs' registered-workspace
predicate uses a correlated existence check to preserve the recency index walk rather than an OR
union followed by a temporary sort.

## Verification

Passed targeted Go checks on the changed source:

- `internal/store/globaldb`: `TestGlobalDBLoopAggregate*`, existing workspace run/origin filtering,
  operational pagination, User isolation, request lifecycle, and node-inventory suites. Real SQLite.
- `internal/daemon`: `TestDaemonLoopCatalog*` and `TestLoopRunList*`; attribution, per-project errors,
  bounds, zero-project User definitions, profile/breadth/order cursor rejection, and 500-row lookahead.
- `internal/api/core`: `TestLoopAggregateHandlers`, `TestBaseHandlersAttentionOperatorScope`, and
  `TestAttentionNotification*`.
- `internal/api/httpapi`, `internal/api/udsapi`: `TestRegisterRoutesCoversTechSpecEndpoints`;
  HTTP `TestAppTokenLoopAggregateAdmission` uses the actual app-access middleware.
- `internal/api/spec`: `TestAppAccessPolicy`.
- `internal/cli`: `TestLoopAllWorkspacesCommands`, existing command/profile/catalog suites, and
  `TestUnixSocketClientLoopCatalog`.
- `internal/store`: `TestProductionMigrationStreams*`, `TestApplyMigrationStream`, and
  `TestApplyRejectsAtlasSumDrift` (fresh/reopen/ahead/integrity coverage).
- `internal/loop`: `TestCoordinatorRunnerShouldParkAskRequests`.
- `go build ./...`; `go vet` on core/spec/cli/daemon/loop/globaldb; changed-test convention checks;
  `git diff --check`.

`make codegen` and `make codegen-check` passed. OpenAPI, TypeScript/Go SDKs, and CLI reference pages
co-ship. The generator also changed only a local minified variable (`css2` to `css`) in the bundled
OpenDesign linter under the installed Bun version; this is generator output with no source behavior change.
`bunx turbo run typecheck --filter=@compozy/client` passed (2/2 tasks, including its codegen-check prerequisite).

No `make gate`, `make gate-full`, full `go test ./...`, E2E, or local race suite was run: the brief
assigns heavy gates to CI and limits race runs to concurrency changes. This slice introduces no
concurrency change. No push or PR publication is part of this worker assignment.

## Integration notes

- U5 replaces `requireOperatorSurface` with the granted-agent guard. No unrestricted grant is simulated.
- A4 owns terminal aggregate reads; the bell's terminal fan-out remains unchanged.
- UW owns aggregate consumers and the “Definitions for {profile}” toolbar. Following the explicit DX
  route contract, `/api/loops` rejects `all_profiles`; the UI selects the acting concrete profile for
  definitions even while other lists display all profiles.
- Existing filter names are preserved (`status`, `run_id`); no aliases for the prose-only `state`/`run`
  variants were introduced for run/request filters.
- New attribution fields may be omitted for User, as allowed by the DX contract, preserving additive
  generated consumer compatibility. Project items always carry their workspace ID.
- The old workspace catalog retains its string `source`. Only the new aggregate uses `source.layer`.
- The integrated browser re-walk in `LP-web-attention-loop-rows` remains pending, explicitly marked
  in the scenario. Backend and real middleware checks do not claim browser evidence.
