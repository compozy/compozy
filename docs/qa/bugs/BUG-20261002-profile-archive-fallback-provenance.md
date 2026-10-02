# BUG-20261002-profile-archive-fallback-provenance: Archived profile fallback loses its reason

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-operate-profiles, resolve a remembered profile after archiving it
- **Scenarios:** ET-profile-selection-precedence
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

1. Remember an active profile for a workspace with `compozy profile use editorial`.
2. Stop its sessions and archive it through `compozy profile archive editorial`.
3. Read `compozy profile current -o json` in the same workspace.
4. Read `/api/profiles/selection` over HTTP and UDS, then unarchive the profile.

**Expected:** The acting profile is `default`, source is `default`, and the documented
`archived_remembered_fallback` note explains why. The remembered choice survives unarchive.

**Actual:** CLI reports `default` with source `remembered` and no note. Both API transports
replace the archived name with `default` without fallback provenance. Unarchive restores the
original name, confirming that the durable selection was retained.

## Evidence

- `docs/qa/evidence/2026-10-02-untested/profile-fallback-cause-current.json`
- `docs/qa/evidence/2026-10-02-untested/profile-fallback-cause-http.json`
- `docs/qa/evidence/2026-10-02-untested/profile-fallback-cause-uds.json`
- `docs/qa/evidence/2026-10-02-untested/profile-fallback-cause-selection-restored.json`

## Fix

- **Root cause:** The API selection projection intentionally substitutes the effective default
  profile for an archived remembered choice, but discards the archive state. The CLI therefore
  treats that effective default as an explicit remembered selection.
- **Correction:** The existing effective `profile` field is preserved and optional `note` carries
  fallback provenance. CLI resolution consumes it without changing durable selections.
- **Verification:** API and CLI cases failed before repair, then passed. Complete race suites
  passed: API core 19.518s; CLI 15.282s. Root codegen regenerated OpenAPI and Web types.
  The rebuilt daemon and CLI preserve the note through HTTP/UDS, `profile current`, and empty
  session JSONL. Explicit selection still wins; unarchive restores the original remembered
  profile, and all three pre-existing sessions retain their owners.
- **Replay evidence:** `profile-fallback-fixed-*.json` and `profile-fallback-race.log` under the
  same evidence directory.
- **Owning invariants / suites:** `TestGetProfileSelectionsReturnsOneStableShape` in
  `internal/api/core/profiles_selection_test.go` owns API fallback provenance and the unchanged
  effective profile. `TestProfileCommandOutputContract` in `internal/cli/profile_test.go` owns
  the CLI resolution source and note.
- **Commit / delivery gate:** pending.
