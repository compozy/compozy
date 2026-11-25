# Contributing to CompozyOS

Thanks for helping build CompozyOS. This guide covers the basics — open an issue when something here is unclear.

## Before you build anything big

For new features, open an issue describing what you want to build **before** writing code. CompozyOS is in early alpha and the design moves fast — agreeing on direction first protects your time as much as ours. Bug fixes and small improvements can go straight to a pull request.

## Dev setup

You need Go 1.27.3+ (see `go.mod`) and Bun 1.4.0 (pinned in `.bun-version`).

```bash
bun install     # install JS workspace dependencies
make build      # compile the compozy binary
make test       # Go unit tests (race detector on)
make web-dev    # run the web UI locally
```

### Building from source

To compile a local `compozy` binary directly from a checkout:

```bash
git clone https://github.com/compozy/compozy
cd compozy
go build -o ./bin/compozy .
export PATH="$PWD/bin:$PATH"
```

Source builds are **development builds**: they run without the mandatory sign-in gate and without
Compozy Cloud features (`version.SignInRequired` is stamped only on official release builds). Official
end-user install methods (desktop app, verified installer, Homebrew, npm, and Linux packages) are
documented at [compozy.com/download](https://compozy.com/download).

## Quality gates

- `make gate` — runs the checks affected by your diff. Run it before every push.
- CI runs the fast lane (preflight, lint, typecheck, unit, build) for affected areas on pull requests. With `CI_MERGE_QUEUE=true`, heavy lanes defer to the merge queue; add `ci:full` to run them before queueing (label events and re-runs both honor it). CI changes and release PRs also select heavy checks early. An absent variable preserves today's opt-in policy. Explicit `CI_MERGE_QUEUE=false` selects the fallback: backend PRs automatically run the equivalent heavy checks; other PRs can still opt in.
- Every `merge_group` for `main` verifies the entire combined tree: fast checks, Go race unit shards, all eight full integration race shards, Windows and runtime/browser/desktop E2E. Require only `Verify (fmt, lint, unit, build)` and `E2E (combined lane)`, with identical names on PRs and merge groups. Both always report and reject failed, cancelled, missing or unexpectedly skipped dependencies. PR title/body and reviewer checks do not run on merge groups and must not become queue-required contexts.
- Queue-ready workflows do not activate the queue. Maestro applies the W6 PR's settings only after merge and Pedro's approval. Once active, enqueue reviewed PRs and let GitHub merge them after both aggregates pass; do not merge around a red queue. Release PRs keep their Release-owned integration run before queueing; the merge group runs integration itself because Release does not run on that event. Publishing and release branches are unchanged.
- `make gate-full` — optional local full verification when you want extra pre-push certainty.
- Lint is zero-tolerance: `make lint` (Go) and `bun run lint` (JS/TS) must pass with no warnings.
- A pre-commit hook formats staged files for you (oxfmt for JS/TS/YAML/Markdown, `make fmt` for Go).
- A pre-push hook checks format and lint on the files your branch changes, including type-aware Go lint on the changed packages. `COMPOZY_SKIP_PRE_PUSH=1` or `git push --no-verify` skips it, and it skips itself in CompozyOS agent sessions, whose checks run in CI.

CI retries failed Go tests once (at most five failures per invocation) and failed Playwright
scenarios once. A recovery emits a `FLAKY` annotation and a JSON receipt; a second failure
still fails the job. A failed test that skips or disappears on retry also fails reporting; only a
passing retry counts as recovery. The daily Go test census updates one rolling seven-day flake ledger
from structured artifacts, without downloading job logs. Local runs do not retry.

E2E-023 runs in the separate nightly performance census, with five complete, unretried
samples and the unchanged 50 ms switch/peer budget applied to each metric's median.
It also runs on PRs explicitly labeled `ci:full`, where its result is informational:
it is outside both merge-gating aggregators and is not a required check. A rejected
census creates or updates one performance tracking issue and retains the raw samples;
passing a later run does not automatically close that investigation.
The 500 ms restore assertion and functional assertions still apply to every sample.
It also stays outside Release publishing dependencies. Run its focused local equivalent
with `COMPOZY_E2E_PERF=1 make test-e2e-web` when local E2E execution is authorized.

### Remote Computers browser scenarios

`make test-e2e-web` builds the `remotetest` harness in `magefiles/e2e.go`.
Each Remote Computers Playwright test selects a scenario by name. The fixture starts
the binary, reads its JSON manifest, uses the controller's web URL as Playwright's
base URL, and sends SIGTERM after the test. CI provides Postgres and uses the Ubuntu
runner's Docker and OpenSSH for the systemd container shared with the Go SSH suite.
For a local run, set `COMPOZY_CLOUD_TEST_POSTGRES` to a disposable Postgres URL whose
user can create schemas. The SSH scenario requires Linux with a Docker bridge.

To add a scenario, extend `prepareScenario` in
`internal/testutil/remotetest/cmd/remotetest/scenarios.go`. Use the shared cloud,
identity provider, and daemon startup, and arrange the prerequisite state through
the CLI or HTTP/UDS APIs. Publish the controller, computer and device references
the scenario needs in the manifest. Add the scenario name to the Playwright fixture
and select it with `test.use({ scenario: "…" })`. Interact with the public UI or CLI
in the test. The harness owns the isolated homes, sockets, schema and containers.

### Merge latency and revisit policy

`scripts/ci-merge-latency.py` makes one paginated GraphQL list call through `gh`.
It requests only merged PR metadata and readiness/queue timeline events, with no
per-PR check or log requests. Run it from an authenticated repository checkout:

```bash
python3 scripts/ci-merge-latency.py \
  --since 2026-11-04T18:44:57Z --split-at 2026-11-05T04:01:40Z \
  --until 2026-11-17T20:30:00Z
# After activation: use the actual activation timestamp as --since, then a
# rolling seven-day --since with an explicit --until for each later comparison.
python3 scripts/ci-merge-latency.py --since <seven-days-ago-UTC> --until <now-UTC>
```

The metric starts at the final ready-for-review event, or creation for a PR that
was never converted to draft, and ends at merge. Queue time sums all observed
enqueue-to-dequeue/merge intervals, including failed/retried entries. No queue
event means unavailable/not queued, not a zero-minute sample. The output reports
sample counts and exclusions; truncated timelines are excluded explicitly, and
incomplete pagination or the 1,000-result search cap fails the report. Narrow
the window if necessary. p90 uses nearest rank; median averages the two middle
samples for an even population. Cohorts use merge timestamps and exclusive ends;
PRs crossing a policy boundary retain their full duration. Historical comparison
is observational, not proof that the queue caused a latency difference.

The initial W6 snapshot (retrieved November 17, 2026 at 20:24 UTC) is:

| Merge-time cohort (UTC)                 | PRs | Median ready/open to merge |        p90 | Queue-covered PRs | Median / p90 queue time |
| --------------------------------------- | --: | -------------------------: | ---------: | ----------------: | ----------------------: |
| Queue: Nov 4 18:44:57 to Nov 5 04:01:40 |  19 |                 158.15 min | 334.33 min |                 9 |       30.00 / 80.52 min |
| No queue: Nov 5 04:01:40 to snapshot    | 248 |                 121.15 min | 786.10 min |                 2 |       48.67 / 57.30 min |

These boundaries are the merge times of #1282 and the queue-removal commit
`708807765`. The query's upper bound was 20:30 UTC; the snapshot only includes
PRs already merged when fetched. Of 278 listed PRs, 267 fall in the exact cohorts;
none were excluded for incomplete timelines. Queue entries total 11 and 4,
respectively. The two queue-covered PRs merged after removal are boundary
crossovers, not evidence of an active queue throughout the no-queue period.

Maestro records the activation timestamp in the W6 PR and compares the first
complete seven days, then each rolling seven-day window, with this frozen
no-queue baseline. Revisit if at least 20 merged PRs have a median above
**151.15 minutes** (baseline +30 minutes). Record insufficient samples explicitly.
The merged-PR query does not measure never-merged/ejected PRs, and the recovered
flake ledger does not establish flake-only queue failures. Neither is used as an
ejection-rate trigger; the queue-duration coverage counts are not an ejection census.
The selected fallback is no queue with heavy checks mandatory on backend PRs:
set `CI_MERGE_QUEUE=false`, remove only the merge-queue rule, and keep both
required aggregates. Web-only/docs PRs retain fast checks and optional `ci:full`.
Maestro applies that rollback with Pedro's approval using the W6 PR commands.

## Commit messages

Format: `<type>: <description>` — type is one of `feat`, `fix`, `refactor`, `perf`, `docs`, `test`, `build`, `ci`. Don't use `chore` or `style`. A commit hook checks the format, and scopes are rejected — write `fix: …`, never `fix(web): …`.

```
fix: drop stale session sockets on daemon restart
docs: document workspace scoping for native tools
```

## Pull requests

- Keep each pull request focused on one change.
- Fill in the pull request template — especially how you verified the change.
- Behavior changes need tests. Changes to public behavior usually also need a docs update in `packages/site`.

### Investigating Go docstring coverage warnings

The [CodeRabbit configuration schema](https://storage.googleapis.com/coderabbit_public_assets/schema.v2.json)
marks `reviews.pre_merge_checks.docstrings` as deprecated: the Docstring Coverage check
has been retired and that setting has no effect. Historical reports used an 80% threshold;
do not interpret retirement as evidence that an earlier warning passed. The Go path
instructions still request evidence for documentation findings during ordinary review.

For a historical warning, an aggregate percentage is not a list of undocumented functions.
Expand **Full details:
Docstring Coverage** and **Review info** in the walkthrough, and retain the review URL,
reviewed head, compared commits, analyzed count and any tool failures.

Before editing comments, ask `@coderabbitai` on that PR to name the exact analyzed
functions, their file/line and receiver, and each covered/uncovered classification.
Ask whether the answer comes from retained analyzer evidence or a fresh manual estimate.
For each named function, inspect the attached Go comment at the reviewed head and
distinguish production code, test entry points, benchmarks and helpers. A comment inside
a body is not an attached Go documentation comment; a receiver method can have one just
like a package-level function. Do not assume the analyzer excludes tests or recognizes
every attached comment without evidence from that assessment.

Add a concise comment when it explains a missing contract, invariant or lifecycle rule.
Do not add name-restating boilerplate, lower the threshold, disable the check or exclude
tests to obtain a pass. If the evidence demonstrates a false positive, record the exact
function, existing comment, classification and reproduction links. An unnamed warning
or an unrelated tool timeout alone does not establish a false positive.

Request an explanation of the existing result on the unchanged reviewed head where
supported. Preserve that evidence before comparing a later assessment: a different diff
can change the denominator. If the service cannot recover its analyzed set, record that
limitation explicitly and keep the warning unresolved rather than claiming it repaired.
Path instructions guide review explanations; they cannot restore the retired check or
recover its historical analyzer inventory.

See [#1413](https://github.com/compozy/compozy/issues/1413) for the historical investigation,
the service's unchanged-head response and the remaining evidence limitations.

## Reporting bugs

Use the bug report form. `compozy version` prints the version line, and `compozy support bundle --yes` produces a secret-redacted diagnostics archive you can attach. Never paste unredacted logs or API keys.

## Security

Report vulnerabilities privately — see [SECURITY.md](../SECURITY.md).

## Cloud source license

`cmd/compozy-cloud/`, `internal/cloud/` and `infra/cloud-edge/` use FSL-1.1-MIT. Add its SPDX identifier to new source files in those directories. Keep released migrations byte-for-byte intact. SQLC stamps the generated Postgres Go code during `make codegen`. The daemon, app, shared protocol and integration harness use MIT. Cloud runtime imports stay within the cloud binary and its integration harness. [Repository license](../LICENSE) · [Cloud license](../internal/cloud/LICENSE)
