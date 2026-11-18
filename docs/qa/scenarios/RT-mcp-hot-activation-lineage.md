---
id: RT-mcp-hot-activation-lineage
area: RT
title: Existing and derived sessions retain tools and follow the current MCP server set
persona: Théo
journey: J-offer-runnable-capabilities
expected: Existing v0.7.0 root, provenance and derived sessions retain native, extension and MCP access after a direct upgrade or a previously applied 00202. Inherited sessions see newly admitted tools and servers. Explicit tool restrictions, spawned/coordinator lists and ancestor ceilings remain enforced.
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
4. Seeded upgrade: use a v0.7.0 home containing a root session with an unflagged stored tool/server
   list, user and system provenance sessions with `{"tools":[]}` and no materialization flags,
   and fork, continue and recovery sessions. Record their callable native/extension tools and MCP
   servers before upgrade. Upgrade directly, then repeat from a copy that already applied 00202.
   Every non-delegated session retains its previous access and sees a newly admitted native tool,
   extension tool and MCP server. Explicit `allowed_tools` profiles and spawned/coordinator lists
   remain restricted. Create a new explicitly narrowed session after 00202 and confirm the corrective
   migration preserves that limit as well. Include an archived session and check it after resume.
5. Seeded upgrade: start a second lab from a home seeded by the previous release containing a
   continued session whose stored lineage has `mcp_servers: []` and no `mcp_servers_mode` marker.
   Start the new build (migrations run), add echo, send "call echo" in the continued session. It
   succeeds. Record `compozy session inspect <id> -o json` lineage `permission_policy.mcp_servers`
   (now the effective list).
6. Spawn a child from C with an explicit MCP list that excludes echo (`compozy__session_spawn`
   with `mcp_servers: ["compozy"]`). In the child, `compozy session mcp` lists `echo` as
   `not_available` · "Limited by the parent session", and "call echo" fails.

7. From a same-workspace session with inherited MCP authority, create an always-on instance without
   naming an MCP restriction. Its stored ceiling retains the caller's inheritance mode and existing
   ancestor ceiling. An explicit tool restriction remains exact. Repeat in another workspace and
   confirm the instance is bounded by the caller's server set at creation. Include a legacy
   descendant of a depth-one instance with no intent or spawn marker: its own stored tool/server
   lists stay restricted, including for a grandchild with only a parent-session link.

Pass: C and every seeded non-delegated session retain tools and call echo without recreation; narrowed sessions keep their limits.
Same-workspace instance creation preserves stored intent, while cross-workspace creation freezes its
inherited bounds. Release repair validation is pending in main CI; no local QA lab was run.

src: .compozy/tasks/mcp-extension-hot-activation/_tests.md

Follow-up regression walk (#1741): repeat the upgrade from a database already at 00203 with a
legacy depth-one instance's main session, a parent-linked child and grandchild, an archived descendant,
and a same-ID/profile-boundary control. Their stored lists stay exact; depth-zero and foreign-profile
sessions retain their own authority. Seed a stale `meta.json` with `list` markers while the corrected
catalog says `inherit`: read-only inspection must leave the file unchanged, and resume uses catalog
intent. Sessions in the ambiguous fractional second of Goose 202 retain their restrictions; explicitly
re-grant access when needed. CI owns upgrade verification; the manual walk is pending.

QA impact 2026-11-17 (#1794, daemon CPU hot paths):
Shared hosted projections retain profile, workspace, effective policy and lineage identity. The targeted walk spawns a child restricted to `alpha` and checks its real hosted catalog excludes the parent's `beta` and newly approved `gamma`. Upgrade and browser legs retain their existing status.
See [targeted runtime evidence](../reports/2026-11-17-daemon-cpu-hot-paths.md).
