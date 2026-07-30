# BUG-20260729-loop-sidecar-lifecycle: Deleted Loops could restore stale config and editor positions

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-07 Loop config and annotations API, manage workspace-scoped sidecars
- **Scenarios:** TA-069
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

An agent could write annotations for a Loop that did not exist, receive a not-found response after
writing config for that same missing name, and later see both values appear when the name was
created. Deleting and recreating a real Loop also restored its previous config and editor positions.
The initial config response separately omitted `config` instead of returning the documented
`config: null`.

## Reproduction

- **Charter:** CH-untested-008-07-ada · **Tour:** Feature Tour
- **Environment:** isolated macOS lab / HTTP + UDS / en-US

1. Register two isolated workspaces and create the same uniquely named Loop in both.
2. Read config and annotations over HTTP and UDS before writing any sidecars.
3. Replace config and annotations in the first workspace through alternating transports and confirm
   that the second workspace remains unchanged.
4. GET and PUT config and annotations for a missing Loop name.
5. Delete the first Loop, read its config and annotations, then recreate the same name.

**Expected:** A known Loop with no override returns `config: null`; missing definitions reject every
sidecar read/write without mutation; delete permanently removes definition-owned config and
annotations; the other workspace remains isolated.
**Actual:** The initial response omitted `config`; annotation GET/PUT for a missing name returned 200;
config PUT wrote before its later 404; annotations remained readable after delete; recreating either
name restored the stale sidecars.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-loop-config-annotations-20260729-180052-857452-lab/qa-artifacts/qa/evidence/048-loop-config-annotations/initial-alpha-config-http.txt`
- `/Users/pedronauck/dev/qa-labs/compozy-loop-config-annotations-20260729-180052-857452-lab/qa-artifacts/qa/evidence/048-loop-config-annotations/missing-annotations-http-put.txt`
- `/Users/pedronauck/dev/qa-labs/compozy-loop-config-annotations-20260729-180052-857452-lab/qa-artifacts/qa/evidence/048-loop-config-annotations/post-delete-alpha-annotations-http.txt`
- `/Users/pedronauck/dev/qa-labs/compozy-loop-config-annotations-20260729-180052-857452-lab/qa-artifacts/qa/evidence/048-loop-config-annotations/post-recreate-alpha-config-http.txt`
- Red regressions: `red-config-null.txt`, `red-missing-config.txt`, and
  `red-sidecar-lifecycle.txt` in the same evidence directory.

## Fix

- **Root cause:** Config mutation did not resolve the definition before upsert; annotation reads and
  writes bypassed definition resolution entirely; delete removed only the file-backed definition;
  and the response DTO used `omitempty` on its nullable config field. The config and annotation
  tables therefore accepted detached rows keyed only by workspace and Loop name, and recreation
  reused them.
- **Correction:** Definition resolution now precedes every sidecar mutation/read. Delete stages the
  definition outside the scanned root, atomically removes config and annotations, publishes the
  removal, and restores both resources through a bounded detached rollback when publication fails.
  The response contract now requires a nullable `config`, and the 404 response set is co-shipped in
  OpenAPI and generated TypeScript.
- **Fix commit:** `351f3535`
- **Regression test:** `internal/loop/service_test.go`,
  `internal/daemon/loop_api_runs_test.go`, `internal/daemon/loop_resources_test.go`,
  `internal/store/globaldb/global_db_loop_api_test.go`, `internal/api/core/loops_test.go`, and
  `internal/api/spec/loops_test.go`.

## Verification

- Red-before evidence reproduced all four failure classes.
- The focused owner suites pass with `-race`, including real SQLite rollback and canceled-request
  reconciliation.
- `make codegen-check` passes.
- A fresh rebuilt-candidate replay passed every original HTTP/UDS branch and proved rejected
  missing-name writes leave no state after later creation. Delete/recreate returned `config: null`
  and empty annotations while the same-name neighboring workspace stayed unchanged.
- Focused Go lint passed for `internal/api/core`, `internal/api/spec`, `internal/loop`,
  `internal/daemon`, and `internal/store/globaldb`; the site Turbo lane passed 51 files and 248 tests.
- Replay evidence:
  `/Users/pedronauck/dev/qa-labs/compozy-loop-config-annotations-retest-20260729-20260729-183928-434217-lab/qa-artifacts/qa/evidence/049-loop-config-annotations-retest`.
- The original HTTP/UDS replay remained green and the correction shipped in `351f3535`.
