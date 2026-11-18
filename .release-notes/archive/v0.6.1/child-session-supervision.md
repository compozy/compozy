---
title: Supervising child sessions sends one accurate wake
type: fix
---

A parent session that watches a child with a session monitor now gets one wake when the child stops early, from that monitor. Before, it could also get a second early-stop wake that assumed the child should have opened a pull request and could arrive after the child had already moved on. The early-stop wake now asks the parent to review the child's answer, and is dropped if the child has started a newer turn.

Compact session records from native tools now include pending reply watches, so a parent can see what it is waiting on without requesting the full record.

A Claude subagent's answer is no longer lost or marked failed when the provider reuses its call ID in a later turn, and a background subagent's late summary is kept after its turn finishes.

Migration notes: no action needed.
