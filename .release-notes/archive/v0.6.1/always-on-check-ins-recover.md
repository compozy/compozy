---
title: Always-on check-ins and heartbeats recover on their own
type: fix
---

Several problems that could leave an always-on agent's check-ins stuck are fixed:

- A check-in reply that wraps its proposal in a sentence or two of commentary is accepted. Before, any text around the JSON rejected the reply, and repeated rejections could pause the agent. A reply with more than one candidate is still rejected, and the failure report now names the reason.
- After you fix the triage model or runtime configuration that made a check-in fail, the agent schedules its next check within its normal cadence instead of waiting up to a day. Its status explains the earlier failure and that you can run a check now.
- A manual heartbeat wake uses the `HEARTBEAT.md` currently on disk, the same policy status shows. Before, a file edited in the repository could look active in status while a wake returned `heartbeat_no_policy`.
- A heartbeat status request that asks for session health without a session ID is rejected as invalid input instead of returning `session_not_found`.

Migration notes: no action needed.
