# QA Run Report — 2026-08-21 — loop-task-legibility

- **Scope:** Runtime/CLI/API phase. The 42 Visual Contract bundles and both `make test-e2e-*`
  lanes remain owned by concurrent agents.
- **Cadence:** Targeted release-grade playbook plus settlement and headless charters.
- **Build:** Started at `4a061d4a`; source frozen after `69c2d74b` plus the official-skill and QA
  documentation update recorded by this report.
- **Environment:** Fresh isolated `northstar-pay` lab, HTTP `http://127.0.0.1:57105`, UDS
  `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-d30b46c8d45d/runtime/compozyd.sock`.
- **Manifest:** `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Started:** 2026-08-21T11:26:16Z · **Status:** runtime phase complete; strict audit and teardown
  are recorded below after they run.

## Isolation and kickoff

- Runtime workspace:
  `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/project`
- QA output:
  `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts`
- `COMPOZY_HOME`:
  `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-d30b46c8d45d/runtime`
- HTTP port `57105`; provider home
  `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-d30b46c8d45d/provider`;
  native CLI kept the operator-home policy.
- Web proxy target came from the manifest: `http://127.0.0.1:57105`.
- Playbook rotation selected `northstar-pay` after `devtool-oss-launch`.
- One operator kickoff was posted to `sess-373883fe1e74049e` at
  `2026-08-21T11:40:52.074432Z`; no agent under test received a follow-up prompt.

## Session matrix and results

| # | Charter / scenario | Runtime-phase verdict | Issue / dependency | Fix |
|---|---|---|---|---|
| 1 | Settlement / LP-terminal-loop-settlement | Fixed | Two boot blockers reproduced; terminal sweep and second boot passed | `0a4fe2d`, `69c2d74` |
| 2 | Settlement / LP-loop-lifecycle-config-cli | Pass | Default, valid, invalid-preserves-prior, and unset walked | — |
| 3 | Headless / LP-run-read-agent-journey | Fixed, prior bug open | Unblocker and beyond-head diagnostics fixed; `BUG-20260719-autonomous-progress-unobservable` remains open | `a53f470`, `b0eaf22`, `37c101d` |
| 4 | Headless / LP-runs-roster-server-ordering | Pass for roster; playbook stall remains | Needs-you ranked before pagination; prior observer bug remains open | — |
| 5 | Catalog parity / TA-task-list-calm-loop-default | Pass | Four transports matched semantically | — |
| 6–9 | Run default-read visual rows | Pending external dependency | Visual Contract bundles owned by concurrent agents | — |
| 10–15 | Tasks/operator-register visual rows | Pending external dependency | Visual Contract bundles owned by concurrent agents | — |
| 16 | Authoring / LP-loop-run-deep-link | Pass for runtime output | Rendered-route parity remains external | — |
| 17 | Authoring / LP-fanout-progress-naming | Pending | Nested fan-out and rendered progress remain outside this bounded settlement/headless phase | — |

## Session debriefs

### Main `northstar-pay` playbook

The lab registered 11 agents, 10 named channels, and 12 deterministic tasks, then released all 12
runs after the single kickoff. The observer recorded kickoff, scheduler release, and 12 starts, but
declared a stall after 300 seconds: nine agents and five channels were silent, and none of the 12
runs had a completion observation at the cut. This is an honest failed playbook observation, not a
prompt-retry: `qa/observation-summary.json` remains the authority. Later task-catalog reads showed
seven completed and five ready ordinary tasks, which further demonstrates why journey-log-only
observation remains insufficient.

Evidence: `qa/operator-kickoff.jsonl`, `qa/task-activation.json`,
`qa/observation-summary.json`, `qa/journey-log.jsonl`.

### CH-loop-legibility-settlement-repair

The first restart failed closed because a daemon-owned coordinator lease had no `session_id`; the
SQL ownership fence treated two nulls as unequal. After that fix, the same coordinator was routed
through ordinary retry exhaustion and became `needs_attention`, blocking reconciliation. Both bugs
were reproduced red, fixed in production with focused store regressions, and committed separately.

The final seeded crash boundary put `looprun-c6fc681a5171ff54` in terminal `canceled` while its three
execution records remained unsettled. Before readiness, the boot sweep reported
`runs_examined=1`, `records_settled=3`, and `orphans_repaired=1`. Fresh public reads showed the
coordinator and both cells canceled; the coordinator timeline carried
`reconciled_run_terminal`. A second boot reported zero examined/settled/repaired rows, the repair
event count stayed `1`, and `task list --loop-run ... --status ready` returned zero rows. Public
`loop cancel` and `loop kill` were also walked against pending request runs.

Evidence: `qa/settlement/boot-after-sessionless-fix.log`,
`qa/settlement/boot-terminal-sweep.log`, `qa/settlement/terminal-loop-public-tasks.json`,
`qa/settlement/coordinator-after-second-boot.json`, and
`qa/settlement/boot-idempotent-second.log`.

### CH-loop-legibility-run-read-resume

Briefing, complete roster, and timeline responses matched semantically across CLI, HTTP, and UDS.
Timeline presentation order differs intentionally — CLI is chronological while HTTP/UDS pages are
newest-first — but sorting by sequence produced the same hash on all three surfaces. Resume after
sequence 5 returned exactly 6–10 once; follow at head 10 exited cleanly. A foreign cursor returned
409 `timeline_branch_changed`, a malformed cursor returned 400 `invalid_cursor`, and a foreign
workspace returned 404 `loop_run_not_found`. Payload scans found no claim token, secret, credential,
or session token.

The runtime-published request unblocker was executed verbatim after its two-part fix. A second
finding showed that beyond-head errors discarded the real head; after `37c101d`, the CLI printed
`position 999 ... (head: 10)` and structured transports returned stable code plus position/head
details.

Evidence: `qa/headless/read-parity.sha256`, `qa/headless/timeline-semantic-parity.sha256`,
`qa/headless/timeline-resume-after5.json`, `qa/request-unblocker-rewalk-execution.txt`, and
`qa/headless/after-beyond-head-fixed.json`.

### CH-loop-legibility-calm-catalog-parity

The default CLI, HTTP, UDS, and session-scoped `compozy__task_list` responses normalized to the
same SHA-256. The calm default contained 12 ordinary tasks; typed inclusion contained 17 rows,
including five Loop records. The run filter returned three typed rows and implied inclusion on all
four transports. Unknown `--loop-run` returned an empty page at exit 0. Invalid
`include_loop=banana` returned `400 {"error":"invalid_query_field","field":"include_loop"}` over
both HTTP and UDS. Cross-workspace run reads returned 404.

Evidence: `qa/cross-surface/default-parity.sha256`,
`qa/cross-surface/include-parity.sha256`, `qa/cross-surface/loop-run-parity.sha256`, and
`qa/cross-surface/http-invalid-include.txt`.

### CH-loop-legibility-authoring-run-canary — runtime slice

The non-dry human command ended with the effective daemon URL. JSON and TOON carried `web_url` for
their persisted runs. Human, JSON, and TOON dry-runs emitted no URL. Nested fan-out naming and the
rendered destination are not claimed in this bounded phase.

Evidence: `qa/headless/authoring-human.txt`, `qa/headless/authoring-toon.txt`, and
`qa/headless/authoring-dry-json.json`.

## What was fixed

| Bug | Root cause | Commit | Focused verification |
|---|---|---|---|
| BUG-20260821-loop-unblocker-invalid-json | Prose placeholder, then shell-unsafe braces | `a53f470`, `b0eaf22` | Canonical `TestBriefingContract` plus verbatim live execution |
| BUG-20260821-loop-timeline-head-omitted | API mapper discarded `TimelinePositionError` fields | `37c101d` | `TestLoopReadHandlersMapping` and live CLI/HTTP/UDS re-walk |
| BUG-20260821-sessionless-lease-recovery | Nullable session ownership fence used non-null-safe equality | `0a4fe2d` | `TestGlobalDBRecoverExpiredRunLeasesThenClaim` |
| BUG-20260821-coordinator-lease-exhausted | Coordinator lease used ordinary task attempt exhaustion | `69c2d74` | Expired coordinator plus network-wake/reconcile focused suites |

No test was weakened. No `make gate`, `make gate-full`, or `make test-e2e-*` command was run.

## Runtime errors and paper cuts

- The one-kickoff playbook observer stalled even though later catalog reads showed seven completed
  tasks. `BUG-20260719-autonomous-progress-unobservable` remains open.
- `compozy task timeline` named by the charter is not a current verb; the public structured detail
  is available through `compozy task get`. This report used `task get` and did not invent a command.
- The test-convention checker reports eight pre-existing inline-case violations elsewhere in
  `global_db_task_claim_test.go`; none is in the edited invariant. They were not changed.

## Compozy Impact Audit

- **Native tools:** No descriptor, schema digest, risk flag, or capability gate changed. Checked
  `compozy__task_list`, `compozy__loop_status`, `compozy__loop_runs`, and
  `compozy__loop_nodes` against the isolated daemon; task-list normalized output matched CLI,
  HTTP, and UDS.
- **Extensibility and hooks:** No extension, hook event, registry, bridge SDK, MCP sidecar, or Loop
  configuration key changed. Checked Loop tool registration and config lifecycle; only error detail
  preservation and internal lease recovery changed.
- **Workspace data isolation:** Changed data remains task-run/Loop-run scoped inside workspace
  `ws_bafc88d97a58b5f5`. HTTP and CLI foreign-workspace run reads returned 404, and all task catalog
  comparisons carried the same workspace ID. No cache, SSE, event, or list path exposed foreign
  data.
- **Official Compozy skill:** Updated `skills/compozy/references/loops.md` with the stable
  beyond-head code and position/head recovery fields. Audit: `qa/skill-audit.md`.

## Remaining dependencies

- 42 Visual Contract reference/implementation bundles and their structural mismatch decisions.
- Concurrent `make test-e2e-runtime` and `make test-e2e-web` results.
- Nested fan-out progress naming across runtime plus rendered web.
- The open autonomous-progress observation bug above.
- Task 07 remains in progress until those owners finish; this report does not mark it complete.

## Strict audit and teardown

- **Strict audit:** pending final invocation.
- **Teardown:** pending exact manifest command; completion requires `qa/teardown.json` with
  `"clean": true`.

## Final status

- **Runtime phase:** complete after strict audit and clean teardown.
- **Owned scenario rows:** settlement/config/catalog/deep-link pass; headless fixes verified;
  run-read/roster retain the pre-existing observer bug; nested fan-out remains pending.
- **Full task:** not complete — visual and E2E dependencies remain external.
