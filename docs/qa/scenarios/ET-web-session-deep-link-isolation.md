---
id: ET-web-session-deep-link-isolation
area: ET
title: Keep session deep links inside the active workspace
persona: Ada
journey: J-operate-workspace-context
expected: A session deep link resolves only inside the active workspace; a foreign canonical link appears not found and redirects to its agent page, while a foreign short permalink remains a non-rendering not-found fallback; neither path changes workspace or exposes foreign session data.
entry_points: /agents/:agent/sessions/:session; /session/:session
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-native-workspace-scope-isolation; MS-workspace-resolution-chain
---

Create sessions in two registered workspaces. Keep workspace A active, arrange a visible window,
then open the canonical and short permalink forms for a session owned by workspace B. Confirm both
paths show `Session not found`, remain in workspace A, preserve A's arrangement, and render no
title, transcript, metadata, or window from the foreign session. The canonical route redirects to
its agent page; the short permalink remains on its inert fallback because the foreign agent name is
intentionally unavailable.

Repeat with a session owned by workspace A and confirm both routes still open the session normally.

QA impact 2026-07-28: Web session lookup and cache identity now require the active workspace instead
of discovering a session owner through the global session endpoint. Planning flag only; no QA
replay ran in this implementation slice.
