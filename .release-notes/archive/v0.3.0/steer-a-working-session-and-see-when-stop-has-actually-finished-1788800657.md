---
title: Steer a working session and see when Stop has actually finished
type: feature
---

Follow-ups now default to steering. Runtimes that advertise live steering receive the guidance inside the active turn; other runtimes report an explicit interrupt-and-replace fallback. The composer shows what happened, restores refused drafts, and Settings lets you choose queueing instead. CLI and native-tool responses expose the send outcome and steering capability.

Stop uses cooperative cancellation, forced stop, and process-group termination with process identity checks. The UI stays at Stopping until the runtime can verify the outcome; an unverifiable stop remains visible with attention. Canceling a turn keeps the session promptable, including when escalation needs to replace its process. Restart reconciliation identifies crashed agents and stale decisions instead of leaving phantom activity.

PR: [#555](https://github.com/compozy/compozy/pull/555).
