# Loop stability — real runtime improvement cycles

Status: in progress. Baseline: `13f4f3dbd`. Scope: loop/graph reliability, built-in engineering loops, and readable operator results.

Lab: `/Users/pedronauck/dev/qa-labs/compozy-loop-stability-20260905-015002-636774-lab`; manifest: `qa-artifacts/qa/bootstrap-manifest.json`. Isolated daemon uses port 50084; Web uses port 3000 and the manifest proxy target. The operator's native Codex login is reused according to the provider's `native_cli`/`operator` policy. No production workspace or database is modified.

## Cycle 1 — carried external results

Related issue: [#541](https://github.com/compozy/compozy/issues/541). Inspection of the latest 100 PRs also covered recent provider failures, node-state projection, review finalization, recovery, and built-in delivery changes.

The public CLI published a billing-planning Loop that imports 60 pending regional rollout tasks through the bundled spec-cycle action, then asks Codex `gpt-5.6-sol` to plan their execution. Its 23,953-byte import result exceeds the inline limit. A second definition introduces a missing receipt-task manifest after that successful import, exercising automatic `failed_only` succession.

- Baseline run `looprun-90d6b772a7421f53` carried the successful import into generation 2 with its original task-run identity. Public task-result reads then failed with `multiple external results`.
- Stopping and restarting the baseline daemon reproduced the reported startup failure: detached-harness recovery could not list terminal task runs. The process exited before readiness.
- Starting the corrected binary against the **same database** succeeded. No state or history was rewritten. CLI/UDS and HTTP returned identical bytes, matching the original import from `looprun-249cd3c22209af4e`.
- Payload SHA-256: `04a086956afa6d4a04df752c6f1fd50196ddb805fd0d110366aa856ce9e81001`.
- Operator rerun was also exercised; its carried cells intentionally omit task-run identity, so automatic reattempt is the relevant reproduction for #541.

Production change: both external-result queries deduplicate identical descriptors. Different content-addressed references still fail as corruption. Invariant owner: the existing `TestGlobalDBCompleteRunLeaseShouldStoreLargeLoopOutputByRef` GlobalDB suite, extended for three carried generations, reopen, all four readers, and conflicting descriptors.

Validation: `CGO_ENABLED=1 go test -race ./internal/store/globaldb -run '^TestGlobalDBCompleteRunLeaseShouldStoreLargeLoopOutputByRef$' -count=1` fails before the fix and passes afterward. The test-shape checker reports the same eight unrelated baseline findings; none belongs to the changed suite. `make gate` passed: Go lint reported zero issues, and all affected `./internal/store/...` race suites passed (GlobalDB 860.622s; lane 878s).

Evidence under `qa-artifacts/qa/`: `recovery-start.json`, `recovery-status-baseline.json`, `recovery-result-baseline.json`, `daemon-restart-baseline.stderr`, `daemon-restart-fixed.json`, `recovery-result-fixed.json`, and `recovery-result-http-fixed.json`.

## Built-in review/fix and observed UI defect

Real run `looprun-61ec517447b2aae9` reviewed a small invoice library with a deliberately incorrect floor-based implementation and existing nearest-cent acceptance tests. The reviewer reproduced the failing half-cent case, the fixer repaired production code and ran `node --test`, the artifact finalizer resolved the finding, and a second independent review ended the Loop as `done`. The run survived the intervening daemon restart. Initial Web observation: 2 rounds, 6m31s, approximately 194k reported tokens.

The live result page displayed raw JSON from every node in its headline, including intermediate fan-out metadata and old review findings. This obscured the final outcome and pushed progress/history below the viewport. Remediation and further edge-case walks remain in progress.

## Cycle 2 — readable outcomes and retained result content

The real review/fix run returned a 2,644-character headline and 14 artifact rows. Nine rows represented control/source execution markers rather than action results; all inline JSON results were falsely labeled pruned because only external blob availability was checked.

The briefing now keeps payloads out of labels/headlines, identifies unnamed results by their producer/round/item, lists the latest round first, excludes execution markers, and recognizes inline results as available. Explicit logical artifact identities and pruned result records remain intact. Web previews three results, expands older results and raw contents on demand, and keeps aggregate partial/pruned signals visible even when their rows are folded. About metadata is collapsed by default; Usage remains visible.

Real validation against the same persisted run after a normal daemon restart:

- CLI and HTTP agree on the 19-character headline and five available action results. The human CLI keeps its separate `Produced:` lines.
- Chrome displays the final review first. Expanding its Details shows `{"issues":[]}`; expanding the additional results and first review exposes the original finding. No history bytes were changed.
- About expands to the original version, inputs, caller, workspace, run ID and copy control.
- `TestBriefingContract` passes with race detection. The affected `make gate` passed: Loop race suite 156.988s, GlobalDB race suite 954.587s, Web lint with zero warnings/errors, typecheck, and all 6,797 Web tests in 748 files. Untouched session/OS tests emitted React act and NaN timer diagnostics; no assertions failed.

Evidence: `review-briefing-fixed.json`, `review-briefing-http-fixed.json`, `review-briefing-fixed.txt`, `daemon-restart-ui.json`; rendered Chrome observations and screenshot in the working session.

## Built-in task implementation

Run `looprun-fcf613da6349b2d6` used `implement-tasks` in `per-task` mode with two dependent invoice-library tasks and Codex `gpt-5.6-sol`. Separate real worker sessions validated monetary inputs, then added exact item totalization. Both task frontmatter statuses reached `completed`; the Loop settled `done` in generation 1. An independent `node --test` passed all seven tests, preserving the original rounding assertions and covering invalid inputs, full discounts, individually rounded items, and safe-integer overflow. No implementation or tracking commit was requested in the customer project.

Evidence: `implement-per-task-start.json`, `implement-per-task-status.json`, and `.compozy/tasks/invoice-validation/` in the lab project.

## Cycle 3 — recovery guidance and command verification

A recovered Loop in generation 3 was still labeled failed because the briefing selected failures
from generation 1. Its terminal predecessor advertised `node requeue`, which returned
`run_terminal`. The briefing now selects current-round node blockers/activity and advertises a
terminal `loop rerun` only when the existing planner accepts that exact cell and its dependencies.
Historical attempts remain inspectable; pending/live operations do not acquire new control rights.

Live run `looprun-eca3516daee3dfad` failed on an absent receipt manifest with reattempt strategy
`halt`. After the manifest was supplied, the printed rerun arguments started generation 2, carried
the successful billing import, and completed the real Codex planning action. CLI reported
`running / ok` during execution and `done / ok` with no blockers afterward. The older recovered run
`looprun-90d6b772a7421f53` likewise has no stale blocker. Evidence: `receipt-dependency-briefing-fixed.json`,
`receipt-dependency-printed-rerun.json`, `receipt-dependency-running-fixed.json`,
`receipt-dependency-final-briefing.json`, `recovery-historical-briefing-fixed.json`.

Real orchestrated run `looprun-c494117598fe2188` created two workers with distinct category models,
completed both receipt tasks, stopped both workers, and passed all 11 independent invoice tests.
Its command judge then repeatedly failed because `compozy` was absent from the evaluator PATH.
The operator canceled the test run to stop futile LLM turns. Production command evaluation now
reuses the same daemon executable/environment binding as ACP sessions, preserving the command's
workspace PWD. The built-in judge calls quoted `COMPOZY_BIN` so renamed binaries are supported.
The canonical daemon command suite invokes the actual test executable through that environment;
the existing ACP suite continues to verify PATH precedence and duplicate removal.

Fresh orchestrated re-walk `looprun-bcd2b1ad861f8a46` finished `done` in generation 1 (91,549 reported tokens). A real worker implemented exact decimal parsing, completed its task and stopped. The command judge accepted that result, and an independent `node --test` passed all 14 invoice tests. Evidence: `orchestrated-fresh-fixed-status.json`, `orchestrated-fresh-fixed-workers.json`, and `invoice-after-orchestrated-fixed-tests.txt`. A separate failure discovered
when rerunning the canceled predecessor (`goal_control_stale`) remains under investigation for the
next cycle; no success is claimed for that recovery yet.

Focused race suites for briefing, command execution, and ACP environment passed using the same
source changes in an overlay before applying them to the checkout. The same focused suites and the built-in judge contract pass in the current checkout. The affected `make gate` passed: Go lint zero issues; daemon race suite 201.817s, Loop 221.158s, GlobalDB 937.904s; repaired spec-cycle fixture passed in the cached rerun. Web lint zero warnings/errors, typecheck and all 6,797 tests passed (existing cached test evidence). No task-file status was changed to bypass a judge.

## Cross-surface impact

Audit follows `docs/_memory/change-impact.md` and is updated here across cycles.

- Native tools: existing task-result and task-run reads recover their intended behavior. Briefing consumers receive corrected artifact identity/availability and concise human wording through the same DTO. No IDs, schemas, gates, or transport shapes change.
- Extensibility/hooks/config: built-in import outputs can be carried safely. No extension manifest, hook, or config key changes. The built-in orchestrated judge now uses the daemon-bound executable; command evaluation receives the existing subprocess environment contract.
- Workspace data isolation: deduplication is limited to identical results for the already-authorized task-run IDs. The existing task/workspace authorization and generation-payload owner checks remain authoritative; no schema migration or persisted rewrite is needed.
- Official skill: checked `skills/compozy/references/loops.md`; documented daemon-bound command discovery; existing public operation and result shapes remain unchanged.
- Web/docs: task-result consumers become readable after automatic succession. `TA-task-run-result-paging`, `LP-web-run-default-read-briefing`, and `LP-run-read-agent-journey` record the affected scenarios. Artifact counts exclude control markers; complete execution history remains in the roster/timeline/Inspect surfaces.

The lab stays alive only for this same-session sequence. Final audit and targeted teardown with `clean: true` are required before closing the overall QA run.
