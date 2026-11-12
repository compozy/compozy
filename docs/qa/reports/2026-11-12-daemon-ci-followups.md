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

These are diagnostic improvements, not claimed causal fixes. The historical
boot and Usage items remain unchecked until CI evidence supports a disposition.

## Provider recovery evidence follow-ups

PR #1560 is still open and its corrective receipt is absent from this base.
Its independent review, comment 6812346714, establishes the following provenance:

| Walk | Binary SHA-256 | Source attribution |
| --- | --- | --- |
| Corrective finalization | `857221bfc7fdf4e17e1d9ddfde00e4ac4086e51416f8380bd48ab665099d5dfd` | After B4 repair, before final cancellation change; previous VCS commit, `vcs.modified=true` |
| Cancellation | `8f6bd49663b21d996845e93513df8f7320f8b0c50f9ecb8ae2220c39d3f5496a` | Final production tree later committed as `76d08f05f7b05f5d5ea5b52f52531a82caa14542`; previous VCS commit, `vcs.modified=true` |

These are retained historical receipts, not clean-head executions. Updating that
unmerged receipt and its scenario metadata awaits the upstream documentation;
this PR must not copy in unrelated recovery implementation. D1 still requires
an orderly full-daemon shutdown during recovery backoff and persisted restart
intent read-back. The temporary-overlay probe on both base and head is not
proof of a regression. No local runtime walk is authorized in this assignment.

## Product decision

Automation `completed` currently records execution lifecycle, not an agent's
semantic success. A new agent-reported outcome needs a specified authenticated
reporting surface, durable outcome model, and scheduler/CLI/Web projection.
Inferring failure from prose or changing existing lifecycle meanings would be
unsafe and incompatible. Leave that item unchecked pending a product contract;
the existing intake-failed/watchdog mitigation remains separate.
