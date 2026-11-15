---
title: Preserve subagent CLI error compatibility
type: fix
---

`compozy session subagents` list, show and cancel keep the v0.7.0 exit code 1 for daemon errors and print each error once, including when a workspace is selected explicitly.
