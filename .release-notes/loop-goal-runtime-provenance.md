---
title: Loop and Goal runs keep the runtime you asked for
type: fix
---

Loop and Goal status now reports where each applied ACP option came from. Options added by the agent definition show `agent`, and options the provider could not apply no longer show a stale source. Before, the report could leave an agent-added option without a source or keep one for an option that was skipped.

A Goal that rotates to a fresh session after its context fills now asks for the reasoning level you requested, including the provider default. Before, it could ask for the level the provider had reported, which failed the Goal's runtime check and stopped recovery.

Migration notes: no action needed.
