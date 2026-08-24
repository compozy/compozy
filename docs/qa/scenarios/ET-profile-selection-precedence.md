---
id: ET-profile-selection-precedence
area: ET
title: Resolve profiles with stable precedence and immutable session binding
persona: Ada
journey: J-operate-profiles
expected: A command resolves --profile before COMPOZY_PROFILE, remembered workspace or Global selection, and default; archived remembered choices fall back with a note, sessions retain their creation profile, and daemon, doctor, and update ignore selection.
entry_points: root --profile; COMPOZY_PROFILE; compozy profile current|use|list; compozy session create|list; compozy daemon|doctor|update; compozy__profile_list|current
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-cli-lifecycle
---

Flagged by Profiles task 04. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. Set different Global and workspace remembered choices, then override them with
   `COMPOZY_PROFILE` and root `--profile`; verify `profile current -o json` source and workspace.
2. Archive the remembered profile and verify fallback to `default` with
   `archived_remembered_fallback`; explicit selection of the archived profile must fail.
3. Start sessions under two profiles from separate terminals, change remembered selection, and prove
   each session remains bound to its creation profile through CLI/API/native reads.
4. Produce an empty JSONL listing and retain the `profile_resolution` frame.
5. Set an invalid profile environment value and prove `daemon`, `doctor`, and `update` behave exactly
   as they do without profile selection.

Expected evidence: paired terminal transcripts, selection rows, session status payloads, native-tool
results, JSONL frames, and machine-command exit/output parity.
