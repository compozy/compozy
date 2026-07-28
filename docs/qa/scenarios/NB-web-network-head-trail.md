---
id: NB-web-network-head-trail
area: NB
title: Network window head drills only into conversations
persona: Théo
journey:
expected: With a channel selected, the network window head stays at root level — glyph, "Network", channel count, "Active · N live" status, and the create-channel action. Opening a thread or direct drills the head to back + Network / #channel crumbs with the conversation title (thread title or @peer) as leaf, swaps the status to the open-work chip when work is open, and suppresses the create action; back returns to the channel, crumbs navigate to their level.
entry_points: web network window (channel rail, thread open, direct open)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: NB-011
---

Introduced by the opendesign network redesign (docs/design/opendesign/network/network.html head contract, implemented 2026-07-21). Visual contract evidence: .compozy/tasks/os-shell/evidence/visual/opendesign-redesigns/VC-N1/.
