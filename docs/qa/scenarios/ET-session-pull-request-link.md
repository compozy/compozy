---
id: ET-session-pull-request-link
area: ET
title: Session branches share canonical pull request observations
persona: Rafa
journey: J-14
expected: HTTP, UDS, native session reads and CLI agree on branch identity, cached PR facts and lookup failures without using display facts as cleanup authority.
entry_points: compozy session pr; session detail; session list; pull-request refresh endpoint
qa_status: untested
retest_status: pending
---

1. Settle a root-session turn on a topic branch. Change the checkout to another branch and read the
   original session again: its branch remains the settled snapshot. Settle on the default branch or
   detached HEAD: the link disappears. A managed session uses its pending/ready worktree branch;
   removing that worktree clears its link without using a root snapshot.
2. Create two sessions on the same branch. Compare `compozy session pr <id> -o json`, HTTP/UDS detail
   and native status/list. They share the same PR, head SHA, checks and lookup observation. A fork's
   identically named branch must not win selection. Changing origin must not display the old PR.
3. Run `compozy session pr <id> --refresh -o json`. Confirm complete check counts beyond the first
   20 nodes, at most five failed names, and Ready only with current, known qualifying facts. A
   provider outage retains facts with an error lookup. Without prior facts it returns 502
   `forge_unavailable`; no branch returns 409 and an unknown session returns 404.
4. Repeat a provider failure: automatic retries follow backoff and identical failures do not cause
   catalog wake loops. An explicit refresh joins an active refresh and ignores backoff. An incomplete
   100-candidate search is distinguished from a confirmed absence.
5. Squash-merge a PR and remove its remote branch. Cleanup must still prove the current local HEAD;
   a later unpublished commit or a mismatched provider SHA blocks removal even with a cached merged PR.

Automated slice evidence: `TestSessionBranchSnapshots`, `TestSessionPullRequestAdapters`,
`TestBaseHandlersRefreshSessionPullRequest`, `TestPullRequestService`, `TestProviderGraphQL`, and the
real-Git `TestWorktreeLifecycleIntegration/Should_clean_up_a_gone_upstream_only_with_current_merged_HEAD_evidence`.
The controller's final QA walk owns live-provider and UI verification; these package checks do not
claim that walk passed.

Review regression checks:

- Run cached CLI reads on a default-branch, detached-HEAD, or never-settled session in human and
  JSON formats: exit 1 with `session_branch_unavailable`. A linked-session explicit refresh uses
  the canonical by-id route and returns the refreshed observation.
- Open managed session topbars while the display cache is stale or rate-limited. Exit-plan reads
  perform no inline forge capability/status requests. Cold plans remain provider-free. On a clean, pushed managed branch after restart, select the
  PR action: preparation resolves native capabilities before opening the dialog. Confirm title/body
  prefill and draft/create actions appear without an unrelated commit or push; no browser fallback
  opens while preparation is pending. Previously observed capabilities supply presentation defaults. Explicit actions revalidate
  capabilities, and removal still obtains fresh evidence against the current HEAD.
- Occupy all four display-refresh slots, queue an automatic refresh under backoff, then join it
  explicitly: exactly one provider call runs for that key after admission. With a backlog of other
  automatic keys, a new explicit key and a promoted automatic key each take the next free slot.
  Repeated list pages and facet reads within 60 seconds discover historical keys only once per
  normalized query, including branches outside the selected page and board column.
- Return HTTP 200 with GraphQL `RATE_LIMITED` and `X-RateLimit-Reset`: retain the reset timestamp.
  Disable the forge extension: retain prior facts and report `provider_disabled`.
- Change cached PR facts while two sessions sharing a branch are open. The workspace catalog wake
  refreshes detail and by-id reads as well as lists; unrelated workspaces stay unchanged. With a
  detail read already in flight, stop a session and append its discard marker, then deliver the
  session upsert followed by a PR layout wake before the old read settles. The mounted transcript
  must receive the marker after the trailing detail read, without remount or polling. A layout wake
  alone must not refresh a stopped transcript.
- Force the supported REST fallback. Prefer matching origin-repository open PRs, then terminal
  PRs, by newest update. Missing identity/update evidence or an exhausted candidate bound without
  a match produces `incomplete`, not a confirmed absence.

Focused review evidence: real-Git exit-plan/removal integration, CLI HTTP-client routing,
`TestSessionPullRequestCommand`, `TestPullRequestService`, `TestProvider`, `TestProviderGraphQL`,
`TestManagerForgeProvider`, `TestDaemonForgeRuntimeErrorTranslation`, `TestSessionCatalogBroadcaster`,
and the catalog-stream Vitest suite. Live GitHub and full desktop walks remain controller-owned.

- Detail enrichment projects only direct active children and preserves the scoped parent title.
  A relation-reader or PR-projection failure leaves the core detail and admission queue available;
  optional family/PR facts are omitted. Owning checks: `TestManagerBoardCatalog` and
  `TestSessionPayloadOptionalRelations`.

Session linked work extension (live walk: docs/qa/reports/session-linked-work/live-walk.md):

- Create a worktree through a session's native tool: it appears in `GET .../sessions/{id}/links` without binding. Soft unlink it, refresh, then manually relink; it stays removed until the manual action.
- Link a branch or PR with `session link <id> <branch> [--pr-number N] -o json`; read it through HTTP and UDS. Reject another repository with `session_link_repository_mismatch`.
- Remove the worktree: linked work retains its name, branch and PR identity and sets `worktree_removed`.
- Link the same PR to parent and child: parent shows one item, direct wins. Archived children still contribute; child-only work never changes the parent's column.
- Keep an unrelated open PR linked while running several turns. Working and Needs you retain precedence; queued and dispatching inputs keep Idle/Done sessions in Working only with open direct work (including their own branch PR). Stopped ignores queued input and goes to In review with open direct work; sessions without open direct work retain their existing column. An unknown session moves only under the legacy own-branch rule.

Root-review regression checks (see the live walk report for what ran live):

- Mixed work lists count only fresh failing checks, requested changes or conflicts as Needs you. Ready comes next, then other open/branch items, unknown lookups and terminal PRs. Stale facts never trigger a decision; the lead follows this order.
- Compare summary `total` and `pull_requests` with the full deduplicated list, including a known PR whose lookup failed.
- Search a linked branch, PR number and #number with a one-item catalog page. Active and durable sessions return the same exact total and cursor results. Unlinked and child-only identities do not make a parent match; another workspace/profile stays excluded.
- Search a known PR by number, #number and a same-repository forge URL while text search returns no matches: the first candidate must contain the requested number, branch and title. Normal search results remain after it, with the resolved PR deduplicated. A confirmed missing number stays absent; another repository is refused before provider calls. Repeat text search and page 2 to confirm normal discovery still works. Automated owner: `TestSessionPullRequestAdapters`; walked live against the real forge.
- An app token may read linked work under existing session-read scope, but add, remove and candidate search return `app_token_route_forbidden`.

Review-round backend checks (automated evidence is in `docs/qa/reports/session-linked-work/backend.md`; repeat the live walk separately):

- Cancel a worktree-create request after acceptance; the tool still returns the accepted worktree if linking fails. In a repository with no origin or a local-path origin, automatic and manual branch links work without contacting a forge. Hosted subgroup repositories retain their hosted identity.
- Forks and continuations never roll work into their source session. A child branch without a PR contributes neither a row nor a summary count. Within each urgency band, Runs here precedes direct work, which precedes rolled-up work.
- Compare detail child counts, optional-health detail, active board and durable board. Bound default branches stay branch-only and do not refresh forge status. Fenced queued input with dispatchable=0 does not hold a card in Working.
- Unlink a derived bound worktree, then Undo: Runs here, worktree name and provenance return. Automatic creation replaces a dead worktree ID on an existing link without clearing an unlink tombstone.
- Reject PRs whose head repository differs with `session_link_fork_unsupported`; providers omitting head repository retain existing behavior. Selecting another PR on the same branch replaces the stored PR number because status follows the branch.
- Search a nested branch substring across page boundaries. An exact PR appears once, on page 1, which may have one extra item. After changing the origin, cached numbers from the old repository do not match; explicitly stored numbers remain searchable.
- Inspect provider search traffic: one GraphQL query supplies number, title, state, branch and head repository for a page; usable nodes remain when another node fails.

Owning automated checks: `TestBoardColumnWithLinkedWork`, `TestSessionCatalogBoard`,
`TestSessionLinkedWorkProjection`, `TestManagerBoardCatalog`,
`TestSessionLinkedWorkSummary`, and `TestAppAccessPolicy`. These exercise backend behavior,
including real SQLite; they do not claim the live UI or provider walk passed.

Session linked work web walk (live walk report lists the steps not yet walked):

1. A session with no links offers "Link a pull request or branch…" in the composer environment menu and through the palette. Neither "Use worktree…" nor "Fork into a worktree…" changes.
2. Let the agent create a worktree mid-turn and open a PR, then add a second through a child session. With no reload and no new turn, the head reads `2 linked PRs` in place of a single PR chip and opens the list, and the child's row reads "From {child title}", opens the child, and has no Unlink. A child's branch with no PR stays off the parent.
3. Link a PR by pasting its URL, by number, and by ticking two search results. A link from another project reads "#N belongs to owner/name", also on a session with nothing linked, where the daemon's `session_link_repository_mismatch` decides it and the dialog must not say the forge isn't connected; an unknown number reads "Can't find #N". Tick a PR from a fork with another PR: the other links, and the dialog stays open with "#N comes from a fork" and "Pull requests from forks can't be linked yet.", with the fork no longer selected. With the forge provider disabled, the dialog says the forge isn't connected and a branch can still be linked. In a repository with no hosted remote, branches link the same way and no `local:` identity shows in the dialog.
4. Unlink from a row: the row leaves at once, the toast says nothing was deleted, the PR stays on the forge and the files stay on disk. Undo restores it. Unlink an automatic item and refresh: it stays gone.
5. Remove a linked worktree: its row reads "Worktree removed" and the PR stays. Disconnect the forge: rows read "PR status unknown" with the cause, never "No PR".
6. Narrow the window until the head clips the status zone: the list stays reachable from the actions side. Hold 50 items: the list groups, folds Done, filters, and stays usable.
7. Session list rows show a count and a dot when an item needs you, and Kanban cards show the lead PR plus `+N`, with no per-row read in the network log. Find a session in the palette by PR number and by branch.
8. Link an unrelated open PR to a working session and watch its card stay in Working through several turns.

Owning automated checks: `lib/__tests__/pull-request.test.ts` (ordering, grouping, labels, input parsing), `use-session-topbar-slot.test.tsx` (head, rolled-up rows, narrow entry), `use-session-linked-work.test.tsx` (unlink with Undo, server search, dialog states), `use-session-catalog-streams.test.tsx` (refresh with no new turn), `palette-session-filters.test.ts` (server hits stay), and `E2E-006` in CI. Proto states: `session-links-header`, `session-links-surfaces`. Live forge and provider legs remain owned by the controller's QA walk.

PR #1161 backend review regressions:

- Initialize a forge provider exposing only capabilities, status and PR creation. It loads without PR discovery; branch linking remains available. Automated owner: `TestInitializeLegacyForgeProvider`.
- Manually link a branch, then create its worktree through the calling session. The existing link gains the worktree ID/name and retains user provenance. Repeat after unlink: the tombstone remains. Automated owner: `TestSessionWorkLinkPersistence` (real SQLite).
- Link and unlink a child's work while parent and grandparent catalogs are subscribed. Both receive scoped upserts immediately. Foreign workspace/profile ancestry receives none; a cycle cannot repeat publication. Automated owner: `TestSessionCatalogBroadcaster/Should wake every scoped ancestor on link and unlink`.

These regressions pass in focused backend suites. The browser/provider walk is not repeated by this backend remediation task.

Statistics-free family traversal regression:

- Use a temporary migrated database with 840 sessions in 120 parent/child trees and no
  `sqlite_stat1`. Recreate an existing profile/workspace catalog index with its unchanged
  definition to exercise upgrade-dependent index ordering. The recursive family step must
  seek `idx_sessions_parent`, return all seven members per root, and preserve scope/lineage
  fences. Compare five samples of `BenchmarkSessionWorkFamily` before and after the fix.
- The active catalog continues to send all roots in one family batch. Verify the existing
  `TestSessionLinkedWorkProjection` batch case and `TestManagerBoardCatalog` real-SQLite cases.
  Session list, facets and attention consumers retain their response shapes and board rules.
- Verify `TestStoreSQLiteHelpers`: background maintenance creates missing index statistics,
  yields to an occupied writer, honors cancellation, and keeps checkpoint/shutdown behavior.
  These are storage/read-model checks; they do not claim a browser or live-provider walk.
