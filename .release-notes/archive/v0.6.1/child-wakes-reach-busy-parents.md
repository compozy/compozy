---
title: Child replies reach a busy parent session during its turn
type: fix
---

A parent session busy with a long turn now receives replies and needs-attention wakes from its child sessions during that turn. Before, monitor wakes were delivered while child replies and attention wakes waited until the turn ended. Each reply keeps its own identity, and a needs-attention wake is dropped when the child has moved on or a session monitor already covers that state. When a pending permission or question changes, the parent gets a fresh wake, so the current state is never lost.

A child that answers a follow-up from the session that created it is no longer flagged as stopped before delivering just because it opened no pull request.

Migration notes: no action needed.
