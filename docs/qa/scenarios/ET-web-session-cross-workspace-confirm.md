---
id: ET-web-session-cross-workspace-confirm
area: ET
title: Confirm and switch into the workspace that owns a linked session
persona: Ada
journey: J-operate-workspace-context
expected: A foreign-workspace session deep link shows a confirmation naming the owning workspace, and confirming activates that workspace and opens the session on both the canonical and short permalink routes; the confirmation state lives in the route so the link is replayable.
entry_points: /agents/:agent/sessions/:session; /session/:session; ?workspaceSwitch=confirm; ?workspaceSwitch=declined
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-session-deep-link-isolation; MS-workspace-resolution-chain
---

This file owns the positive switch journey. The pre-confirmation exposure and cancel behavior belong
to `ET-web-session-deep-link-isolation`.

With workspace A active, open the canonical deep link for a session owned by workspace B. Confirm the
dialog names workspace B by its registered name and describes the switch as changing the active
workspace and its open windows. Confirm the switch and verify the active workspace becomes B, the
session opens on the same route, and B's desktop arrangement is the one restored. Repeat on the short
permalink form.

Reload the URL carrying the routed confirmation state and confirm the confirmation replays instead of
resolving from stale client memory, and that the declined state does not re-open the dialog on its
own. Confirm no foreign workspace id or name can be injected through the URL: the owning workspace
comes from the resolved owner lookup, not from search parameters.

Operator UX only. Agent sessions never cross workspaces through web routing — that path is
`ET-workspace-access-mode-matrix`.

QA impact 2026-07-29: new behavior from the cross-workspace access program (ADR-004). Planning flag
only; no QA replay ran in this documentation slice.
