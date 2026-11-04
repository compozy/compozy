---
title: "Always allow" now sticks for cross-workspace reads
type: fix
---

When an always-on agent asked to read another workspace and you chose **Always allow**, the answer was forgotten as soon as the agent started a new session or the daemon restarted, so you were asked again.

The answer is now saved as a tool approval for that agent, tool and target workspace/profile, and it survives new sessions and restarts. It covers only that read. Writes, other tools, other targets and other agents still ask. You can review and revoke it in Settings with your other tool approvals.
