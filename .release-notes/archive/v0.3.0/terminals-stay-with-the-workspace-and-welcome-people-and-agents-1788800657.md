---
title: Terminals stay with the workspace and welcome people and agents
type: feature
---

Open real terminals inside CompozyOS, watch an agent's deliberate command run live, and reconnect after closing a window or reloading the app while the daemon keeps the process running. Visible agent terminals appear without taking focus. The same terminals are available through the Web and Desktop apps, CLI, HTTP/UDS, and native tools.

Authorized people and agents in the same workspace and profile can type, resize, answer input, signal, and close concurrently. Each submitted write stays whole and actor-attributed; there is no control handoff. Explicit read-only attachments remain available. Agent execution uses the approval policy, and hidden input is redacted from retained terminal surfaces.

A command journal records who ran what, approval, outcome, and boundary-detection confidence. Recording is opt-in with retention limits. Local macOS, Linux, and Windows support interactive terminals; remote sandboxes support command execution and journaling without interactive attachment.

PRs: [#490](https://github.com/compozy/compozy/pull/490), [#552](https://github.com/compozy/compozy/pull/552).
