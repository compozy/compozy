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

Three 30-second trials follow five-second warmups, alternating baseline/head
order. Every trial starts from a fresh copy of the same fixture. The stop
boundary stops admission and waits for requests, health flush and checkpoints;
it never cancels accepted work to manufacture clean latency results. Each
request has a two-second deadline. Failed operations and long-lock/contention
messages remain in the output and fail head acceptance.

The modernc commit hook counts write commit attempts by core, telemetry and
session database. Operation errors and rollback hooks are reported separately;
only an error-free run supports interpreting those hooks as successful commits.
A separate pair of 15-second runs uses `strace` to sum successful `write` and
`pwrite64` return values for `.db` and `-wal`. This measures actual application
file-write volume, including clean shutdown, rather than file-size growth. It
excludes shared-memory mmap dirtying and filesystem/device amplification.
Traced latencies are excluded from the p99 comparison.

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
