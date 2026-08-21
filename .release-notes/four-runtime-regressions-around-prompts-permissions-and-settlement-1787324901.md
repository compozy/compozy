---
title: Four runtime regressions around prompts, permissions, and settlement
type: fix
---

A cluster of failures where the runtime reported the wrong thing about its own state. (#447)

- **Prompts stop replaying history** (fixes #399). Submitting a message sent the entire persisted chat transcript to the session prompt endpoint. Only the newest user message goes now, with its original message ID and retry idempotency preserved.
- **Permissions follow the live agent** (fixes #415). The observer carried a duplicate permission-mode resolver instead of the resource-backed agent catalog the daemon uses. Live snapshots are built from effective permissions and cached by runtime identity and revision, so a revision change can no longer leave stale permissions in place; a stopped session's fallback snapshot stays deliberately shallow.
- **A committed result survives a failed publish** (fixes #435). A pre-commit lease failure and a post-commit publication failure looked identical, so an already-settled run could be failed and settled a second time. A claimed run fails only when completion returned no committed run.
- **A crashed process is reported as crashed** (fixes #436). A non-zero exit code or a signal is now classified as a process failure and maps to the process-exited stop cause. A clean exit with code `0` after a transport failure stays on the transport path and keeps `error`, so `agent_crashed` means the subprocess actually died.
