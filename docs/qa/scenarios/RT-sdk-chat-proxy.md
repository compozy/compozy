---
id: RT-sdk-chat-proxy
area: RT
title: Render agent replies through a server chat route
persona: Ana
journey: J-build-external-app
expected: The browser chat renders the daemon UI message stream through the app server with preserved status and stream header, and disconnecting the page leaves the admitted turn running.
entry_points: /docs/apps/chat-ui; promptResponse; AI SDK useChat
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-sdk-ts-golden-path
---

Run the documented server route and chat page against a real configured session. Capture the browser request to the app server and the response header x-vercel-ai-ui-message-stream: v1; verify incremental text renders. Confirm no daemon credential appears in the browser bundle or request. Trigger accepted work and a daemon error and verify their 202 JSON and original error status/body are not parsed as a UI stream. Close the browser during a running turn, then read its completed transcript through a fresh client.

QA impact 2026-10-17: App SDK implementation adds this user-visible journey. Authored planning only;
no live execution verdict is recorded. Execute in the assigned isolated lab and attach current-head
evidence before marking it passed.
