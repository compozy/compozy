# BUG-20261002-foreign-history-blocks-startup: One misplaced history prevents access to every workspace

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-operate-daemon-schema, step 3
- **Scenarios:** RT-session-event-owner-isolation
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

After restoring another workspace's history into one retained session directory, Bruno cannot start
Compozy or read the correctly owned sibling session. The ownership check protects the misplaced
database, but its refusal takes the entire daemon offline.

## Reproduction

- **Charter:** CH-session-event-owner-refusal · **Tour:** Garbage Tour
- **Environment:** isolated macOS lab, desktop / wifi-fast / en-US; CLI, HTTP, and UDS.

1. Create and stop two real sessions in separate registered workspaces.
2. Stop the daemon and preserve both complete session directories.
3. Copy beta's intact `events.db`, WAL, and SHM family into alpha's directory without editing rows.
4. Start the daemon, then try to read beta's unchanged history.

**Expected:** Alpha's foreign history is refused without changing any family byte; the daemon and
beta remain available. Restoring alpha's matching complete directory restores its history.

**Actual:** Startup fails before readiness with `upgrade retained session database` wrapping the
owner mismatch. A clean second start fails identically; beta is unreachable. All three misplaced
family digests remain unchanged.

## Evidence

- `docs/qa/evidence/2026-10-02-untested/session-owner-start-with-foreign-family.json`
- `docs/qa/evidence/2026-10-02-untested/session-owner-foreign-boot-clean-retry.json`
- `docs/qa/evidence/2026-10-02-untested/session-owner-sibling-unavailable.json`
- `docs/qa/evidence/2026-10-02-untested/session-owner-failed-walk-summary.json`

## Fix

- **Root cause:** The boot inventory upgrade added for retained schema migrations treats a typed
  per-session identity refusal as a fatal daemon-wide migration error. It returns before processing
  healthy sessions or publishing public listeners.
- **Repair:** Keep refused identity stores out of boot history processing, log the exact
  session/workspace refusal, and retain fatal handling for cancellation and migration failures.
- **Regression owner:** `internal/daemon/daemon_test.go`, `TestBootSessionRepair`; storage ownership
  and physical-family integrity remain owned by the existing SessionDB suites and real lab replay.
- **Fix commit:** `212d9aea1`.

## Verification

- **Retested:** 2026-10-02, Bruno through fresh daemon starts and CLI/HTTP/UDS reads.
- **Result:** The daemon starts with the foreign store present, each foreign history/events read
  refuses, and the healthy sibling's history remains exact. All supplied database/WAL/SHM hashes
  stay unchanged. Restoring the matching complete directory and restarting restores both original
  four-event histories. The documented daemon log retains the scoped refusal warning.
- **Checks:** Three new boot cases fail before repair and pass afterward; existing migration
  refusal and the cancellation case pass. The full SessionDB race suite passes, including its
  same-owner physical-open/ABA probes (storage fault injection, not a public timing maneuver).
  `make gate` passes after correcting two formatter differences. The test-shape check has the
  same 67 pre-existing findings and adds none.
- **Evidence:** `session-owner-fixed-refusal-summary.json`, `session-owner-restored-*.json`,
  `session-owner-daemon-refusal-log.json`, `session-owner-storage-race.log`, and
  `session-owner-gate-final.log` in the dated evidence directory. Strict lab audit passes and
  `session-owner-teardown.json` records a clean targeted teardown.
