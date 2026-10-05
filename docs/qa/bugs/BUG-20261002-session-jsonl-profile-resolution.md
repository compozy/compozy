# BUG-20261002-session-jsonl-profile-resolution: Session JSONL omits the selected profile

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-operate-profiles, read an empty profile-scoped work catalog
- **Scenarios:** ET-profile-selection-precedence
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

1. Select a profile with no sessions.
2. Run `compozy --profile default session list --all-workspaces -o jsonl`.
3. Repeat with a profile that owns sessions.

**Expected:** The documented `profile_resolution` frame identifies the selected profile and source
before any rows, including when the catalog is empty. The final page record remains usable.

**Actual:** The empty result contains only a page record. The populated result contains session
rows and a page record, without resolution provenance.

## Evidence

- `docs/qa/evidence/2026-10-02-untested/profile-empty-jsonl.json`
- `docs/qa/evidence/2026-10-02-untested/profile-empty-json.json`
- `docs/qa/evidence/2026-10-02-untested/profile-populated-jsonl.json`

## Fix

- **Root cause:** The session-list JSONL formatter writes rows and continuation metadata but never
  emits the profile resolution already recorded by the command. Existing empty-list coverage
  exercises the profile catalog rather than the session work catalog.
- **Owning invariant / suite:** Session-list JSONL emits one resolution frame before scoped rows
  or an empty page, and keeps aggregate ownership and continuation records. The canonical owner
  is `TestSessionListProfileReadScope` in `internal/cli/session_test.go`.
- **Correction:** The existing session-list formatter writes its recorded profile frame before
  rows and pagination. Aggregate output retains its existing single frame.
- **Regression proof:** Both empty and populated scoped cases failed before the correction.
  Focused session-list tests pass afterward, and the complete CLI race suite passes (12.102s).
  The continuation test now accounts for the documented prefix while retaining its health,
  cursor, count, limit, and `has_more` assertions.
- **Live replay:** The rebuilt CLI emits exactly one profile frame for empty `default`, populated
  `studio`, and aggregate reads. Page records remain last. See `profile-jsonl-fixed-*.json`,
  `session-jsonl-red.log`, `session-jsonl-green-recheck.log`, and `cli-race.log` in this cycle's
  evidence directory.
- **Conventions:** The file-wide heuristic reports 13 pre-existing subtest-shape findings outside
  the changed cases, compared with 14 at the base commit. Both changed tests use canonical
  subtests. Baseline and current checker transcripts are retained; unrelated tests are untouched.
- **Commit / full scenario replay:** pending; selection fallback and machine-command parity remain.

## Completed verification

- **Fix commit:** `42db6579f`.
- The original persona completed the affected fresh replay; `make gate` passed all affected lanes.
- Current-head PR CI and the remaining full-sweep journeys are separate delivery work, still in progress.
