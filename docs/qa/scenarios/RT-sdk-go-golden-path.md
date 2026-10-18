---
id: RT-sdk-go-golden-path
area: RT
title: Run an agent from the Go quickstart
persona: Ana
journey: J-build-external-app
expected: The documented Go client connects through UDS or explicit TCP and reads a real prompt reply and typed event frames for the same durable session.
entry_points: /docs/apps/go; sdk/go/client; local HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Follow the Go tutorial with the module version derived from the installed daemon. Record actual module resolution and transport. Resolve a workspace, create a session, iterate text chunks to EOF, then subscribe to transcript events. Repeat with an explicit TCP base URL and compare session identity and response data. Exercise a queued result through Accepted(), context cancellation while reading, and a typed 4xx error. A local replace is valid development evidence but not evidence of published module availability.

QA impact 2026-10-17: App SDK implementation adds this user-visible journey. Authored planning only;
no live execution verdict is recorded. Execute in the assigned isolated lab and attach current-head
evidence before marking it passed.

QA impact 2026-10-18: HTTP-only operation examples explicitly select the daemon HTTP URL.
For operator tool invocation, preserve both the completed 200 payload and the approval-required
202 payload, including the approval token; inspect StatusCode before decoding the matching variant.
The canonical Go client suite uses real HTTP responses for both outcomes.
