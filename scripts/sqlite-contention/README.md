# SQLite contention acceptance experiment

This harness owns issue #1227's reproducible store-level measurements. The
`SQLite contention acceptance` workflow runs the heavy experiment in CI.
It builds the **same harness source** against baseline
`d35c63c9a9f70806583d8595f0279f78c5dff84f` and the PR head, then runs them sequentially
on one Linux runner. It never reads an operator runtime home.

`seed` uses the production stores to create 30 sessions with 1,000 completed
turns each. `data.py` grows the baseline's retained summary ledger to at least
1 GiB using synthetic rows, all original indexes and enforced constraints. It
preserves a retired sequence high-water mark. The head's production open path
upgrades a copy. Streaming SHA-256 covers **every column of every summary**;
counts, surviving sequences, retired cursors, Observe results, integrity,
index pages/query plans and open/upgrade/reopen time are retained as JSON.

`load` runs 20 Hz transcript chunks, 5 Hz transcript reads, 4 Hz health,
2 Hz summaries and 2 Hz interactive state updates per session. A shared reader
requests the session catalog at 20 Hz. Every two seconds, 1,500 model rows and
90 process observations refresh. Model content changes once every three
refreshes, exercising both replacement and unchanged-content paths.
Each measured phase also sweeps the first six hours of retained synthetic
summaries (at least 20,000 rows), while the streams and interactive routes run.
The report records exact eligible/deleted counts. The whole sweep has a one-minute
deadline, with a stricter 30-second acceptance ceiling on every untraced head sweep; each interactive operation retains its two-second deadline. The sweep
must complete. Write holds retain the one-second budget, with the narrowly paired
COMMIT-pause decision described below.

Four 30-second pairs follow five-second warmups, alternating baseline/head
order equally (AB, BA, AB, BA). Every trial starts from a fresh copy of the same fixture. Each fixture
copy is fsynced before process launch so its bulk copy does not become work in
SQLite's first commit. The stop
boundary stops admission and waits for requests, health flush and checkpoints;
it never cancels accepted work to manufacture clean latency results. Each
request has a two-second deadline. Failed operations and long-lock/contention
messages remain in the output. They fail untraced head acceptance except for
the measured COMMIT pauses explained by their own baseline pair below. The checkpoint worker retains a 200 ms admission budget, but its
I/O runs under the owned worker lifetime so it can advance the WAL backfill
pointer. It has a dedicated connection and never reserves the foreground writer.
Shutdown cancels and joins it; non-deadline I/O errors remain warnings. The family lease still serializes
physical connection opens with checkpoint I/O, so a cold pool open can wait on a
slow sync until its own request deadline. A short SQLite deadline cannot interrupt
a filesystem sync already in progress; interrupting backfill does not provide
that cold-open guarantee. Final session shutdown also retains the family lease
through backfill and last-connection close, which can itself checkpoint. Caller
expiry bounds the close wait while owned cleanup finishes after an in-flight
physical open releases the lease; reopening must preserve the committed events.
Existing pooled reads and writes remain independent. Idle maintenance also schedules PASSIVE backfill on this worker; only its
TRUNCATE attempt stays on the writer with the existing 200 ms I/O budget. Oversized WALs
attempt TRUNCATE first (which also backfills), scheduling PASSIVE only on deferral.
A below-limit idle PASSIVE deferred by a family guard retries on the next activity
or close; no data or size guarantee depends on a new timer. Foreground deadlines and
other one-second write holds remain failures.

The modernc commit hook counts write commit attempts by core and telemetry
database. Session connectors own a separate driver, so session commit counts are
not instrumented. Operation errors and rollback hooks are reported separately;
only an error-free run supports interpreting those hooks as successful commits.
A separate pair of 15-second runs uses seccomp-filtered `strace` to sum successful `write` and
`pwrite64` return values for `.db` and `-wal`. This measures actual application
file-write volume, including clean shutdown, rather than file-size growth. It
excludes shared-memory mmap dirtying and filesystem/device amplification.
Traced runs are diagnostics only: ptrace changes scheduling and I/O enough to
exhaust deadlines in this workload. They cannot measure production throughput or
deadline compliance. Raw failures and retention counts remain in the artifact;
Diagnostic process, JSON and write-accounting failures are recorded separately
in `*-writes-error.json` and `results.json`, preserving normal acceptance.
`trace_comparable=false` marks byte rates for incomplete work as unsuitable for
efficiency comparisons. Untraced acceptance requires every operation's median
paired head/baseline rate ratio to be at least 0.95. Every head trial must contain a positive sample count for every compared
operation; missing head work cannot be hidden by a median. Runs with operation errors fail independently, so counts
cannot hide failed work. Rates and regressions are retained in the report and JSON;
latency and file-write comparisons remain visible alongside throughput.

The 5% relative budget covers the observed 0.6–2.0% median shared-runner/drain variation
without accepting a sustained 6% loss. Ratios are computed within each pair before
taking the median, retaining local runner conditions; four pairs balance run
order. Individual historical pairs varied by up to 6.3%; the median reduces
that sensitivity but can still fail if two of four pairs are slow. For fixed-count
work, 5% allows roughly 1.5 seconds of extra drain. This is an explicit noise
budget, not a statistical confidence claim. Missing or zero baseline samples
fail instead of silently reducing the paired sample count, including operations
observed only on head.
The [four-run investigation](../../docs/qa/reports/2026-11-03-sqlite-contention.md#ci-acceptance-investigation-2026-11-04)
records the evidence and limitations. The head
retention sweep is 7–10 times slower than baseline: bounded 64-row background
items prioritize interactive work, which has no errors on head. This intentional
#1241 scheduling trade-off now has its own 30-second acceptance ceiling; the
count-1 throughput metric cannot measure sweep duration. The baseline remains fixed deliberately to
retain the pre-change upgrade fixture and historical contention comparison.

COMMIT-pause acceptance also uses the four same-run pairs. Transcript writes
provide the shared-storage control on both binaries. For each pair,
the budget is `max(1000 ms, baseline transcript_write max_ms)`, with no added
relative margin. The median of the four head-max/budget ratios must be at most
`1`. An individual long-hold warning can pass only when it names either the session
`append session events and transcript` transaction or the telemetry
`coalesced background writes` transaction, has a callback below one second and a
COMMIT of at least one second, and its entire elapsed time fits **its own pair's**
budget. A large baseline in another pair or a favorable median cannot excuse that
warning. Missing/nonfinite/nonpositive tail samples and baseline transcript
operation errors fail calibration. Head operation errors, other warnings,
retention and upgrade correctness remain strict. `results.json` and `report.md`
retain every raw warning in JSON and every head warning with its individual
decision in Markdown, alongside paired maxima, budgets and ratios.

This is a conservative paired order-statistic decision, not a confidence interval
or a guarantee about individual production latency. The one-second floor retains
the original budget on fast runners. It accounts only for the transcript and telemetry COMMIT
signatures measured in #1324. The telemetry layout differs from the fixed baseline,
so the common transcript workload controls shared-storage variance rather than
comparing unlike telemetry transactions. Core holds, background callback holds
and checkpoint errors are not exempted. The [eight-run investigation](../../docs/qa/reports/2026-11-03-sqlite-contention.md#paired-transcript-commit-tail-investigation-2026-11-06)
records shared-runner fsync evidence and residual attribution limits.

The core database's checkpoint worker also owns WAL restart. A retained read
transaction prevents foreground commits from reusing a fully backfilled WAL.
Each maintenance tick advances that read mark with overlapping snapshots before
running PASSIVE outside writer admission. Backfill keeps advancing during busy
application writes. Restart is attempted at 1000 WAL frames, rather than every
tick, and idle maintenance never adds a frame. The restart obtains background
writer admission (at most 200 ms per attempt); after three deferrals it reserves
the next writer slot, cancelable by worker shutdown. It releases its read mark,
completes any remaining PASSIVE tail, and commits the existing `user_version`
unchanged. This writes one header page and synchronizes the new WAL header on
the maintenance connection before pinning a new read mark. Application writes
queue before `BEGIN IMMEDIATE` during restart I/O; existing reads use their
independent pool. Startup admission timeouts and a BUSY initial writer probe
defer priming to the worker. If an external writer takes the lock after that
probe, restart retains admission while retrying; cancellation of the startup
context can then abort database open. Long-lived external readers can defer
reuse as usual.

`synchronous=NORMAL`, the one-second maintenance cadence, shutdown backfill,
and every acceptance threshold remain unchanged. This moves the filesystem
pause out of application transactions; it does not guarantee a wall-clock
deadline for writes queued behind slow storage. The extra unchanged header-page
write per size-triggered restart and two read connections during snapshot
handover are the cost. A restart's remaining PASSIVE tail can also synchronize
the WAL and database while admission is held, before the new header sync;
queued writers can therefore span up to three sync barriers per restart.
No persisted version, schema,
application row, migration, or public contract changes. Session and telemetry
durability policies are unchanged. WAL salts in `TestStoreSQLiteHelpers` prove
the old foreground-restart control and maintenance-owned reuse over repeated
cycles; CI owns repeated paired-load and public daemon validation.

The runner's decision suite is `python3 -m unittest discover -s
scripts/sqlite-contention -p '*_test.py' -v`, also run by the workflow before the
experiment. It owns pairing, relative budgets, missing operations and strict
correctness rejection.

After a failed acceptance run, the runner preserves its result and replays the
head separately with `-runtime-trace <path>`. The Go execution trace records named
workload regions and scheduling, GC, lock and syscall events for diagnosis. It
is uploaded as `head-diagnostic.trace`; `go tool trace` can inspect it or produce
`-pprof=sync`, `-pprof=syscall` and `-pprof=sched` profiles. This replay never
replaces a measurement or turns the original failure into a passing job.

For a dedicated Linux lab, build the harness in each checkout, then run:

```sh
python3 scripts/sqlite-contention/run.py \
  --baseline /path/to/baseline-binary --head /path/to/head-binary \
  --baseline-sha <baseline-sha> --head-sha <head-sha> \
  --output /new/isolated/results-directory
```

The report and raw receipts are CI artifacts. Copy the inspected conclusions
into `docs/qa/reports/2026-11-03-sqlite-contention.md`. These measurements are
synthetic store workload evidence; the separate isolated daemon exercise owns
public HTTP/SSE/UDS/CLI behavior. Neither is a live-model latency claim.

The workflow's second job builds the daemon and the existing ACP fixture driver.
`qa.py` provisions one canonical `eng-qa-bootstrap` targeted lab, then runs the
opt-in `TestSQLiteContentionDaemonLoad` suite (`contentionload` build tag). Thirty
scripted ACP peers stream for at least 30 seconds while HTTP transcript/catalog
reads and session renames have two-second deadlines. Public UDS reads and CLI
status/Observe are captured before and after a clean restart. All accepted
transcript content must survive, health must remain readable, and no SQLite
exhaustion or one-second hold diagnostic is accepted. A forced process kill
fails the clean-restart assertion. Raw route samples and process logs are kept.

The canonical strict evidence auditor and exact manifest teardown command run
before a passing result; teardown must report `clean: true`. No operator home,
provider login, or live model is used. The heavy lab runs in CI, separately from
ordinary runtime tests. On a dedicated CI runner, reproduce with:

```sh
python3 scripts/sqlite-contention/qa.py \
  --binary /path/to/compozy --driver /path/to/acpmock-driver \
  --output /new/isolated/daemon-results
```

Review follow-up impact (#1241): checkpoint shutdown changes only internal Go
ownership and deadline handling. Native tool IDs, HTTP/UDS/CLI shapes, hooks,
extension contracts, config, workspace isolation and `skills/compozy/` are
unchanged; Web needs no update. The paired gate changes CI acceptance only.
Shutdown QA coverage is tracked in `RT-daemon-drain-admission`; heavy paired and
public daemon runs remain CI-owned.
