---
title: Carry the selected Profile through Loop compilation and worker sessions
type: fix
---

Profile-scoped Agents, Skills, Loops, and Session references now resolve consistently through validation, persisted responses, and execution. Orchestrated implementers and nested daemon-issued Session commands retain the validated caller Profile while preserving workspace and identity checks.

Lifecycle extension tools are authorable only in the workspace and Profile where they are enabled. Compilation sees the same tool schema and placement as execution, with normal validation and permission policy still enforced.

Loop-managed sessions also inherit configured ACP options and speed, with explicit runtime options taking precedence. This prevents session creation-profile mismatches before the provider starts.

PRs: [#527](https://github.com/compozy/compozy/pull/527), [#531](https://github.com/compozy/compozy/pull/531), [#542](https://github.com/compozy/compozy/pull/542).
