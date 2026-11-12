# Telemetry WAL reclamation

Issue: #1465. Scope: temporary synthetic databases only; no operator database inspected.

## Confirmed mechanisms

Periodic PASSIVE maintenance already exists. A pinned reader prevents complete backfill;
sustained writes then append required WAL frames. Releasing that reader allows backfill,
but PASSIVE does not reduce the retained file allocation. The default journal size limit
is -1 (unlimited). New writes can reuse the allocation without making the file smaller.
These mechanisms reproduce the report; the exact reader lifetime or cause of the reported
2 GB operator WAL has not been established and is not inferred from file size alone.

A synthetic SQLite control used 128 committed 64 KiB inserts with FULL synchronous mode,
auto-checkpoint disabled, and one retained read transaction:

| Phase | WAL bytes | Frames | Backfilled |
| --- | ---: | ---: | ---: |
| Reader held, PASSIVE | 9,562,552 | 2,321 | 0 |
| Reader released, PASSIVE | 9,562,552 | 2,321 | 2,321 |
| RESTART | 9,562,552 | 2,321 | 2,321 |
| Small commit after restart, PASSIVE | 9,562,552 | 3 | 3 |
| TRUNCATE | 0 | 0 | 0 |

TRUNCATE with the reader held returned busy immediately with busy_timeout=0. Integrity
was `ok`; closing the final connection removed the WAL. This control used Python's
SQLite; the owning Go suites separately exercise the shipped modernc SQLite driver.

## Implementation and tradeoffs

Only telemetry opts into reclamation above 16 MiB on the existing one-second maintenance
worker. PASSIVE remains the normal checkpoint. Above the threshold, TRUNCATE uses the
existing maintenance handle with busy_timeout=0: reader or writer contention defers
reclamation instead of waiting on snapshots. It runs independently of ingestion's
single writer connection and leaves synchronous=FULL and commit acknowledgement intact.
SQLite owns checkpoint locking and preservation of required frames. A successful
`wal_checkpoint(TRUNCATE)` holds SQLite's write lock even on this separate connection.

The threshold avoids truncating a small reusable WAL every second. It is an internal
maintenance policy, not a new config contract or a hard disk bound. A live reader can
still require more than 16 MiB; release is required for reclamation. Filesystem sync or
truncate latency is not bounded by busy_timeout. No hard storage-latency guarantee is
claimed. The worker retains its cancellation/join ownership and passive shutdown path.

Setting journal_size_limit on ingestion would defer shrinkage until a writer reset and
put truncation on that writer; it cannot reclaim idle allocation or override readers.
RESTART permits reuse but does not itself shrink allocation. The selected path reclaims
idle excess allocation on maintenance without a waiting RESTART/TRUNCATE retry loop.
See [SQLite checkpoint modes](https://sqlite.org/pragma.html#pragma_wal_checkpoint) and
[journal size limit](https://sqlite.org/pragma.html#pragma_journal_size_limit).

## Real SQLite verification

- `go test ./internal/store ./internal/store/globaldb -run 'TestStoreSQLiteHelpers|TestGlobalDBTelemetryDelivery' -count=1 -v`: passed.
- Shipped-driver reader fixture: 4,783,352 bytes, 1,161 frames, zero backfilled while
  pinned; 4,795,712 bytes, all 1,164 frames backfilled after release. Opted-in maintenance
  then truncated to zero, preserved 67 committed rows and `integrity_check=ok`, and kept
  a subsequent small WAL reusable. Occupied reader and writer maintenance attempts
  returned within their one-second contexts; reader snapshot contents stayed unchanged.
- Telemetry ingestion fixture: 20 acknowledged 1 MiB payloads with a pinned observer;
  periodic maintenance reclaimed 22,441,672 bytes to 12,392 bytes after reader release
  (statistics maintenance may append a small WAL). FULL remained enabled. Clean close
  joined maintenance; reopen retained all rows and payload bytes with integrity `ok`.

The test-shape heuristic passes for store helpers. Its globaldb findings concern six
unchanged pre-existing top-level tests; the added telemetry subtest follows canonical shape.
Local full suites, race suites, E2E, and broad gates are CI-owned and were not run.
No schema, data migration, CLI/HTTP/UDS/native tool, hooks, extension, config, Web, or
site change is required. Database paths, profile/workspace ownership, and existing
read semantics are unchanged. The PR owns the cross-surface impact audit.
