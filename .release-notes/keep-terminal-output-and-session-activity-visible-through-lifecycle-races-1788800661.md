---
title: Keep terminal output and session activity visible through lifecycle races
type: fix
---

A short-lived terminal that exits while the browser is attaching now keeps its exited state when an older catalog response arrives late. The exit bar and retained output remain discoverable. Terminal selection controls no longer resize the process and clear the selection, hidden panes no longer publish invalid dimensions, and a completed CLI detach no longer causes an unintended reconnect.

Session stream closure drains the persisted stop marker. The pending-reply indicator also appears when a delayed React effect has already consumed its initial guard interval, instead of staying hidden indefinitely. Delayed Settings navigation and runtime-selector closing focus no longer override a newer operator action.

PRs: [#547](https://github.com/compozy/compozy/pull/547), [#557](https://github.com/compozy/compozy/pull/557). CI follow-ups: [862e138](https://github.com/compozy/compozy/commit/862e138113f438e532777421ae3b85343162360e), [52d2c4a](https://github.com/compozy/compozy/commit/52d2c4a63f8dbaa429ff61f31671f52bee8a518b).
