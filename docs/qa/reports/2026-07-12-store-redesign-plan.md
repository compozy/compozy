# QA Plan — store-redesign (2026-07-12)

**Cycle:** store-redesign (Goose/Atlas/sqlc hard cut, repository partition, and structured schema diagnostics).
**Tier:** Targeted — the changed daemon-schema journey plus one adjacent persistence canary.
**Scope source:** `.compozy/tasks/store-redesign/_techspec.md`, `_tests.md`, ADR-001..004, Tasks 01–05,
and their workflow memories. This infrastructure program intentionally has no PRD or `_user_stories.md`.
**Planning only:** all scoped scenarios remain `untested`; Task 07 owns execution, evidence, bugs, and verdicts.

## No-UI rationale

The only new public payload field is additive and intentionally unrendered. The plan therefore exercises daemon
startup, CLI, HTTP, and UDS. It does not invent a web control or spend a browser session proving an absent UI.

## Session order

| Risk order | Journey | Charter | Persona | Tour | Time box |
| --- | --- | --- | --- | --- | --- |
| 1 | J-operate-daemon-schema | CH-database-refusal-recovery | Bruno | Garbage Tour | 60 min |
| 2 | J-operate-daemon-schema | CH-daemon-schema-parity | Ada | Feature Tour | 30 min |
| 3 · canary | J-20 | CH-model-catalog-storage-canary | Ada | Feature Tour | 30 min |

The refusal session runs first because a silent mutation has the highest blast radius. The parity session proves
the supported path and shared-file ownership. The model-catalog session is the adjacent canary because Task 05's
real-runtime E2E exposed timestamp-nullability and error-context regressions in that Task 04 repository.

## Tasks 01–05 coverage map

| Change slice | User/operator risk | Planned coverage |
| --- | --- | --- |
| Task 01 · migration engine and baselines | Fresh apply, sum/version state, shared-file stream collision, ahead database | CH-daemon-schema-parity; CH-database-refusal-recovery; automated `_tests.md` migration/equivalence suites remain the table-level owner |
| Task 02 · hard-cut open paths | Legacy daemon refusal, byte preservation, whole-family recovery, direct-open CLI error shape, eager memory stream | CH-database-refusal-recovery; RT-refuse-legacy-database; RT-refuse-legacy-cli-open; RT-refuse-ahead-database |
| Task 03 · deterministic Atlas/sqlc codegen | No runtime surface; generated drift must stay green | Task 07 final `make verify`/`codegen-check`; no persona scenario invented for build plumbing |
| Task 04 · repositories and sqlc | Intended invisible refactor; adjacent persistence drift | CH-model-catalog-storage-canary over MS-043/MS-044; broad automated parity remains the owning gate |
| Task 05 · `schema_streams`, docs, skill | Cross-surface drift, incomplete global/memory state, stale broad status envelope | CH-daemon-schema-parity; RT-inspect-schema-streams; RT-preserve-shared-schema-isolation; reset RT-001 |

## Surface → scenario → charter

| Surface | Concrete entry point | Scenario(s) | Charter |
| --- | --- | --- | --- |
| Daemon fresh boot | `agh daemon start --foreground` | RT-inspect-schema-streams; RT-preserve-shared-schema-isolation | CH-daemon-schema-parity |
| HTTP status | `GET /api/status` | RT-inspect-schema-streams; RT-001 | CH-daemon-schema-parity |
| UDS status | `GET /api/status` over the daemon socket | RT-inspect-schema-streams; RT-001 | CH-daemon-schema-parity |
| CLI status | `agh status -o json` | RT-inspect-schema-streams; RT-001 | CH-daemon-schema-parity |
| Shared-file domain smoke | `agh workspace list -o json`; `agh memory list -o json` | RT-preserve-shared-schema-isolation | CH-daemon-schema-parity |
| Legacy/ahead daemon refusal | `agh daemon start --foreground` against prepared fixtures | RT-refuse-legacy-database; RT-refuse-ahead-database | CH-database-refusal-recovery |
| Local extension open | `agh extension list -o json` with daemon stopped | RT-refuse-legacy-cli-open; RT-refuse-ahead-database | CH-database-refusal-recovery |
| Local MCP-auth open | `agh mcp auth status -o json` with daemon stopped | RT-refuse-legacy-cli-open; RT-refuse-ahead-database | CH-database-refusal-recovery |
| Local provider-auth open | `agh provider auth status <bound-secret-provider> -o json` | RT-refuse-legacy-cli-open | CH-database-refusal-recovery |
| Model-catalog canary | `agh provider models refresh -o json`; `agh provider models status -o json` | MS-043; MS-044 | CH-model-catalog-storage-canary |

## Regression hot spots

- **Legacy probe and ADR-002 hard cut:** digest the prepared database before and after every refused open;
  require refusal before readiness and remediation that stops AGH, preserves or moves the complete containing
  `AGH_HOME` or workspace `.agh` family, and selects a separate fresh home. No single-file or in-place repair.
- **Shared-file isolation (Safety Invariant 4):** require exactly the ordered `global` and `memory` status entries,
  then smoke public global and memory reads before and after restart. Automated store suites own physical table
  disjointness; manual QA does not use direct SQLite reads as its verdict source.
- **Ahead refusal (Safety Invariant 12):** prepare the ahead fixture before the session, then require a newer binary
  or complete-family preservation followed by a separately selected fresh home from daemon and direct-open paths
  without modifying the recorded version.
- **Public mapping boundary (Safety Invariant 9):** compare the contract-shaped fields over HTTP, UDS, and CLI;
  no storage-layer type or extra per-session/per-workspace stream may appear.
- **Task 04 canary:** model refresh/status must tolerate empty optional timestamps, retain source context in failures,
  redact secrets, and rehydrate after restart.

## Journey × taxonomy coverage

| Journey | Journey | Functional | Experiential | Edge/error/empty | Cross-cutting |
| --- | --- | --- | --- | --- | --- |
| J-operate-daemon-schema | Fresh/legacy/ahead branches reach a true running-or-preserved end state | Status parity, domain reads, checksums, deterministic remediation | Ada gets parseable structured output; Bruno gets actionable recovery copy | Legacy, ahead, stopped-daemon direct opens, abandon-and-resume with preserved home | HTTP/UDS/CLI consistency and restart continuity; UI/a11y/mobile deliberately skipped because no rendered surface changed |
| J-20 canary | Refresh → status → restart readback | Optional timestamps, source ownership, stale preservation | Ada receives stable, redacted diagnostics | Deterministic failing source when the lab provides one; otherwise the branch is explicitly skipped | Persistence/restart regression canary; full curation/native/web coverage remains owned by CH-031 |

## Entry and exit gates for Task 07

- Bootstrap one fresh isolated lab and record the manifest, public base URL, socket path, PID registry, and
  `TEARDOWN_COMMAND` before starting a daemon.
- Execute all three charters; update only scenario verdict fields after each debrief and deduplicate any symptom
  against `docs/qa/bugs/` before filing a content-addressed bug.
- Run the store-related runtime E2E lane and the one final full `make verify` required by the program.
- Write `docs/qa/reports/2026-07-12-store-redesign.md`, materialize the disposable `docs/qa/state.csv` view, and
  cite `teardown.json` with `"clean": true` after the mandatory teardown.

## Completeness

- [x] The changed journey has a Mermaid flow, true end state, and abandonment paths for legacy and ahead homes.
- [x] Every in-scope journey has at least one immutable charter with a canonical persona, exactly one tour, and
  a 30/60-minute box.
- [x] Both Task 05 scenarios are owned by a charter; the broader RT-001 status canary is reset to `untested`.
- [x] Fresh boot, legacy refusal/remediation, ahead refusal, direct-open CLI, shared-file smoke, HTTP, UDS, CLI,
  and the adjacent model-catalog persistence path have concrete entry points and expected observables.
- [x] Only the living `docs/qa/` tree is used; no legacy case/report artifact, fake UI journey, or planning-time verdict was introduced.

**Handoff:** Task 07 runs CH-database-refusal-recovery, CH-daemon-schema-parity, and
CH-model-catalog-storage-canary in that order against the same isolated cycle envelope, then publishes the dated
execution report and teardown evidence.
