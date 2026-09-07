---
title: Keep sessions alive while observable work is still running
type: feature
---

Session supervision now considers agent progress, running tools, active children, Loop runs, task leases, and scheduled waits. A quiet transcript alone no longer means the session is idle. If a work-signal source cannot answer, supervision reports attention and suspends automatic stopping.

Observed silence warns after 30 minutes by default and enters the normal stop ladder after another 10 minutes; fresh work cancels the pending stop. Network Live participation has no aggregate wall-time limit by default, while explicitly configured and other budgets remain effective.

Capacity starvation now escalates visibly. Blocked automation fires defer durably and resume after restart, and scheduler counters distinguish skipped wakes from successful work.

PR: [#557](https://github.com/compozy/compozy/pull/557).
