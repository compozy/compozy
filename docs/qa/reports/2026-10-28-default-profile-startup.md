# QA Run Report — 2026-10-28 — Default profile startup

- **Scope:** Issue #999; preserve the permanent default profile's customized presentation through daemon restart.
- **Cadence tier:** targeted
- **Build:** `fix/issue-999-default-profile-presentation`, base `7ccc00670d1e278515049d6ff7ba8b3c00b7ee25` plus scoped fix.
- **Environment:** macOS arm64, isolated home, HTTP port and Unix socket; real compiled daemon and public CLI/API.
- **Status:** closed

## Session Matrix & Results

| Journey / scenario | Persona | Tour | Status |
|---|---|---|---|
| J-operate-profiles / ET-profile-identity-appearance-catalogs: default presentation restart | Dora, power user | Persistence | Pass |

## Session charter

Dora wants her default profile to keep its chosen green color and apple symbol when she restarts Compozy. Start a fresh isolated daemon, update `default` through the CLI, stop and start the daemon, and independently read the profile through HTTP. Repeat with an emoji through PATCH and verify through CLI after another restart. Check that renaming or archiving the permanent default is still refused. No browser or provider inference is required for this backend startup defect.

## Session Debrief

The real daemon reached readiness with the customized default profile after restart. The
CLI update accepted `#4cb782` and `apple`; HTTP `/api/profiles` returned those same values
and the permanent zero-ULID ID after the next start. PATCH then replaced the icon with
`🍎`; after another start, both CLI list and HTTP returned the emoji, null icon, original
color, name `default`, and active state. Rename, archive, and delete commands all refused
the permanent profile without changing it. The final daemon stop succeeded.

The evidence collector needed two format corrections: HTTP profile catalog returns an array,
and CLI list omits IDs. These were collector exceptions, not product failures. The same
isolated database was retained, and successful public readbacks were recorded after resuming.
No database was edited directly. The untouched `open-design` profile supplied by bundled
resources remained present with its original identity throughout.

No product paper cuts or additional defects were found in this bounded CLI/API journey.

## Regression evidence

The four presentation cases (color, icon, both, emoji) in the owning globaldb migration/open suite failed before the production fix with the reported permanent-identity error after closing and reopening the database. The existing invalid-identity test now corrupts the immutable name instead of treating supported color customization as corruption. Missing and archived identities remain negative cases.

## Coverage limits

This targeted run does not re-certify the historical full icon-picker/browser scenario or the packaged desktop bootstrap. It verifies the daemon readiness boundary that blocked desktop startup.

## Verification and evidence

- `CGO_ENABLED=1 go test -race ./internal/store/globaldb -run TestOpenGlobalDBAppliesGlobalMigrationsAndEnablesWAL -count=1`: pass (15.881s).
- `go build` of the real daemon: pass.
- Changed owning test function passes the test-shape checker. Whole-file heuristic findings are pre-existing unrelated cases (plus the TestMain false positive); they were not rewritten.
- Live CLI/API transcript and journey log are retained in the isolated lab. Public evidence is summarized here without machine-specific paths.
- Scoped normal/integration lint passed with zero issues. The initial broad local race run encountered context deadlines in two untouched historical migration Close operations. The subsequent delivery instruction assigns broad validation to CI; no deadlines or assertions were changed. Exact-head CI/reviewer results are recorded in the PR.
- Canonical targeted lab teardown completed with `clean: true`.

## Final status

The targeted startup persistence journey passes. The existing database requires no reset,
repair, migration, or presentation rewrite. Full browser appearance coverage remains the
historical scenario's separate evidence; this run changes no rendering behavior.
