# Daemon and CI follow-up investigation

Refs #1547. Source inspection and retained CI logs only; no local test, lint,
build, daemon or QA execution is claimed. CI on the PR head owns validation.

## Historical failures

- Late boot reconciliation: job 130360806209 timed out at five seconds before
  entering observer reconciliation. The current suite already reports early boot
  errors and uses its bounded context. Capture bounded goroutine stacks on its
  remaining deadline path; do not infer a production defect from the old sample.
- Loop neutralization: job 130435223440 timed out after releasing the barrier.
  Current source already captures goroutine stacks on that path. The original
  log predates those diagnostics. Preserve the five-second completion assertion.
- Integration shutdown: job 129540374905 failed stopping an active session at the
  two-second phase budget. Capture elapsed time, phase and bounded goroutine stacks
  on deadline errors, preserving the error chain and every teardown phase.
- Usage cooldown: job 130931103759 observed 00:20 instead of 00:21 without a race
  report. Preserve the assertion and log the target, clock and call count on a
  mismatch. Linux/race repetitions must distinguish a reproducible defect from
  an isolated sample. Earlier non-race passes do not establish a cause.

The boot changes are diagnostic improvements, not claimed causal fixes. The
historical boot items remain unchecked until CI evidence supports a disposition.

## Usage cooldown root cause

[Linux/race job 130976658467](https://github.com/compozy/compozy/actions/runs/43156218682/job/130976658467)
on `96da0511b` reproduced the original floor-after-success mismatch: expected
00:21, observed 00:20. The temporary fixture instrumentation captured
`net/http: HTTP/1.x transport connection broken: http: CloseIdleConnections called`
with no request-context cancellation. The collection recorded `network_error`,
so the service correctly retained the previous successful snapshot's 00:20
refresh time instead of receiving the new HTTP 429 and calculating 00:21.

The fixture used `http.DefaultTransport`, shared with parallel HTTP tests.
Go's `httptest.Server.Close` closes idle connections on that global transport,
allowing another fixture's teardown to interrupt a newly reused connection.
The owning `TestUsageRateLimitBackoff` suite must preserve exact cooldown/reset
behavior without unrelated fixture teardown changing its HTTP response.

`usageFixture` now supplies `server.Client().Transport`, which is owned and
cleaned up by that server. Existing cleanup ordering shuts down the service
before the server. Real loopback HTTP, parallelism, fake-clock expectations,
call counts and the Linux/race stress workload are unchanged. Temporary raw
transport logging is removed after capturing the causal evidence; status and
timestamp diagnostics remain. No production retry/backoff behavior changes.
Current-head CI must verify the repair; no local execution is claimed.

## Provider recovery evidence follow-ups

PR #1560 merged as `84874c0f5` and is included after the main rebase.
Its independent review, comment 6812346714, establishes the following provenance:

| Walk | Binary SHA-256 | Source attribution |
| --- | --- | --- |
| Corrective finalization | `857221bfc7fdf4e17e1d9ddfde00e4ac4086e51416f8380bd48ab665099d5dfd` | After B4 repair, before final cancellation change; previous VCS commit, `vcs.modified=true` |
| Cancellation | `8f6bd49663b21d996845e93513df8f7320f8b0c50f9ecb8ae2220c39d3f5496a` | Final production tree later committed as `76d08f05f7b05f5d5ea5b52f52531a82caa14542`; previous VCS commit, `vcs.modified=true` |

These are retained historical receipts, not clean-head executions. The corrective
receipt now records the per-walk binary hashes, dirty VCS attribution and source
ordering; the unknown exact previous VCS revision is explicit. Scenario frontmatter
links the #1557 receipt and fix commits while preserving older evidence and its
partial/pending status. No new local walk is claimed.

The existing full-daemon shutdown integration suite now includes a real ACP
subprocess authentication failure, observes recovery backoff, requests the normal
restart transition, completes `Daemon.Shutdown`, and reopens durable session
history to check resumable intent, absence of a skip and shutdown metadata.
CI must execute this case before D1 receives a verified disposition. The earlier
temporary-overlay observation is still not attributed as a regression.

## Product decision

Automation `completed` currently records execution lifecycle, not an agent's
semantic success. A new agent-reported outcome needs a specified authenticated
reporting surface, durable outcome model, and scheduler/CLI/Web projection.
Inferring failure from prose or changing existing lifecycle meanings would be
unsafe and incompatible. Leave that item unchecked pending a product contract;
the existing intake-failed/watchdog mitigation remains separate.
