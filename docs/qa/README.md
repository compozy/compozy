# QA — Living Docs

Canonical QA tree for AGH. Owned by the `qa-report` (planning) + `qa-execution` (sessions) skill pair; `real-scenario-qa` (playbook lab + runtime observation) also lands its findings here. One tree, forever: rounds append, ids never reset, history lives in dated reports.

## Layout

- `scenarios/<AREA>-<slug>.md` — source-of-truth scenario tracker, one flat-frontmatter file per behavior. Existing counter-based scenario ids are grandfathered; new ids are content-addressed.
- `state.csv` — generated, gitignored tracker view. Regenerate with `python3 .agents/skills/qa-report/scripts/materialize_state.py docs/qa`; never edit or commit it.
- `bugs/BUG-<YYYYMMDD>-<slug>.md` — global bug registry. Existing `BUG-0001..0038` ids are grandfathered; new ids are content-addressed and deduplicated by symptom.
- `journeys/J-<slug>.md` — journey maps + Mermaid flows. Existing `J-01..J-32` ids are grandfathered.
- `charters/CH-<slug>.md` — immutable session missions. Existing `CH-001..CH-052` ids are grandfathered; debriefs belong only in run reports.
- `reports/<YYYY-MM-DD>-<scope>.md` — one per run, never overwritten
- `evidence/<date>-<scope>/` — checkpoint/failure screenshots + cited run artifacts only (lean). **Skeeper-managed** (`git@github.com:compozy/specs.git`, namespace `agh`, pattern `docs/qa/evidence/**`): gitignored from the main repo, mirrored to the sidecar, restored via `skeeper restore --all`. Reports reference evidence by repo-relative path, which resolves after restore. Uncited bulk dumps are pruned before sync.
- `automation-backlog/<slug>.md` — one conflict-resistant automation intent per file.
- `templates/` — project copies of scenario, bug, charter, and report templates.

## Area codes (scenario id prefixes)

| Code | Area |
|---|---|
| RT | Runtime & sessions (daemon, session lifecycle, providers) |
| TA | Tasks & automation (task runs, leases, scheduling, loops) |
| ET | Extensibility & tools (extensions, hooks, skills, registries, bundles) |
| NB | Network & bridges (channels, threads, bridge SDKs, delivery) |
| MS | Memory & settings (memory, config lifecycle, sandbox/env) |
| LP | Loops (workflow runs, catalog, configure/fork, editor) |
| GL | Goal (conversational convergence, controls, context, recovery) |

New areas: define the code here first, then mint ids.

## Entry points

- CLI: `agh` (structured output; UDS + HTTP parity)
- Web: `make web-dev` (export `AGH_WEB_API_PROXY_TARGET` from the bootstrap manifest for isolated labs)
- Release/scenario labs: `agh-qa-bootstrap` skill (isolated `AGH_HOME`/ports/provider homes; see CLAUDE.md Workflow Rules)

## Adopted from

### Conflict-resistant living docs (2026-07-12)

- Exploded the 439-row tracked `state.csv` into `scenarios/`, preserving every id, verdict, link, evidence path, report path, overlap, and note. The converter exposed and repaired one malformed legacy row (`RT-083`, one surplus empty CSV field) before the 439-file round trip passed.
- `state.csv` is now a disposable materialized view and is ignored by Git. Scenario files are the only tracker source of truth.
- Split the 12 `AB-001..AB-012` entries from the shared `automation-backlog.md` into content-addressed files under `automation-backlog/`; legacy ids remain recorded inside the files for historical report lookup.
- Grandfathered the healthy single-tree counter ids already cited by reports: scenarios, `BUG-0001..0038`, `J-01..J-32`, and `CH-001..052`. New artifacts use the content-addressed formats documented above; no counter is read again.
- Existing charter debriefs are frozen pre-migration history. Executors must not append to them; every new debrief is written incrementally into its dated report.
- The pre-migration README changelog was closed and moved to `reports/2026-07-12-living-docs-migration.md`; cycle history continues exclusively in dated reports.

### Initial consolidation (2026-07-05)

- The legacy `state.csv` was seeded from the feature-stories tracker (253 stories, cycle 2026-06). Its frozen origin and five subsystem analyses live in `_seeds/feature-stories/`; its rows now live in `scenarios/`. Original prose statuses remain in scenario bodies (`migrated-status:`). Empty journey fields remain intentional until a journey flow legitimately owns the behavior.
- `bugs/BUG-0001..0017` re-minted from the feature-stories registry (old per-round `BUG-001..017`); impact tiers unclassified — classify on next touch.
- **Evidence caveat:** the origin lab (`~/dev/qa-labs/agh-feature-stories-20260621-...-lab/`) was accidentally deleted during the 2026-07-05 cleanup, so its lab-relative `qa/evidence/...` and `qa/issues/...` paths in scenario files are dangling. Treat those migrated `pass` verdicts as historical claims backed by the surviving `file:line` code citations; the next Full cycle re-validates with fresh evidence.
- `_seeds/final-qa/` — pre-release master plan (283 scenarios across 15 modules) + openclaw/hermes QA pattern libraries, from the retired `.compozy/tasks/final-qa/`. Mine into journeys/charters as cycles touch each module, then prune.
- `_seeds/qa-e2e-playbook.md` — the 2026-04 E2E playbook (evidence standard, execution profiles, suite matrix, automation backlog seed), formerly `docs/ideas/qa-e2e/`.
- Historical per-round QA trees (29 under `.compozy/tasks/_archived/*/qa/`), `final-qa/_runs/` evidence, and 28 stale external labs were deleted on 2026-07-05 (no live references; ids collided across rounds and were never migrated).
