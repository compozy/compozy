# QA Run Report — 2026-09-27 — Package cleanup real runtime

- **Scope:** bounded retirement and authored-context preservation walks; no external services or paid providers.
- **Cadence tier:** targeted
- **Build:** current working tree; daemon compiled into the isolated lab; daemon-served production Web.
- **Started:** 2026-09-27T22:37:00Z · **Status:** complete
- **Manifest:** `/Users/pedronauck/dev/qa-labs/compozy-pkgs-cleanup-hardcut-20260927-223700-515958-lab/qa-artifacts/qa/bootstrap-manifest.json`

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Structured-surface operator | Desktop / local / en-US | CH-retired-product-catalogs; CH-authored-context-persistence |

## Flows in Scope

- [J-validate-compozy-hard-cut](../journeys/J-validate-compozy-hard-cut.md): retirement surface subset only.
- [J-manage-agent-authored-context](../journeys/J-manage-agent-authored-context.md): local authoring and persistence subset.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-retired-product-catalogs | ET-retired-product-surfaces-absent: discovery, rejection, Web, restart | Ada | Feature Tour | Pass | | |
| 2 | CH-authored-context-persistence | RT-authored-context-lifecycle: CRUD, revisions, scope, restart | Ada | Feature Tour | Pass | | |
| 3 | CH-retired-product-catalogs | Disposable old-state migration, live execution/SSE, external webhook delivery | Ada | Feature Tour | Skipped | Outside bounded provider-free walk; owning integration evidence remains separate. | |
| 4 | CH-authored-context-persistence | Provider session resolution, Soul refresh, eligible live Heartbeat wake | Ada | Feature Tour | Skipped | No external services or provider execution authorized for this lab. | |

## Session Debriefs

### CH-retired-product-catalogs

Ada used the current CLI and daemon-served production Web in a fresh isolated home. The CLI root
help omitted Network, Bridge/Bridges, and Sandbox. All four attempted command groups exited 1.
Retired config writes were rejected as unsupported, and four representative native tool IDs
returned `tool_not_found`. The public native catalog contained 261 tools, zero retired product
IDs, and the surviving Task and authored-context tools. Hook discovery retained all five authored
context events and no retired product event; the public command catalog had no retired entries.

HTTP and UDS returned 404 for `/api/network`, `/api/network/peers`, `/api/bridges`, and `/api/sandbox`.
The representative roots remained 404 after restart. Ada created `qa-retained-task`, edited its
name and priority, enqueued `run-aa0ebae21ff3acea`, and independently read it through CLI, HTTP,
UDS, and Web. Scheduler pause survived restart: one queued run, zero active claims, zero provider
spawns, and zero sessions. The Goal command family remained discoverable under `session goal`;
no live Goal execution is claimed. Web showed Tasks, Loops, Agents, the retained task, two built-in
Loops, and no retired dock app. Settings omitted retired sections; Notifications showed local
Toast/Sound/System delivery and workspace muting without Bridge presets. Exact palette searches
for Sandbox and Bridges exposed no product command (the generic ask-agent fallback remained).
Network fuzzy search returned the unrelated Next workspace action, with no Network product.

The real browser was Chrome headless with an isolated profile, driven by browser-use over local
CDP. Viewport was 1290×827, DPR 2, locale en-US, timezone America/Los_Angeles. The existing Chrome
connection required consent; CUA was unavailable/timed out, so those drivers performed no workflow
steps. The isolated browser was registered in the lab PID registry. No page data or screenshots
were fabricated. Tasks and Loops were arranged through the public layout CLI for the hero capture.

Evidence: [Tasks and Loops](../evidence/2026-09-27-pkgs-cleanup-real-runtime/tasks-loops.png),
[Settings](../evidence/2026-09-27-pkgs-cleanup-real-runtime/settings.png),
[palette](../evidence/2026-09-27-pkgs-cleanup-real-runtime/palette.png),
[catalog analysis](../evidence/2026-09-27-pkgs-cleanup-real-runtime/catalog-analysis.json),
[notification controls](../evidence/2026-09-27-pkgs-cleanup-real-runtime/web-notifications-scrolled.txt).
Raw receipts and exact commands live in the manifest's `qa_output_path/qa/`, including
`removed-*`, `http-*`, `uds-*`, `restart-*`, `task-enqueue.json`, and `task-after-restart.json`.

### CH-authored-context-persistence

Ada created `qa-context` in two workspaces and authored distinct Soul and Heartbeat bodies, plus a
third distinct pair under `qa-secondary`. CLI validation, write, and read agreed with HTTP and UDS
reads. Stale CLI writes exited 65 without replacing the second version; direct HTTP and UDS stale
writes both returned 409 with `soul_conflict` or `heartbeat_conflict`. History listed revision IDs,
rollback restored the first body, deletion returned `present=false`, and managed writes recreated
the bodies. A rollback attempt without its required digest flag was correctly refused; it was not
used as restoration evidence. Foreign workspace reads initially reported missing sidecars, and the
foreign profile initially refused the absent agent. After authoring the distinct bodies, each scope
kept its own digest. Restart preserved primary authored bodies, digests, and complete revision lists.

The native Heartbeat status tool returned completed/valid. The Web agent Instructions controls
retained AGENT.md, SOUL.md, and HEARTBEAT.md; the editor fields independently showed the exact
restored first bodies after restart. Heartbeat displayed enabled status, 30-minute minimum interval,
no recent wake, and a disabled wake action explaining that an active session is needed. No provider
session or eligible wake was started. A nonexistent-session dry-run wake was refused by CLI; native
wake and health invocation returned the generic tool-backend error for that absent session. This
proves refusal only, not live wake behavior or typed reason parity for a real ineligible session.

Evidence: [authored context UI](../evidence/2026-09-27-pkgs-cleanup-real-runtime/authored-context.png),
[persistence comparison](../evidence/2026-09-27-pkgs-cleanup-real-runtime/preservation-comparison.json),
[Soul editor](../evidence/2026-09-27-pkgs-cleanup-real-runtime/web-soul-fields.json),
[Heartbeat editor](../evidence/2026-09-27-pkgs-cleanup-real-runtime/web-heartbeat-observed.json).
Raw `soul-*`, `heartbeat-*`, and `compozy__agent_heartbeat_*` receipts retain every command,
response, scope, digest, and revision. No private database or implementation inspection was used
as the behavioral verification oracle.

## What Was Fixed

No product edits are owned by this QA session; findings go to the responsible owner.

## Paper Cuts

No user-impact finding observed in the bounded walk.

## Runtime Errors Observed

No application crash or persistent connection failure observed. The expected layout reconnect state
appeared during the deliberate daemon restart and recovered after reload. Daemon startup logged an
existing user-skill verification warning for `motion-graphics`; the lab did not modify that skill.
Expected negative-probe errors remain in the raw receipts.

## Human Verifications Needed

None requested; omitted provider-dependent legs are explicit scope exclusions.

## Decisions for a Human

None identified.

## Learnings

The public CLI already supports the complete local authored-context mutation cycle without a
provider. Scheduler pause makes an enqueue canary safe to inspect without launching paid work.
The broad scenarios include distinct upgrade and provider-runtime contracts, so the bounded
observations must not turn either full scenario into a pass.

## Final Status

PASS for the two bounded local charters: two Pass rows, two Skipped rows, zero pending rows and
zero product bugs identified in the exercised paths. The full scenarios remain `skipped` for this
cycle because their additional legs were explicitly excluded: old-state migration, live session
start/stop and Goal execution, task SSE cursor resumption, external webhook delivery, package-owned
sidecar mutation refusal, session Soul refresh, and an eligible advisory wake. The trigger inspection
only confirmed that the public webhook event filter remains usable with an empty fresh catalog.

Verification is separate from behavioral proof. Current daemon build (`go build -p 4`) passed.
Root-filtered Turbo Web build passed in `/tmp/pkgs-cleanup-qa-web-build.log` (1 task, 39.142s).
The owning filtered Web/UI/SDK typecheck and lint receipt is
`/tmp/pkgs-cleanup-web-ui-final-checks2.log` (6 tasks, zero errors/warnings). Root owns the broader
`make gate` and final generation checks; this report does not claim their completion or CI success.
The strict targeted evidence audit and manifest teardown results are recorded below before handoff.


The strict targeted evidence auditor returned PASS with zero blockers and zero warnings.
The exact manifest teardown command exited 0 and recorded `clean: true`; the daemon and isolated
Chrome processes are stopped. The personal browser and unrelated runtimes were not targeted.
See [audit](../evidence/2026-09-27-pkgs-cleanup-real-runtime/qa-audit-report.md) and
[teardown](../evidence/2026-09-27-pkgs-cleanup-real-runtime/teardown.json).
