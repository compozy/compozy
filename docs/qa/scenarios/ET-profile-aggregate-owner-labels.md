---
id: ET-profile-aggregate-owner-labels
area: ET
title: Label every owner in explicit aggregate reads
persona: Ada
journey: J-operate-profiles
expected: Explicit aggregate reads return work across profiles with profile_name on every row, JSONL begins with profile_resolution even for an empty result, and --profile conflicts with --all-profiles.
entry_points: compozy --all-profiles; -o json|jsonl; HTTP/UDS all_profiles=true
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-scoped-work-reads; ET-profile-deep-link-owner
---

Flagged by Profiles task 06. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. List work owned by at least two profiles with `--all-profiles` and verify every row identifies its
   `profile_name`.
2. Repeat through HTTP and UDS with `all_profiles=true` and compare owners and record sets.
3. Produce an empty aggregate JSONL result and retain its leading `profile_resolution` frame.
4. Combine `--profile` and `--all-profiles` and verify the structured conflict response.

Expected evidence: CLI JSON and JSONL captures, HTTP and UDS payloads, owner comparisons, and the option
conflict response.
