---
id: RT-session-cwd-resume
area: RT
title: Preserve a session working directory across sandbox launch and resume
persona: Théo
journey: J-11
expected: A session created with a valid working directory below its workspace launches in the corresponding sandbox runtime directory, persists that choice, and resumes in the same directory without escaping the workspace boundary.
entry_points: session create CWD; daemon session reactivation; provider process launch
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-context-rebuild
---

Create a nested workspace directory with a file whose relative path is unique. Start a sandboxed
session with that directory as its CWD and confirm the provider observes the mapped runtime path.
Stop and reactivate the session, then confirm the provider starts in the same nested directory and
can still address the file relative to its CWD. An outside-workspace CWD must remain rejected.

QA impact 2026-07-15: the integration lane exposed that sandbox launch replaced an explicit nested
CWD with the workspace root and resume did not persist it when no creation store was configured.
Production now maps and persists the validated session CWD. Planning flag only; no QA replay ran in
this implementation slice.
