# QA Run Report — 2026-08-21 — loop-task-legibility

- **Scope:** Full Loop legibility cycle across runtime, CLI/API, daemon-served Web, scenario walks,
  and the 42-row Visual Contract matrix.
- **Cadence:** Targeted release-grade playbook plus settlement and headless charters.
- **Build:** Started at `4a061d4a`; source frozen after `69c2d74b` plus the official-skill and QA
  documentation update recorded by this report.
- **Environment:** Fresh isolated `northstar-pay` lab, HTTP `http://127.0.0.1:57105`, UDS
  `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-d30b46c8d45d/runtime/compozyd.sock`.
- **Manifest:** `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Started:** 2026-08-21T11:26:16Z · **Status:** QA execution `PASS`; every lab teardown is clean.
  The user explicitly delegated the workstream-closing full gate to PR CI after the controller's
  local focused checks passed.

## 2026-08-22 — Task 07 daemon-served web continuation

- **Lab:**
  `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-20260822-025427-168223-lab`
- **Strict accessibility invariant:** Every rendered Loop state chip exposes a non-empty accessible
  name while retaining its visible label, icon, tokens, and reduced-motion behavior.
- **Owner and canonical suites:** The Loop state-chip component owns the semantic output;
  `loop-run-page.test.tsx` owns all 15 component variants and E2E-019 in `loop-run.spec.ts` owns the
  daemon-served browser contract.
- **Strict result:** PASS. The rejected `toContainText` assertion was restored to
  `toHaveAccessibleName(/\S/)`. The canonical component/model selection passed 175/175, the web
  build (including typecheck) passed, focused real-browser E2E-019 passed, and the complete
  canonical Loop run file passed 17/17 in the daemon-served lane.
- **Fresh final lab:**
  `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab`.
- **Full web lane:** PASS. After classifying and repairing every one of the 23 deterministic
  Dashboard, OS shell, Settings, Storybook, and Worktrees failures, the fresh daemon-served lane
  completed with **227 passed, 3 declared conditional skips, and 0 failed** in 36.0 minutes. The
  strict E2E-019 accessible-name assertion passed in that complete lane. Remediation used focused
  reruns only; no timeout or expectation was weakened.
- **Data-boundary correction:** Palette catalog carry-forward is limited to the same workspace's
  null-client → attached-client transition. The canonical hook regression proves workspace A
  commands never render while workspace B is pending.
- **Unavailable palette rows:** cmdk rows stay enabled and keyboard-navigable because they retain
  valid meta-actions. The visible unavailability reason is excluded from name-from-content and is
  supplied once through `aria-describedby`; UT-128 proves the exact accessible name, exact
  description, navigation, seam refusal, and meta-actions.
- **Scenario walks:** PASS. All 13 planned tracker walks ran only after the complete lane was green;
  the three overlapping regression scenarios for gate provenance, pending-request provenance, and
  cancellation/duration truth also pass. Evidence is indexed by the final lab's
  `qa/task07-scenario-walks.md`.
- **Visual Contract:** PASS. Task 04 VC-01–06 and Task 05 VC-01–36 have complete reviewed bundles
  under `.compozy/tasks/loop-task-legibility/evidence/visual/`; all 42 validators report zero
  blocking divergences. Broken unstyled prototype captures were rejected and recaptured from the
  correct reference root before comparison.
- **Manual browser walk:** PASS. The two-register run page was walked through Graph → Nodes →
  Events, with arrow-key movement back to Generations, followed by needs-you provenance,
  cancellation-cause, and mixed-state roster walks. The 33-frame recording is at
  `/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/task07-loop-operator-register`.
- **Final teardown:** PASS. The exact final-lab manifest teardown completed at
  `2026-08-22T16:55:08Z`; `qa/teardown.json` records `"clean": true`, an empty survivor list, and
  termination of the registered daemon, Storybook, and visual-reference server. Ports 62112, 6006,
  and 6008 have no listener.
- **Task status:** QA complete. The controller reviewed the remediation and ran focused race and
  lint checks; the user explicitly delegated the full monorepo gate to PR CI.

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
| 3 | Headless / LP-run-read-agent-journey | Pass | Required-schema unblocker and runtime-owned observer fixed and re-walked | `a53f470`, `b0eaf22`, `37c101d`, observer batch |
| 4 | Headless / LP-runs-roster-server-ordering | Pass | Roster passed; public-read observer matched the independent catalog through terminal state | observer batch |
| 5 | Catalog parity / TA-task-list-calm-loop-default | Pass | Four transports matched semantically | — |
| 6–9 | Run default-read visual rows | Pass | Daemon-served walks passed; Task 04/05 Visual Contract bundles validate | QA remediation batch |
| 10–15 | Tasks/operator-register visual rows | Pass | Daemon-served walks passed; Task 04/05 Visual Contract bundles validate | QA remediation batch |
| 16 | Authoring / LP-loop-run-deep-link | Pass | Runtime output and rendered-route parity passed | QA remediation batch |
| 17 | Authoring / LP-fanout-progress-naming | Pass | Nested fan-out and rendered progress passed in the final Web lane | QA remediation batch |

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

The first request-unblocker re-walk only proved that `{}` passed a permissive schema. Root review
corrected that unsafe claim in a fresh targeted lab: a request requiring `environment` published
`--payload-stdin`, prompted for explicit JSON, and resolved only after the operator entered
`{"environment":"production"}`. A second finding showed that beyond-head errors discarded the real head; after `37c101d`, the CLI printed
`position 999 ... (head: 10)` and structured transports returned stable code plus position/head
details.

Evidence: `qa/headless/read-parity.sha256`, `qa/headless/timeline-semantic-parity.sha256`,
`qa/headless/timeline-resume-after5.json`, `qa/headless/after-beyond-head-fixed.json`, and remediation
lab `qa/request-unblocker-required-schema-rewalk.md`.

### Root-review remediation re-walk

A fresh targeted lab used isolated HTTP `63979`, UDS
`/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-315af9a19c7c/runtime/compozyd.sock`,
and workspace `ws_d0de16f57018e715`. The required-schema command printed by `loop why` was executed
verbatim with the remediation binary on `PATH`; it waited at `Response JSON:` and accepted the
operator's explicit response. CLI and HTTP independently reported the same terminal progress.

Manifest:
`/Users/pedronauck/dev/qa-labs/compozy-loop-unblocker-operator-input-20260821-20260821-124157-149087-lab/qa-artifacts/qa/bootstrap-manifest.json`.
Evidence: `qa/request-unblocker-required-schema-rewalk.md`.

The targeted strict audit inspected the CLI/API/runtime journey and failed only C14 because this
remediation was explicitly prohibited from running `make verify` or any gate. Audit evidence:
remediation lab `qa/qa-audit-report.json` and `qa/qa-audit-report.md`. This focused re-walk is not a
release-grade PASS claim.

### Autonomous-progress observer closure

A fresh `consumer-saas-growth` lab used isolated workspace `ws_a988176b3f8fc9d5`, HTTP
`http://127.0.0.1:53317`, and a unique `COMPOZY_HOME`. One operator kickoff was posted at
`2026-08-21T13:10:19.761407+00:00`; no follow-up prompt was sent. The fixed observer read public
Task catalog/detail state and the Loop runs API, with explicit scenario/runtime workspace, workspace
id, API, home, and binary inputs. This replay listed no Loop runs, so the conditional `loop why` and
`loop events` branch was not exercised.

Independent Task catalog captures advanced from 4 completed / 7 in progress to 11/0 while the
observer recorded eight durable transitions and no stall. Its final account matched all 11 ids and
statuses from an independent public catalog read. The observer neither read nor wrote
`journey-log.jsonl`, and no prompt followed kickoff.

Manifest:
`/Users/pedronauck/dev/qa-labs/compozy-loop-legibility-observer-closure-20260821-130214-633585-lab/qa-artifacts/qa/bootstrap-manifest.json`.
Evidence: `qa/observation-summary.json`, `qa/task-catalog-independent-before.json` through
`qa/task-catalog-independent-after.json`, `qa/observer-catalog-comparison.json`, and the itemized
`qa/skill-audit.md` in that lab.
This satisfies the exact closure condition for BUG-20260719-autonomous-progress-unobservable; it
does not change task 07 or upgrade the broader release-grade playbook verdict.

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
| BUG-20260821-loop-unblocker-invalid-json | The projector fabricated `{}` despite request-owned schemas | remediation batch | Canonical required-schema briefing + CLI stdin regressions and verbatim live execution |
| BUG-20260821-loop-timeline-head-omitted | API mapper discarded `TimelinePositionError` fields | `37c101d` | `TestLoopReadHandlersMapping` and live CLI/HTTP/UDS re-walk |
| BUG-20260821-sessionless-lease-recovery | Nullable session ownership fence used non-null-safe equality | `0a4fe2d` | `TestGlobalDBRecoverExpiredRunLeasesThenClaim` |
| BUG-20260821-coordinator-lease-exhausted | Coordinator lease used ordinary task attempt exhaustion | `69c2d74` | Expired coordinator plus network-wake/reconcile focused suites |
| BUG-20260719-autonomous-progress-unobservable | Observer used a lab-owned journey log instead of runtime-owned public state | observer batch | Four focused behavior cases plus fresh one-kickoff public-read replay |
| BUG-20260822-requeued-continuation-epoch-drift | Requeue advanced the continuation attempt instead of propagating its fenced output epoch | controller remediation batch | Both canonical GlobalDB requeue suites plus focused cross-package race tests |

No test was weakened. The QA worker did not run `make gate` or `make gate-full`.

## Runtime errors and paper cuts

- The original one-kickoff playbook observer stalled even though later catalog reads showed seven
  completed tasks. The focused closure replaced that lab-log dependency with public Task/Loop reads
  and verified the new account against an independent catalog; the bug is now verified.
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
  configuration key changed. Checked Loop tool registration and config lifecycle; the observer skill
  invocation now carries explicit isolated runtime inputs and consumes existing public reads.
- **Workspace data isolation:** Changed data remains task-run/Loop-run scoped inside workspace
  `ws_bafc88d97a58b5f5`. HTTP and CLI foreign-workspace run reads returned 404, and all task catalog
  comparisons carried the same workspace ID. No cache, SSE, event, or list path exposed foreign
  data.
- **Official Compozy skill:** Updated `skills/compozy/references/loops.md` with the stable
  beyond-head code and position/head recovery fields. Audit: `qa/skill-audit.md`.

## Remaining dependencies

- PR CI owns the workstream-closing full gate by explicit user decision; no QA execution evidence
  is outstanding.

## Strict audit and teardown

- **Strict audit:** `BLOCKED` (exit 2), report `qa/qa-audit-report.md`. The 12 blockers are: missing
  provider/web journey surfaces; fewer than three cross-surface objects recognized by the playbook
  auditor; zero live provider-backed sessions; zero later-used playbook artifacts; zero completed
  disruption probes; no explicit verdict before this final update; no `make verify` evidence by
  explicit phase restriction; one of two required `ts_test` deliverables; zero of 14 peer messages;
  zero of two review cycles; zero disagreement resolutions; and no duplicate lab-local `qa/issues`
  file for the already registered observer-stall bug. The audit also emitted one warning that API
  deep equality is not implemented by the auditor. None of these results was hidden or filled with
  synthetic activity.
- **Teardown:** exact manifest command completed at `2026-08-21T12:31:15Z`.
  `qa/teardown.json` says `"clean": true`, killed registered daemon PID `65799`, and reports no
  survivors.
- **Remediation teardown:** the fresh targeted lab's exact manifest command completed at
  `2026-08-21T12:46:03Z`. Its `qa/teardown.json` says `"clean": true`, killed registered daemon PID
  `69040`, and reports no survivors.
- **Observer-closure teardown:** the fresh one-kickoff lab's exact manifest command completed after
  the terminal catalog comparison. Its `qa/teardown.json` says `"clean": true`, killed registered
  daemon PID `72878`, and reports no survivors.
- **Task 07 final Web teardown:** the final lab's exact manifest command completed at
  `2026-08-22T16:55:08Z`. Its `qa/teardown.json` says `"clean": true`, killed registered PIDs
  `34948`, `86638`, and `91977`, and reports no survivors.

## Final status

- **Task 07 QA execution:** PASS. The daemon-served Web lane is green at 227 passed, 3 declared
  conditional skips, and 0 failed; all 13 planned walks and three overlapping regression walks pass.
- **Visual evidence:** PASS. All 42 reference/implementation bundles validate with zero blocking
  divergences.
- **Tracker:** Every scenario reset or added by this QA tail records `qa_status: pass` with current
  evidence.
- **Open issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 ·
  Cosmetic 0. Every reproduced issue in this run is fixed and re-walked.
- **Delivery state:** QA is ready for PR publication. By explicit user decision, PR CI owns the
  workstream-closing full gate.
