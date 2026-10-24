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
