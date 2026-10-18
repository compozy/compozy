---
id: RT-sdk-ts-golden-path
area: RT
title: Run an agent from the TypeScript quickstart
persona: Ana
journey: J-build-external-app
expected: The documented Node script connects, resolves its workspace, creates a session, streams a real reply, and a fresh client reads the same durable transcript.
entry_points: /docs/apps; @compozy/client; local daemon
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Follow every command and code block in the Node quickstart against an isolated running daemon. Record the package version/source, daemon version, resolved workspace, session ID, ordered text chunks, and final transcript. Exercise an empty task list and a nonempty in-progress list through the generated resource method. Confirm the final result is about the requested workspace, not merely a successful connection. Queue a second prompt while the first runs and observe accepted status, delivery, and queue position without attempting to iterate chunks. Compare a fresh session read after the script exits.

QA impact 2026-10-17: App SDK implementation adds this user-visible journey. Authored planning only;
no live execution verdict is recorded. Execute in the assigned isolated lab and attach current-head
evidence before marking it passed.
