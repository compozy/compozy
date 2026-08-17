---
title: "Attention: know which session needs you"
type: feature
---

CompozyOS has one daemon-owned attention model. Pending input, permission requests, finished-unseen sessions, operator presence, notification delivery, and cross-workspace session discovery are runtime state instead of per-surface guesses. Orchestrator agents get structured wait, spawn, stop, approve, clarify-answer, prompt-cancel, and notify controls across native tools, CLI, HTTP, and UDS, so an agent supervising other agents no longer polls a shell or depends on the web UI to act. (#422)

- The global catalog persists canonical attention revisions, pending interactions, seen and settled state, and cursor-stable attention ordering.
- Presence leases, attention summaries and events, sanitized interaction discovery, generalized waits, prompt cancellation, operator notifications, and session wake propagation are available on every transport, with deterministic CLI exit behavior.
- Desktop shortcuts moved to a daemon-owned, configurable keymap, and the command palette gained nested views including an attention-first Sessions view.
