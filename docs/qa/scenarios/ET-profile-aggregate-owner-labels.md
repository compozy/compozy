---
id: ET-profile-aggregate-owner-labels
area: ET
title: Label every owner in explicit aggregate reads
persona: Ada
journey: J-scope-work-by-profile
expected: Explicit aggregate reads return work across profiles with profile_name on every row; workspace-scoped JSONL begins with workspace_resolution containing the workspace and source, global JSONL begins with profile_resolution for profile=all, and --profile conflicts with --all-profiles.
entry_points: compozy --workspace <root> --all-profiles; compozy --all-profiles; -o json|jsonl; HTTP/UDS all_profiles=true
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-profile-scoped-work-reads; ET-profile-deep-link-owner
---

Flagged by Profiles task 06. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. List work owned by at least two profiles with `--all-profiles` and verify every row identifies its
   `profile_name`.
2. Repeat through HTTP and UDS with `all_profiles=true` and compare owners and record sets.
3. Produce an empty workspace aggregate JSONL result and retain its leading `workspace_resolution`
   frame with workspace and source; repeat without a workspace and retain `profile_resolution` with
   `profile=all`.
4. Combine `--profile` and `--all-profiles` and verify the structured conflict response.

Expected evidence: CLI JSON and JSONL captures, HTTP and UDS payloads, owner comparisons, and the option
conflict response.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.


Issue #832 extension verified on 2026-10-19: focused E2E-032 proves owner-grouped folders, cross-profile drop rejection, owner-bound rename and reload persistence. Real Codex QA proves runtime/agent metadata for root and child rows, including never-bound, idle-released and stopped/unbound after daemon restart. Ordering and collapsed-folder navigation are covered by the owning unit suites. Evidence and exact scope: [issue #832 report](../reports/2026-10-19-issue-832-session-list.md).
