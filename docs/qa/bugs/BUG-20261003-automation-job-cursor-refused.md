# BUG-20261003-automation-job-cursor-refused: A jobs cursor cannot read the next page of the same query

- **Status:** verified
- **Fix commit:** f38ee2ec19e738f6f27836d7c19c9fc34f1d7d40
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-24, continue through the automation job catalog
- **Scenarios:** TA-052; TA-056
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

The isolated profile-recovery lab has 64 enabled monthly edition jobs, all owned by archive-editions
and scheduled for November 1. They were authored through the public CLI before the archive walk.

1. Run `compozy automation jobs --all-profiles --json`.
2. The response returns 50 jobs, total 64, has_more true, and an opaque next_cursor.
3. Repeat the same command with `--cursor` carrying that exact returned value.
4. The CLI refuses it: `cursor does not match this query`.
5. A fresh first page followed immediately by its cursor reproduces the refusal without mutation.

Expected: the unchanged query can consume its own continuation and read the remaining 14 jobs.
Actual: the public continuation is unusable. Other transports and trigger pagination are not yet
qualified by this finding; inspect their owning boundary during the separate fix loop.

## Evidence and scope

docs/qa/evidence/2026-10-02-untested/profile-archive-crash-setup-{jobs-before,
jobs-before-page-two,cursor-first,cursor-second,jobs-help}.json. The original creation receipt
retains all 64 definitions. This was discovered during preparation between persona walks; it is
not a completed Bruno charter. No database query, fabricated cursor or product-code mutation
established the reproduction. The archive recovery branch remains independently testable via its
complete public plan and documented larger page limit; that does not count as a pagination fix.

## Investigation

The cursor fingerprint includes the profile ID/all-profiles read scope. CLI query parsing validates
that fingerprint with a zero read scope before transport selection applies. The shared HTTP/UDS
parser and native-tool input parser repeat the same ordering error: their caller assigns the
resolved scope only after validation. Extension Host already supplies the profile before validation.
The canonical automation catalog and persisted store correctly bind cursors; preserve those checks.

Repair the boundary order: HTTP/UDS and native parsers receive the resolved read scope before full
query validation. The CLI keeps field parsing and forwards the opaque cursor to the authoritative
daemon validation instead of deriving an incomplete fingerprint. No cursor format, ordering,
filter, ownership, or public wire shape changes.

Invariant, owner, canonical suite: CLI jobs/triggers preserve a server-issued profile-bound cursor
with its selected filters. Existing TestAutomationJobsListAndUpdateCommands and
TestAutomationAdditionalCommandsAndQueries in internal/cli/automation_test.go own the transport
boundary. Their cursor fixtures currently omit the real profile scope, hiding this regression; bind
those fixtures to default and preserve every existing assertion. Real CLI/HTTP/UDS/native/browser
replays will verify the full query and refusal behavior; no duplicate test file is needed.

The same failure is now independently reproduced over HTTP and native invocation, and in the
CLI trigger catalog populated with 64 disabled editorial review triggers. Receipts:
automation-cursor-http-red-{first,second}.json, automation-cursor-native-red-{first,second}.json,
and automation-cursor-triggers-red-{first,second}.json. The two existing CLI tests fail with
realistic profile-bound fixture cursors before the repair and pass afterward. The affected
CLI/core/native/model race cohort passes (automation-cursor-{cli-red,focused-green}.log).
The test-shape checker reports the same ten pre-existing direct-assertion suites before and
after the fixture-only change; no new test-shape finding or assertion change is introduced.

## Final source replay

Bruno's fresh CH-038 slice verifies both 64-row catalogs on the rebuilt real daemon. The CLI
aggregate pages return 50 + 14 exact, unique IDs; HTTP first pages and UDS continuations agree.
Native jobs/triggers return the same complete populations using their own profile-bound cursors.
Changed profile/filter reuse is refused; malformed cursors still receive the canonical daemon
validation error (CLI exit 65). The helper initially expected the former local error exit 1; that
driver assumption is recorded separately and no production refusal is relaxed.

Profile-scoped combinations of workspace, dynamic source, disabled state, search and trigger event
continue across four 17-row pages. Chrome's Load more reaches all 64 jobs and triggers; reloading
restarts at 50 and continues to 64 again. Both screenshots were inspected, the 12-frame recording
is stopped, and no product source was read during the walk. Evidence:
automation-cursor-bruno-ended.json, automation-cursor-bruno-*.json,
automation-cursor-{jobs,triggers}-all.png and automation-cursor-production-build.log.

This verifies the cursor repair, not the entire TA-052/TA-056 contracts. Their remaining CRUD,
read-only source and Loop binding legs stay Pending in the report. The required gate and commit
proof are recorded separately before delivery.

The first delivery gate catches formatter drift in the new multi-argument signatures. The owning
golangci-lint formatter is applied; this is formatting only, with no behavior or expectation change.
The subsequent gate uses the final formatted source.
