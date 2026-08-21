---
id: ET-profile-deep-link-owner
area: ET
title: Preserve profile ownership on aggregate deep links
persona: Ada
journey: J-scope-work-by-profile
expected: A direct read cannot expose foreign-profile work in scoped mode, while the explicit aggregate form returns the item with its profile_name owner.
entry_points: task get|inspect; task run show; automation job|trigger|run get; bridge get; network detail routes
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-scoped-work-reads; ET-profile-aggregate-owner-labels
---

Flagged by Profiles task 06. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. Capture identifiers for task, task-run, automation, bridge, and network records owned by a second
   profile.
2. Open each identifier from the first profile and verify it returns not found without disclosing owner
   data.
3. Repeat with explicit aggregate mode and verify the record appears with the correct `profile_name`.
4. Compare CLI, HTTP, and UDS behavior for one representative deep link.

Expected evidence: scoped and aggregate response pairs for every detail surface plus transport-parity
captures for the representative item.
