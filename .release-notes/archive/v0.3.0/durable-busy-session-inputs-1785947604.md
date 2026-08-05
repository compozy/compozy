---
title: Durable inputs for busy sessions
type: fix
---

Queue, Steer, and Interrupt are now daemon-owned durable operations instead of client-side intent that could quietly disappear. An input is persisted before it is acknowledged, survives a refresh and a daemon restart, dispatches exactly once in FIFO order, and can be listed, edited, canceled, or promoted to steering by its entry ID from the CLI, HTTP, UDS, native tools, or the extension host. Disruptive changes are fenced against the turn you meant to change, so a stale client cannot interrupt a newer turn. (#304)

- `compozy session prompt` accepts `--queue`, `--interrupt`, and `--steer`; `compozy session input list|edit|steer|cancel` manages pending input by its persisted ID.
- The queue is readable and mutable over HTTP and UDS at `/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue`, including per-entry replace, steer, and cancel.
- Agents get `compozy__session_inputs_list`, `compozy__session_input_replace`, `compozy__session_input_cancel`, and `compozy__session_input_promote`.
- The composer clears only after the daemon acknowledges, a failure keeps your draft, and a refresh reconstructs pending input from the daemon. Queued, steered, interrupted, canceled, accepted, and dropped markers no longer render as warnings, and an expected ACP cancellation no longer appears in the transcript as a provider failure.

Migration notes: the dedicated interrupt endpoint is removed — interrupt is now a prompt mode plus a fenced queue operation. The legacy ACP steer handler and the runtime steer source are removed, and the web client no longer mirrors the queue in local state.
