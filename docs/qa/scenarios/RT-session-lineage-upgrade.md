---
id: RT-session-lineage-upgrade
area: RT
title: Sessions created before continue/fork upgrade to a truthful lineage kind without loss
persona: Théo
journey: J-15-operate-session-via-cli-api
expected: A COMPOZY_HOME written by the pre-feature build (merge base 67b86a9b9) boots on the continue-fork build with migration 00122 applied once; every pre-existing session keeps its id, title, transcript, and meta fields, and reads back with lineage.kind "" for roots, provenance for --parent children, and spawn for spawned/coordinator/system-role sessions on session list, session status, and GET /api/sessions; a pre-feature --parent child rewinds while a spawned child is still refused as daemon-managed; a pre-feature session resumes without accepted_route and can be continued and forked like a new one.
entry_points: pre-feature compozy binary built from 67b86a9b9 on the same COMPOZY_HOME; compozy session list -o json; compozy session status <id> -o json; GET /api/sessions; compozy session rewind <child>; compozy session resume <id>; compozy session continue <id> --agent <name>; compozy session fork <id>
qa_status: pass
bug_ids: BUG-20260928-derive-stopped-source-turn-in-progress
fix_status: fixed
retest_status: pass
fix_commits: uncommitted (task_08 part B1)
evidence: docs/qa/evidence/2026-09-28-session-continue-fork-upgrade/walk-summary.json
last_report: docs/qa/reports/2026-09-28-session-continue-fork-exec-c.md
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

## 2026-09-28 walk (task_08 part C, Lab B) — PASS

Théo, Interrupt Tour, isolated lab `session-continue-fork-upgrade-20260929-005516-488280` (acpmock only;
`teardown.json` `clean: true`). Pre-feature build from `git archive 67b86a9b9`; branch build from
`git archive 3dbf81524`. Durable evidence: `docs/qa/evidence/2026-09-28-session-continue-fork-upgrade/`.

1. Pre-feature daemon (goose `121`, no `lineage_kind` column) wrote: prompted root `sess-2dac0818ae8c1d30`;
   `session new --parent` child `sess-e2840770f780c14b` (2 turns); spawned child `sess-b855be48db40d34d`
   (`POST /api/agent/spawn` over UDS with the root's agent identity headers, `spawn_role: worker`,
   2 turns); stopped session `sess-4656a9d3759db3db` with ACP id `up-alpha-session-1`. List/status/history,
   every `meta.json` (sha256 + mtime) and a DB copy recorded after `daemon stop`.
2. Branch daemon on the same home: `store.migrations.applied stream=global version=122`; a second
   restart stays at `122` (one row for version 122). Boot rewrote no `meta.json` (sha256 and mtime equal).
3. `session list -o json`, `GET /api/sessions?workspace_id=…`, and `GET …/sessions/{id}` agree:
   root and stopped `""` (omitted, `kind` is `omitempty`), `--parent` child `provenance`, spawned child
   `spawn`; the DB backfill matches. Reads did not rewrite any `meta.json`. Ids, names, and transcript
   text identical to step 1 (the extra history turn is the pre-feature `daemon stop` escalation turn).
   `session status` works on every session; its JSON is a lifecycle projection and carries no
   `lineage` block in either build (the Origin line exists only for derived children).
4. Rewind of the pre-feature `--parent` child to its first prompt: exit 0, same id, `transcript_epoch 1`,
   `draft_text "hello child"`, 10 events archived; root transcript unchanged (`max_sequence 12`, 7 entries).
   Rewind of the spawned child: CLI exit 65 and HTTP `409` `managed sessions cannot be rewound`.
5. The stopped session resumed on its next prompt (`session/load` of `up-alpha-session-1` in the acpmock
   diagnostics, reply `resumed reply`); zero `route_missing`/fallback log lines; the pre-feature meta had
   no `accepted_route`, the resumed bind wrote one. `session resume <stopped-id>` (attach) answers
   `session not attachable` for any stopped session — unchanged pre-feature attach rule, not an upgrade effect.
6. Continue (to `up-beta`, with a first message) and whole-session fork of the pre-feature root created
   exactly one child each (`sess-7080bd72cf225f26` kind `continue`, `sess-08f505551f5ea84d` kind `fork`,
   `origin_agent_name up-alpha`, two `session_derivations` rows); root transcript unchanged. Both outcomes
   reported `source_turn_in_progress: true` for a stopped source — filed as
   BUG-20260928-derive-stopped-source-turn-in-progress (reproduces on a branch-created stopped session too,
   so it is not an upgrade defect).

Metadata that lifecycle writes touched afterwards (rewound child, resumed session) now persist
`lineage.kind`; untouched sessions (root, spawned child) keep their pre-feature bytes.
Owning suites re-run green on the same head: `TestGlobalDBSessionLineageMigration`,
`TestResumeUpgradesPreFeatureLineageMetadata`, `TestConversationRewindLineageRule`,
`TestUpgradeSessionLineageKind`, `TestReadSessionMetaStopFieldsOmitted`.

Step 6 follow-up 2026-09-28 (task_08 part B1): BUG-20260928-derive-stopped-source-turn-in-progress fixed in
`internal/session/derive_cut.go` (lifecycle-only turns — hook dispatch, stop escalation, `session_stopped` — are not prompt turns and
never count as in progress). Re-walked on part B1's lab: preview of a stopped source → `source_turn_in_progress: false`
(`docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-src-preview-stopped-fixed.json`), a never-prompted session → `false`, a live running turn still `true`. Lab B (the
pre-feature root) was torn down before the fix; the same code path serves it.
