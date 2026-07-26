# QA Run Report — 2026-07-12 — store-redesign

- **Scope:** Goose/Atlas/sqlc hard cut, daemon-global schema diagnostics, incompatible-database refusal, and the adjacent model-catalog persistence canary
- **Cadence tier:** targeted
- **Build:** `04ea6b39` plus the contained Task 07 fix loop · **Environment:** isolated lab, CLI/HTTP/UDS only
- **No-UI rationale:** the change adds structured schema diagnostics and database-open behavior without a rendered
  control; runtime E2E plus CLI/HTTP/UDS parity owns the user contract.
- **Started:** 2026-07-12T14:48:24Z · **Ended:** 2026-07-12T15:57:19Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
| --- | --- | --- | --- |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-database-refusal-recovery |
| Ada | Power User | desktop / wifi-fast / en-US | CH-daemon-schema-parity; CH-model-catalog-storage-canary |

## Flows in Scope

- `J-operate-daemon-schema` — start AGH without silently rewriting incompatible alpha state and inspect the exact daemon-global schema state (`../journeys/J-operate-daemon-schema.md`)
- `J-20` — inspect model-catalog refresh/status persistence as the adjacent storage canary (`../journeys/J-20-catalog-curation-agent-surfaces.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | CH-database-refusal-recovery | J-operate-daemon-schema / RT-refuse-legacy-database; RT-refuse-legacy-cli-open; RT-refuse-ahead-database | Bruno | Garbage Tour | Pass | | |
| 2 | CH-daemon-schema-parity | J-operate-daemon-schema / RT-inspect-schema-streams; RT-preserve-shared-schema-isolation; RT-001 | Ada | Feature Tour | Pass | | |
| 3 | CH-model-catalog-storage-canary | J-20 / MS-043; MS-044 | Ada | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-database-refusal-recovery — Bruno

- **Ran:** 2026-07-12T14:51:42Z → 2026-07-12T14:58:02Z (box respected: yes)
- **Findings:** Legacy and ahead daemon starts exited before readiness; all three local direct-open families
  preserved the typed Go refusal and both fixture digests remained byte-identical. The captured direct-open output
  was plain text; round-4 review later identified the missing single-document JSON boundary and reset these verdicts.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-refuse-legacy-database → pass; RT-refuse-legacy-cli-open → pass;
  RT-refuse-ahead-database → pass
- **Paper cuts:** The refusal copy is intentionally destructive in tone, but it names the exact path and next
  action; the site guidance supplies the preserve/move option. No sharp friction remained.
- **Surprises:** A long fixture path exceeded the macOS UDS path limit after migrations succeeded. The invalid
  environment attempt was discarded and recovery was re-walked with the bootstrap-provided short home.
- **Suggested next charter:** Re-run after any migration-engine or database-path change.
- **Tour/edges attempted:** legacy daemon retry; extension, MCP-auth, and vault-backed provider-auth direct opens;
  checksum before/after; preserve-and-fresh-start recovery; ahead daemon and direct-open refusal.

### CH-daemon-schema-parity — Ada

- **Ran:** 2026-07-12T14:53:47Z → 2026-07-12T14:56:10Z (box respected: yes)
- **Findings:** No divergence. CLI, HTTP, and UDS produced identical ordered schema-stream arrays before and after
  restart; global and memory public reads remained usable.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-inspect-schema-streams → pass; RT-preserve-shared-schema-isolation → pass; RT-001 → pass
- **Paper cuts:** none
- **Surprises:** The broad status health is degraded on a no-provider lab because bundled agent commands are
  intentionally unconfigured; schema/persistence health remains `ok` and the condition is unrelated to this journey.
- **Suggested next charter:** Keep this as the migration/status release smoke.
- **Tour/edges attempted:** immediate post-readiness status; three-surface comparison; repeated read; empty memory
  page; default workspace read; stop/restart; three-surface comparison and domain reads after restart.

### CH-model-catalog-storage-canary — Ada

- **Ran:** 2026-07-12T14:55:00Z → 2026-07-12T14:56:10Z (box respected: yes)
- **Findings:** No persistence divergence. All 61 source statuses remained structurally valid; 53 succeeded and
  eight unavailable live sources retained provider/source context, stayed stale, and exposed no secret value.
- **Bugs filed/updated:** none
- **Scenarios settled:** MS-043 → pass; MS-044 → pass
- **Paper cuts:** none
- **Surprises:** The default catalog supplied deterministic failing live sources, so the error-context/redaction
  branch was exercised without external credentials.
- **Suggested next charter:** CH-031 remains the full catalog-curation owner; retain this narrow canary for store changes.
- **Tour/edges attempted:** provider-less refresh; provider-less status; mixed success/failure sources; optional
  timestamp omission; repeated status; daemon restart; normalized source ownership/state comparison.

## What Was Fixed

The release gates exposed five contained technical regressions, all fixed and covered by their existing owning
suites:

- The Goal E2E ACP fixture now advertises the configured deterministic judge model, preserving real model-option
  negotiation instead of bypassing it.
- HTTP, UDS, and CLI integration bridge fakes now delegate the current bounded route-count and secret-binding
  batch contract to the real store.
- `globaldb.ListEventSummaries` no longer queries the memory-owned `memory_events` table; Observe performs the
  cross-stream aggregation through its canonical memory event source.
- Heartbeat sqlc nullable mapping preserves authored body whitespace, including the final newline required for
  byte-identical rollback.
- The bridge bounded-metrics test stops its worker before directly arranging queue state, eliminating a scheduler
  race without changing broker production behavior.

No persona-visible defect remained and no `BUG-NNNN` entry was required.

Per the tracker-impact rule, the gate fixes reset out-of-charter scenarios RT-036 (Heartbeat rollback) and MS-049
(logs/event aggregation) to `untested` for the next QA cycle. This cycle does not claim a post-teardown persona
retest for those adjacent surfaces.

Implementation peer review later strengthened shared-file legacy detection so either migration stream refuses
either pre-Goose marker before mutation. Because that cross-stream combination was not walked in the completed
persona session, RT-refuse-cross-stream-legacy-marker is registered as `untested` for the next targeted cycle;
the existing eight pass verdicts remain scoped to the evidence they actually exercised.

The next review rounds also routed every read-only session consumer through the session stream preflight and
replaced single-file recovery guidance with a cold move of the complete containing `AGH_HOME` or workspace `.agh`
family. Those changes landed after teardown, so RT-refuse-legacy-session-database is registered as `untested` and
the three previously passed refusal scenarios are reset to `untested`; no new persona verdict is claimed by this
report for the changed recovery copy.

Round-4 review then fixed the direct-open boundary itself: provider-auth, extension, and MCP-auth now emit one JSON
error document with `legacy_database` or `schema_ahead`, surface, canonical path, and whole-family remediation,
without a preceding migration log. That behavior also landed after teardown, so the three refusal scenarios remain
`untested`; their earlier log files are historical byte-preservation evidence, not evidence for the new JSON shape.

Round-5 review decoupled the shared memory schema stream from `memory.enabled`: a fresh memory-disabled daemon must
still report the migrated memory stream while keeping memory prompt, recall, and native-tool behavior disabled.
RT-migrate-memory-stream-when-disabled is registered as `untested`; this report claims no post-teardown persona run.

Before the round-5 checkpoint, the global declarative schema was decomposed from one 2,276-line file into 21 ordered
domain fragments consumed directly by Atlas, sqlc, embed, and the Goose-equivalence suite. Exact migration and
`atlas.sum` identities are preserved. This is an internal ownership/codegen refactor with no public behavior, so it
does not add or reset a QA scenario and does not alter any persona verdict.

## Paper Cuts

No sharp or dull paper cut survived the completed sessions.

## Runtime Errors Observed

- A discarded recovery attempt used a Unix socket path longer than macOS permits. It was an invalid lab path,
  not product behavior; the clean rerun used the bootstrap-provided short path and supplies every verdict.
- The first runtime E2E attempt exposed missing ACP model-option metadata in the deterministic Goal fixture; the
  rerun passed after the fixture co-shipped that contract.
- The next runtime E2E attempt exposed stale bridge integration fakes at compile time; the final runtime E2E passed
  after all three transport fakes implemented the current batch catalog surface.
- The first full verify exposed the obsolete cross-stream Observe query, Heartbeat body trimming, and a flaky
  bridge test arrangement. Scoped red/green runs passed before the fresh full gate rerun.

## Human Verifications Needed

None planned; the cycle deliberately avoids external OAuth and provider login.

## Decisions for a Human

None at run start.

## Experiential Lens Pass

| Journey | Usability | Accessibility | Perceived performance | Compatibility | Error recoverability | Production parity |
| --- | --- | --- | --- | --- | --- | --- |
| J-operate-daemon-schema | partial historical pass — schema status was structured; direct-open JSON landed after the session and is reset `untested` | pass for observed text/status JSON; the new error JSON awaits the next cycle | pass — local reads/refusals returned within the persona windows | pass for the scoped macOS CLI/HTTP/UDS runtime; no rendered UI claim | pass for byte preservation; changed recovery/error shape awaits retest | pass — current binary, real daemon, real SQLite files, no mocks |
| J-20 | pass — source ownership and failure state are explicit | pass — machine-readable text surface | pass — refresh/status completed without perceived stall | pass for the scoped CLI/runtime surface | pass — failed sources retain context, stale state, and successful neighbors | pass for persistence; external provider success is intentionally outside the charter |

## Learnings

- The cycle report was created with every matrix row Pending before the first runtime interaction.
- The strongest public proof is the normalized `schema_streams` hash matching across CLI, HTTP, and UDS before
  and after restart; direct database inspection is unnecessary for the persona verdict.
- Optional source timestamps are best observed as omitted fields, never null or SQL errors; normalized catalog
  state is stable even when boot refresh advances timestamps.
- Stream ownership must extend through observability queries: globaldb cannot retain a convenience UNION over a
  memory-owned table after the migration hard cut.

## Verification Gates

- `make test-e2e-runtime` — PASS (exit 0); evidence
  `/Users/pedronauck/dev/qa-labs/agh-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/runtime-e2e-final.log`,
  SHA-256 `26454d646cbf661037659fc3c44d95fd0d5c2e48a8b07189f076ea1119c6a2fc`.
- `go test -race ./internal/bridges ./internal/heartbeat ./internal/observe ./internal/store/globaldb -count=1`
  — PASS, 1,041 tests.
- `make lint` — PASS, zero issues.
- Fresh post-fix `make verify` — PASS (exit 0); evidence
  `/Users/pedronauck/dev/qa-labs/agh-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/final-make-verify-rerun.log`,
  SHA-256 `a44a9032de43e7af1b4fe4df4746eea79deeccf91031292c9690d525f8d1c4f7`.
- Fresh post-peer-review `make verify` — PASS (exit 0) on 2026-07-12T16:58Z after the stream-open,
  cross-marker, file-split, test-shape, and nil-receiver remediations.
- Fresh post-round-2 `make verify` — PASS (exit 0) on 2026-07-12T17:57Z after read-only session preflight,
  typed SQLite classification, public recovery guidance, official-skill, and QA tracker updates.
- Fresh post-round-3 `make verify` — PASS (exit 0) on 2026-07-12T18:48Z after whole-family runtime recovery copy,
  typed preset/task/workspace constraint mapping, and joined two-phase SQLite opener cleanup failures.
- Fresh post-round-4 `make verify` — PASS (exit 0) on 2026-07-12T19:53Z after immutable memory-v1 history
  repair, append-aware migration tests, structured direct-open diagnostics, cleanup-error joins, Notification SQL
  classification, and regenerated Daytona sidecars. This was a code gate, not a new persona QA run.
- Fresh post-round-5 `make verify` — PASS (exit 0) on 2026-07-12T21:13Z after unconditional memory-stream boot,
  Network sqlc ownership, residual cleanup-error joins, and normative/canonical contract synchronization. This was a
  code gate, not a new persona QA run.
- Fresh post-schema-decomposition `make verify` — PASS (exit 0) on 2026-07-12T21:45Z after Atlas/sqlc/embed/test
  consumers moved to the 21 global domain fragments. The gate passed 13,721 Go/race tests (two intentional helper
  skips), 3,290 web tests, 511 UI tests, 247 site tests, production build, zero-issue Go/Bun lint, and boundaries.
  Existing React `act(...)`, reduced-motion, and Vite chunk-size stderr warnings remain outside this persistence diff.
  This was a code gate, not a new persona QA run.
- Fresh post-round-6 `make verify` — PASS (exit 0) on 2026-07-12T22:32Z after declarative-source ADR alignment,
  cleanup-time Notification rollback contexts, joined Loop rows-close errors, and schema-fragment EOF cleanup. The
  gate passed 13,724 Go/race tests (two intentional helper skips), all Bun/typecheck/test/build lanes, codegen-check,
  zero-issue lint, production build, and package boundaries. These are internal contract/error-preservation changes,
  so no living QA scenario was added or reset and no post-teardown persona verdict is claimed.
- Lab teardown — PASS; `qa/teardown.json` reports `"clean": true`, no survivors, completed at
  2026-07-12T15:57:19Z.

## Final Status

**PASS for the completed persona session.** All three charter sessions and all eight original scoped scenarios
passed their then-recorded checks; runtime E2E and all recorded fresh full `make verify` gates are green. The tracker
honestly resets post-session recovery-copy, direct-open JSON, cross-marker, and session-read behavior to `untested`;
no persona bug remains open, and the isolated lab teardown is clean with no surviving processes.
