---
id: RT-session-lineage-upgrade
area: RT
title: Sessions created before continue/fork upgrade to a truthful lineage kind without loss
persona: Théo
journey: J-15-operate-session-via-cli-api
expected: A COMPOZY_HOME written by the pre-feature build (merge base 67b86a9b9) boots on the continue-fork build with migration 00122 applied once; every pre-existing session keeps its id, title, transcript, and meta fields, and reads back with lineage.kind "" for roots, provenance for --parent children, and spawn for spawned/coordinator/system-role sessions on session list, session status, and GET /api/sessions; a pre-feature --parent child rewinds while a spawned child is still refused as daemon-managed; a pre-feature session resumes without accepted_route and can be continued and forked like a new one.
entry_points: pre-feature compozy binary built from 67b86a9b9 on the same COMPOZY_HOME; compozy session list -o json; compozy session status <id> -o json; GET /api/sessions; compozy session rewind <child>; compozy session resume <id>; compozy session continue <id> --agent <name>; compozy session fork <id>
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-conversation-rewind; ET-cli-session-continue; RT-session-fallback-chain
---

Planning 2026-09-28 (session-continue-fork task_07): the compatibility leg deferred by task_01 (live
dev-daemon check of the migration backfill and the `meta.json` read-boundary upgrade) and task_05 (a
pre-feature home). SD-013 user-state regime: lossless upgrade, no manual step.

1. Build the pre-feature binary from the merge base into the lab (`git archive 67b86a9b9 | tar -x`,
   then `go build -o "$LAB/bin/compozy-prefeature" ./cmd/compozy`). Boot it on the lab
   `COMPOZY_HOME` with the acpmock provider and create: a prompted root session; a
   `session new --parent <root>` child, prompted; a spawned child (agent tool call, or a
   coordinator/role session if the fixture cannot spawn); one stopped session with an ACP id.
   Record `session list -o json`, each `session status -o json`, and a copy of every `meta.json`.
2. Stop the pre-feature daemon; start the branch build on the same home. Boot succeeds, the goose
   version is 00122, and no session directory is moved or rewritten on boot.
3. `session list -o json` and `GET /api/sessions` report `lineage.kind` `""` / `provenance` / `spawn`
   as expected; ids, titles, message counts, and `max_sequence` match step 1.
4. Rewind the `--parent` child to its first prompt: succeeds, same id, parent untouched. Rewind the
   spawned child: `409` daemon-managed.
5. Resume the stopped session: it loads normally (no `accepted_route` in its meta; no
   `session.fallback.route_missing` log).
6. Continue and fork the pre-feature root: exactly one child each, `lineage.kind` continue/fork,
   source unchanged.

Automated evidence at authoring time: `TestGlobalDBSessionLineageMigration` (IT-017, backfill on a
seeded pre-migration catalog), `TestResumeUpgradesPreFeatureLineageMetadata` (IT-019),
`TestUpgradeSessionLineageKind`, `TestReadSessionMeta` (UT-002). task_08 owns the live walk.
