---
id: ET-retired-product-surfaces-absent
area: ET
title: Keep retired products absent across public surfaces
persona: Ada
journey: J-validate-compozy-hard-cut
expected: Network, Bridges, Sandbox, agent memory, Dream, the Knowledge app, and CompozyOS-side session compaction have no active command, route, native tool, toolset, config key, hook event, trigger event, settings section, app entry, docs navigation, or example, while Tasks, sessions, Loops, Goal, gateway, ACP, SOUL.md, and HEARTBEAT.md remain usable; a home upgraded from a memory-enabled release archives or ignores the retired memory state without blocking start.
entry_points: CLI help and command catalog; HTTP and UDS route catalogs; native tool catalog; hooks introspection; settings; Web dock and palette; public docs and Marketplace; compozy status -o json; compozy config set; Web /knowledge and /settings/memory
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-09-27-pkgs-cleanup-real-runtime; /Users/pedronauck/dev/qa-labs/compozy-pkgs-cleanup-hardcut-20260927-223700-515958-lab/qa-artifacts/qa/journey-log.jsonl
last_report: docs/qa/reports/2026-09-27-pkgs-cleanup-real-runtime.md
overlaps: ET-web-catalog-navigation; RT-authored-context-lifecycle; RT-observe-overview-cli; MS-037; RT-gateway-public-ingress-bindings; RT-upgrade-memory-removal-home; RT-pressure-context-compaction
---

In an isolated fresh home, inspect the public catalogs and attempt representative retired commands,
routes, tool ids, and settings mutations. Refusal must occur before mutation, with no alias, hidden
activation, or stale navigation entry. Extension manifests with `network_participation` must be
refused in both TOML and JSON with guidance to rebuild using `[gateway]` and confirm the new
requirement digest; old stored consent must not authorize the changed declaration. Repeat after
restarting the daemon and reloading the Web.

Upgrade a disposable copy of state containing retired tables, config, and persisted windows. Verify
that migration removes retired projections according to the recorded retirement decision while
preserving unrelated Task, session, Loop, Goal, provider, workspace, and authored-context state.
The memory-removal upgrade leg is owned by `RT-upgrade-memory-removal-home` (see below).

Complete one Task create/edit/enqueue/read cycle, one session start/stop, an authored-context cycle,
and a webhook ingress inspection using the surviving public surfaces. Observe task status SSE and
cursor resumption without channel grouping. These are bounded preservation canaries; broader
product journeys keep their own existing scenarios.

Keep generic transport networking, browser Network tooling, ACP/provider bridges, Herdr integration,
and operating-system sandbox terminology distinct from the removed products. A text match alone is
not a product-residue finding. The 2026-09-27 bounded walkthrough verified public retirement and local preservation; the full scenario was deliberately skipped for this cycle as recorded below.

## 2026-09-27 bounded real runtime walk

The two local charters passed through a real isolated daemon, production Web, CLI, HTTP, and UDS.
The full scenario remains skipped in this provider-free cycle; the linked report lists exact observed
steps, retained receipts, and excluded upgrade/provider/runtime legs. This is partial scenario coverage,
not a full-scenario pass.

## PR 681 manifest rejection recheck

The current real CLI rejected both TOML and JSON legacy `network_participation` manifests in
an isolated temporary home, with explicit `[gateway]` rebuild and digest-confirmation guidance.
Receipt: `.cache/pkgs-cleanup/gateway-manifest-cli.json`. Canonical manifest-boundary and exact-digest
registry tests passed with race detection. This adds the manifest-refusal leg only; the broader
scenario exclusions above remain unchanged.

## Memory, Dream, Knowledge, and CompozyOS-side compaction (2026-10-07)

Same canary, same method: an isolated fresh home, representative attempts, refusal before mutation, no alias
or hidden activation, repeated after a daemon restart and a Web reload. The runtime E2E lane
(`make test-e2e-runtime`) owns the CLI, HTTP, and status checks; the Web checks ride the Playwright shell and
settings specs.

Absent surfaces:

- CLI: `compozy memory list` (and any `compozy memory …` verb) exits 1 with `error: unknown command "memory" for "compozy"`;
  `compozy extension init --template memory-backend-ts` fails with `extension: unknown scaffold template "memory-backend-ts"`;
  the command catalog and `compozy help` list no `memory`.
- HTTP and UDS: `GET /api/memory`, `/api/settings/memory`, and `/api/workspaces/{id}/memory/**` return 404, the same
  response as any unknown API path, on both transports; the OpenAPI catalog has no memory operation;
  `component=memory` is an invalid event filter; `type=dream` is an invalid session type;
  `PATCH /api/settings/roles` rejects removed role fields with the strict-JSON unknown-field error.
- Status: `compozy status -o json | jq 'has("memory")'` → `false` (HTTP and UDS payloads agree); `schema_version`
  is `2026-10-07`.
- Agent plane: the native tool catalog, toolset list, and hosted MCP surface offer no `compozy__memory_*` tool, no
  `compozy__memory` or `compozy__memory_admin` toolset, and no `compozy_host__memory__*` tool; the builtin
  `dreaming-curator` agent does not exist; a Host API call to `memory/*` from a running extension returns
  JSON-RPC `-32601`.
- Config: `compozy config set memory.enabled true` (any `memory.*`) is refused with
  `cli: config path "memory.enabled" is not supported by config set`; the retired `session.compaction.*` and
  `roles.dream|checkpoint_summary|memory_extractor|memory_controller` paths are refused as unknown or unsupported;
  the native `compozy__config_set` refuses the same paths deterministically.
- Automation: creating a trigger with `event = "memory.consolidated"` fails the existing trigger-event validation on
  CLI, HTTP, and the native tool; the trigger event catalog does not list it.
- Web: the dock, Go menu, and command palette show no Knowledge; `/knowledge` renders the standard not-found
  route; `/settings/memory` renders not-found (inside a restored Settings window it lands on the overview);
  Settings lists 18 sections with no Memory; Settings → Roles shows two panels (Coordinator, Auto title);
  Home shows 5 system tiles with no Memory tile; the session context meter has no threshold warning.
- Prompts and compaction: a new session's first prompt carries no memory startup content and no
  `<workspace-knowledge-snapshot>` block even when `<workspace>/knowledge/` exists; usage at 0.95 of the window
  creates no child session and archives nothing (owned in depth by `RT-pressure-context-compaction`).

Upgrade leg: a disposable home written by the previous release with retired memory tables, `config.toml` tables,
SOUL `memory_policy`, retired tool IDs, extension memory entries, and a saved Knowledge window is archived or
ignored without blocking start, with the Markdown memory files left on disk. Walk it through
`RT-upgrade-memory-removal-home`; do not duplicate it here.

Not residue: `daemon.memory_report_interval`, the `runtime.memory` doctor probe and `MS-daemon-memory-reporting`
(process memory), in-memory stores, approval memory (`CH-approval-grant-memory`), and spec-cycle workflow memory
(`cy-workflow-memory`). Markdown memory files and `<workspace>/knowledge/` directories may remain on disk after an
upgrade; nothing in CompozyOS reads them. A text match alone is not a product-residue finding.

QA impact 2026-10-07 (memory removal): the canary now also covers the memory, Dream, Knowledge, and compaction
retirement; stale skipped verdict reset to untested so the next cycle walks the added legs. The 2026-09-27 and
PR 681 receipts above still cover the Network, Bridges, and Sandbox legs.
