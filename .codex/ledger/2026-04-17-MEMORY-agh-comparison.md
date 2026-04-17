Goal (incl. success criteria):

- Produce a concise, read-only product-architecture comparison between Compozy (`looper`) and AGH (`agh`) with a reuse map for daemonization.
- Success means: identify which AGH subsystems/patterns Compozy should borrow first, where direct copying would be a mistake, and the smallest viable target architecture for Compozy focused on failover, second-interface support, and extension-triggered orchestration.

Constraints/Assumptions:

- Read-only task. Do not edit source files in either repository.
- Must include concrete file references from both repositories.
- Scope is product architecture, not implementation planning in code.
- Need cross-agent awareness by reading other ledger files only; never modify them.

Key decisions:

- Use AGH as the reference architecture for daemon/process/interface patterns, but evaluate reuse against Compozy's existing run journal, hook runtime, and extension model instead of assuming one-to-one adoption.
- Focus analysis on high-signal composition-root, runtime, persistence, API transport, and extension orchestration files.

State:

- In progress.

Done:

- Read root instructions for Compozy and AGH.
- Read the `architectural-analysis` skill.
- Read relevant prior ledgers covering Compozy daemon architecture, extension foundation, hook dispatches, provider registration, and workflow headless streaming.

Now:

- Map Compozy runtime, extension, and interface seams with concrete files.
- Map AGH daemon, API transport, persistence, and failover-related seams with concrete files.

Next:

- Synthesize a reuse-first comparison and smallest viable target architecture.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: Whether “failover” should be framed narrowly as local daemon crash/reconnect recovery or more broadly as executor/process failover across long-running workflows.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-agh-comparison.md`
- `.codex/ledger/2026-04-17-MEMORY-daemon-architecture.md`
- `.codex/ledger/2026-04-10-MEMORY-hook-dispatches.md`
- `.codex/ledger/2026-04-11-MEMORY-ext-provider-registration.md`
- Commands: `rg`, `sed`, `find`, `wc`
