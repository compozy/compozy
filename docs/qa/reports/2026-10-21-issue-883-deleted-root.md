# QA Run Report — 2026-10-21 — Deleted session root

- **Scope:** Fork and continue an existing fork after its original root was deleted; preserve the source and conversation.
- **Cadence tier:** targeted
- **Build:** branch `fix/issue-883-fork-deleted-root`, working tree over `b1008af2627e92db4838c403501a661abd916978`.
- **Environment:** isolated bootstrap lab, dedicated runtime home and ports; host sessions excluded.
- **Started:** 2026-10-21T20:19:51Z · **Status:** completed

## Session Matrix & Results

| Charter | Journey / Scenario | Persona | Tour | Status |
|---|---|---|---|---|
| CH-session-derive-web (deleted-root slice) | J-14 / ET-web-session-fork-from-here | Bruno, desktop, en-US | Feature | Pass |
| CH-session-derive-web (deleted-root slice) | J-14 / ET-web-session-continue | Bruno, desktop, en-US | Feature | Pass |

## Session Debriefs

The isolated Web app created both a fork and a continuation with a different agent from a second-generation fork whose original root had been deleted. Both dialogs completed and opened new session windows without the missing-root error. Independent CLI reads confirmed both children use the surviving source as parent and root, retain historical depth 3, and preserve the source’s stored lineage. After restarting only the lab daemon, both operations succeeded again with two carried messages; the earlier children’s lineage remained intact. The original two messages came from a real Codex conversation.

The focused manager regression reproduced both failures before the fix and passed afterward with real SQLite and the race detector. Chrome desktop evidence was captured at 1920×963. No frontend code changed.

## Verification

- `CGO_ENABLED=1 go test -race ./internal/session -run 'TestDeriveLineageReadModel|TestCreate.*Lineage|TestCreateSystemSessionRecordsInternalProvenance' -count=1`: passed.
- CLI/daemon integration suites: passed (`CGO_ENABLED=1 go test -race -tags=integration ./internal/daemon -run '^TestDaemonE2ESession(Continue|Fork)CLI$' -count=1`, 74.978 seconds).
- `TMPDIR=/private/var/folders/.../T make gate`: passed all affected lanes; session tests passed in 418.727 seconds. The macOS linker emitted its existing `-bind_at_load` deprecation warning while building the lint tool; both lint configurations reported zero issues.
- Browser walk: fork and continue succeeded; isolated-daemon restart and independent lineage checks passed.
- Manifest-scoped lab teardown completed with `TEARDOWN_CLEAN=true`; no host sessions were touched.
- Initial gate failed two existing hook tests because macOS temporary paths differed by `/private`; both passed unchanged with canonical `TMPDIR`. The complete gate passed with that environment.

## Final Status

Targeted behavioral verification passed. Manifest-scoped teardown is clean. PR CI and external review are separate delivery gates.
