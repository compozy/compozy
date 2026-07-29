---
id: ET-web-session-deep-link-isolation
area: ET
title: Never move a session deep link across workspaces without confirmation
persona: Ada
journey: J-operate-workspace-context
expected: A deep link to a foreign-workspace session never changes the active workspace and never renders foreign session data before the operator confirms; cancelling keeps the active workspace, its arrangement, and the existing not-found state, and a session that exists nowhere stays not found with no confirmation offered.
entry_points: /agents/:agent/sessions/:session; /session/:session
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-session-cross-workspace-confirm; ET-native-workspace-scope-isolation; MS-workspace-resolution-chain
---

This file owns the negative control: what must not happen before confirmation. The positive switch
journey belongs to `ET-web-session-cross-workspace-confirm`.

Create sessions in two registered workspaces. Keep workspace A active, arrange a visible window, then
open the canonical and short permalink forms for a session owned by workspace B. Confirm that neither
path changes the active workspace on its own, that A's arrangement survives, and that no title,
transcript, metadata, or window from the foreign session renders while the confirmation is pending —
only the owning workspace's identity may be resolved. Cancel the confirmation and confirm both paths
return to the not-found state inside workspace A.

Open a session id that exists in no workspace and confirm the not-found behavior is unchanged and no
confirmation appears.

Repeat with a session owned by workspace A and confirm both routes still open the session normally
with no confirmation.

QA impact 2026-07-28: Web session lookup and cache identity now require the active workspace instead
of discovering a session owner through the global session endpoint. Planning flag only; no QA
replay ran in this implementation slice.

QA impact 2026-07-28: failure to load the active workspace now remains a route error instead of
being rendered as `Session not found`. Retest both the foreign-session negative control and an
authoritative workspace-list failure. Status remains untested; no QA replay ran.

QA impact 2026-07-29: the unconditional foreign-session block is gone. A foreign deep link now
resolves a minimal owner projection and asks before switching (ADR-004), so the expectation changed
from "never changes workspace" to "never changes workspace or exposes foreign session data without
confirmation". Retest the pre-confirmation exposure surface and the cancel path. Status remains
untested; no QA replay ran in this documentation slice.
