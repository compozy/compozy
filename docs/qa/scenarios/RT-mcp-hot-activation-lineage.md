---
id: RT-mcp-hot-activation-lineage
area: RT
title: Forks, continues and pre-upgrade continued sessions follow the current MCP server set
persona: Théo
journey: J-offer-runnable-capabilities
expected: A session C forked from B, itself continued from A, calls a server added after all three exist, on its next message. A continued session created by a pre-upgrade build in a seeded home does the same after the upgrade (marker backfill migration). An explicitly narrowed child stays without the server.
entry_points: compozy session continue; compozy session fork; web Settings → MCP servers; compozy session mcp <id>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-mcp-hot-activation-next-message
---

Covers E2E-002 (US-004, US-001.AC-3).

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket), built from the
integrated `feat/mcp-hot-activation` head, default `tools.policy.external_default = "disabled"`, real
provider CLIs signed in through the lab's provider homes. Echo fixture: a Streamable HTTP or stdio MCP
server exposing an `echo` tool (for example `npx -y @modelcontextprotocol/server-everything`).
Capture every command with timestamp and output, plus a screenshot of each Web state named below.

Steps:

1. Create session A (Claude), continue it into B (`compozy session continue <A>`), fork B into C
   (`compozy session fork <B>`). Send one message in C so it binds.
2. Add the echo fixture in Settings (Personal). In C send "call echo". It succeeds.
3. `compozy session mcp <C> -o json`: `echo` is `ready`; no `not_available` rows.
4. Seeded upgrade: start a second lab from a home seeded by the previous release containing a
   continued session whose stored lineage has `mcp_servers: []` and no `mcp_servers_mode` marker.
   Start the new build (migrations run), add echo, send "call echo" in the continued session. It
   succeeds. Record `compozy session inspect <id> -o json` lineage `permission_policy.mcp_servers`
   (now the effective list).
5. Spawn a child from C with an explicit MCP list that excludes echo (`compozy__session_spawn`
   with `mcp_servers: ["compozy"]`). In the child, `compozy session mcp` lists `echo` as
   `not_available` · "Limited by the parent session", and "call echo" fails.

6. From a same-workspace session with inherited MCP authority, create an always-on instance without
   naming an MCP restriction. Its stored ceiling retains the caller's inheritance mode and existing
   ancestor ceiling. An explicit tool restriction remains exact. Repeat in another workspace and
   confirm the instance is bounded by the caller's server set at creation.

Pass: C and the seeded continued session call echo without recreation; the narrowed child never gets it.
Same-workspace instance creation preserves stored intent, while cross-workspace creation freezes its
inherited bounds. Release repair validation is pending in main CI; no local QA lab was run.

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md
