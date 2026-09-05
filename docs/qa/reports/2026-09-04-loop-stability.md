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

## Cross-surface impact

Audit follows `docs/_memory/change-impact.md` and is updated here across cycles.

- Native tools: existing task-result and task-run reads recover their intended behavior. No IDs, schemas, gates, or transport shapes change.
- Extensibility/hooks/config: built-in import outputs can be carried safely. No extension manifest, hook, or config key changes in cycle 1.
- Workspace data isolation: deduplication is limited to identical results for the already-authorized task-run IDs. The existing task/workspace authorization and generation-payload owner checks remain authoritative; no schema migration or persisted rewrite is needed.
- Official skill: checked `skills/compozy/references/loops.md`; public operation and result semantics remain unchanged in cycle 1.
- Web/docs: task-result consumers become readable after automatic succession. Existing `TA-task-run-result-paging` gains this scenario; the raw Loop headline finding is tracked above.

The lab stays alive only for this same-session sequence. Final audit and targeted teardown with `clean: true` are required before closing the overall QA run.
