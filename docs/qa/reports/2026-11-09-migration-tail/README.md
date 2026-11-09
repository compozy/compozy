# Migration-tail deadline investigation

## Reported race failure

[Issue #1402](https://github.com/compozy/compozy/issues/1402) reports a
180-second deadline at statement 1 of migration 195. The per-test JSON artifact
from [the failing job](https://github.com/compozy/compozy/actions/runs/42523632526/job/128862529576)
separates fixture preparation from migration execution:

| Event | UTC timestamp |
| --- | --- |
| Ownership subtest resumes and creates its deadline | 21:24:48.271 |
| Prefix 107 finishes opening | 21:26:04.247 |
| Tail fails at the original deadline | 21:27:48.322 |

The prefix helper serializes fixture admission with `globalMigrationTestPrefixMu`,
copies a historical snapshot, and opens it using its own context. The caller's
three-minute context does not control that work, but previously started before it.
Preparation consumed approximately 76 seconds; the prefix's actual `store.Apply`
reported only 126 milliseconds. Approximately 104 seconds remained for the tail.
The error identifies where the shared deadline expired, not the cost of index 195.
Only this test opens the prefix immediately before migration 108 in this package.
The JSON logger can attribute process-wide log output to another active test, so
the prefix identity, rather than the log's enclosing test label, identifies this event.

The fixture seeds three historical automation runs. Neither fixture row volume nor
an expensive index build is established by this failure. Local runs of the unchanged
SQL completed successfully, including race-enabled runs. Starting the same
three-minute operation context after the fixture helper returns removes unrelated
fixture admission from the operation budget. All ownership, orphan preservation,
metadata, integrity and foreign-key assertions remain unchanged.

## Related integration timeout

[The release integration job](https://github.com/compozy/compozy/actions/runs/42591931638/job/129075513606)
timed out at the package's 45-minute limit. The basic lineage persistence/filter
case had already passed in 4.86 seconds. The unfinished notify-creator upgrade case
was runnable inside SQLite's rename/schema parser, called through the nontransactional
migration runner. Its stack does not show a database-lock wait.

That evidence does not establish the same fixture-budget defect. Full-pointer-check
migration cost and the package-wide deadline remain separate from the operation
context corrected here. This change does not claim to fix that integration timeout.

## Local evidence and limits

- The original two focused suites passed without race instrumentation in 4.634 seconds.
- An original race run passed in 61.660 seconds, with ownership at 32.27 seconds
  and the notify-creator upgrade at 57.56 seconds.
- A two-CPU original race run passed in 119.038 seconds, with ownership at 79.33
  seconds and the notify-creator upgrade at 111.86 seconds.
- A discarded SQL-batching experiment did not demonstrate improvement and is absent
  from the change. Shared-host timings are observations, not a throughput claim.
- The corrected two-CPU race run passed in 63.653 seconds: ownership 34.69 seconds,
  basic lineage/reopen 3.16 seconds, and notify-creator upgrade 59.57 seconds.
- Pinned golangci-lint v2.15.0 reported zero issues for `internal/store/globaldb`.
  The test-convention heuristic has identical findings on the original and corrected
  file; none concern the changed case.
- CI owns the full race shard and integration validation. The historical SQL,
  checksums, production migration runner, and three-minute operation timeout are unchanged.

This is a test-harness correction. Native tools, HTTP/UDS/CLI, hooks, config,
workspace/profile isolation, Web, public documentation, and `skills/compozy/`
have no behavior or contract changes. No user migration or runtime QA scenario changes
are required; the existing migration-tail and lineage suites exercise real SQLite.

Focused validation from the repository root:

```sh
CGO_ENABLED=1 GOMAXPROCS=2 go test -race -gcflags=modernc.org/...=-d=checkptr=0 ./internal/store/globaldb -run 'TestGlobalAutomationRunProfileMigrationTail|TestGlobalDBSessionLineagePersistsAfterReopenAndFilters' -count=1 -parallel=4 -v -timeout=5m
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --allow-parallel-runners --timeout=5m ./internal/store/globaldb/
```
