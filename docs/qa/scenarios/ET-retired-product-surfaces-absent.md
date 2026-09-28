---
id: ET-retired-product-surfaces-absent
area: ET
title: Keep retired products absent across public surfaces
persona: Ada
journey: J-validate-compozy-hard-cut
expected: Network, Bridges, and Sandbox have no active command, route, native tool, hook event, settings section, app entry, docs navigation, or example, while Tasks, sessions, Loops, Goal, gateway, ACP, SOUL.md, and HEARTBEAT.md remain usable.
entry_points: CLI help and command catalog; HTTP and UDS route catalogs; native tool catalog; hooks introspection; settings; Web dock and palette; public docs and Marketplace
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-09-27-pkgs-cleanup-real-runtime; /Users/pedronauck/dev/qa-labs/compozy-pkgs-cleanup-hardcut-20260927-223700-515958-lab/qa-artifacts/qa/journey-log.jsonl
last_report: docs/qa/reports/2026-09-27-pkgs-cleanup-real-runtime.md
overlaps: ET-web-catalog-navigation; RT-authored-context-lifecycle; RT-observe-overview-cli; MS-037; RT-gateway-public-ingress-bindings
---

In an isolated fresh home, inspect the public catalogs and attempt representative retired commands,
routes, tool ids, and settings mutations. Refusal must occur before mutation, with no alias, hidden
activation, or stale navigation entry. Repeat after restarting the daemon and reloading the Web.

Upgrade a disposable copy of state containing retired tables, config, and persisted windows. Verify
that migration removes retired projections according to the recorded retirement decision while
preserving unrelated Task, session, Loop, Goal, provider, workspace, and authored-context state.

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
