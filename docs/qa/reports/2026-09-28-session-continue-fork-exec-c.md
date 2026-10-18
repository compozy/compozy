# QA Run Report — 2026-09-28 — session-continue-fork (part C: pre-feature home upgrade walk)

- **Scope:** `RT-session-lineage-upgrade` and the pre-feature provenance leg of `RT-conversation-rewind` (task_08 part C). Parts B1/B2: `2026-09-28-session-continue-fork-exec-b1.md`, `2026-09-28-session-continue-fork-exec-b2.md`. Plan: `2026-09-28-session-continue-fork-plan.md`.
- **Cadence tier:** targeted · **Status:** closed


Lab B `session-continue-fork-upgrade-20260929-005516-488280` (manifest `~/dev/qa-labs/compozy-session-continue-fork-upgrade-20260929-005516-488280-lab/qa-artifacts/qa/bootstrap-manifest.json`), acpmock only. Pre-feature `compozy` + acpmock driver built from `git archive 67b86a9b9`; branch binaries from `git archive HEAD` (`3dbf81524`, excludes part A's uncommitted edits). Lab fixture `qa-artifacts/fixtures/upgrade_fixture.json` (agents `up-alpha`, `up-beta`); driver swapped via the `acpmock-driver-current` symlink between builds.

- **`RT-session-lineage-upgrade`: PASS.** Goose 121 → 122 applied once (restart stays 122). Kinds `""`/`provenance`/`spawn` agree on `session list -o json`, `GET /api/sessions?workspace_id=`, `GET …/sessions/{id}`, and the DB backfill. `meta.json` sha256/mtime unchanged by boot, reads, rewind refusal, derive-as-source, and restart; only lifecycle writes (rewind rebind, prompt resume) rewrite a meta, and then persist `kind`. History intact. `--parent` child rewind ok (epoch 1, parent untouched); spawned child 409. Stopped session resumes via `session/load` on its next prompt, no `route_missing`. Continue and fork of the pre-feature root created one child each. Owning suites re-run green (`TestGlobalDBSessionLineageMigration`, `TestResumeUpgradesPreFeatureLineageMetadata`, `TestConversationRewindLineageRule`, `TestUpgradeSessionLineageKind`, `TestReadSessionMetaStopFieldsOmitted`). Strict audit PASS.
- **`RT-conversation-rewind`:** pre-feature provenance leg + spawned 409 recorded as a body note; status left for the Lab A web / new-child legs.
- **Bug filed: `BUG-20260928-derive-stopped-source-turn-in-progress`** (open, P2, Trust-Damage). Any stopped source (branch-created too) reports `source_turn_in_progress: true` on preview and on the continue/fork outcome, so Web shows "A turn is still in progress…" and the fork dialog drops its fences. Hypothesis: `lastSettledTurn` (`internal/session/derive_cut.go`) treats the stop lifecycle turn (`session.stop_escalated`/`session_stopped`/stop marker, no `done`) as open. Repair is routed to the controller; affects the ET-web continue/fork walks and VC bundles on stopped sources.
- **Spawned child creation:** no CLI verb; used `POST /api/agent/spawn` over UDS with the root's `X-Compozy-Session-ID`/`X-Compozy-Agent` headers (`ttl_seconds` must be > 0).
- **Not a regression:** `session resume <stopped-id>` answers `session not attachable` (attach requires `state=active`, same rule in 67b86a9b9); a stopped session revives on prompt.
- **Teardown:** `teardown.json` `clean: true`. `make qa-reap` ran only after a dry run showed no other live lab (Lab A was not up yet). Hazard for later runs: `teardown-qa-env.py --all` SIGTERMs every PID in every stale `pids/*.pid` across 113 old labs without checking the process identity (`collect_target_pids` → `registered_pids`), so a recycled PID could hit an unrelated process. Do not run `make qa-reap` while Lab A or other agents' labs are live.
- **Durable evidence:** `docs/qa/evidence/2026-09-28-session-continue-fork-upgrade/` (walk-summary.json, DB snapshots, meta sha lists, 409 body, preview repro, owning-suites.log). Lab scratch stays under the lab root (not purged).
- **Owed to the controller:** fold this leg into `docs/qa/reports/2026-09-28-session-continue-fork.md` (not created by part C; the scenario's `last_report` points there); route the bug fix plus a re-walk of step 6 (preview on a stopped source must read `false`).


## Follow-up

- `BUG-20260928-derive-stopped-source-turn-in-progress` fixed in part B1 (`internal/session/derive_cut.go`, `TestLastSettledTurn`); step 6 re-walked: stopped and never-prompted sources preview `source_turn_in_progress: false`, a running turn still `true`.
