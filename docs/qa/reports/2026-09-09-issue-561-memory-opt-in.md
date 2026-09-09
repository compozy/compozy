# Memory opt-in and extractor robustness

Status: targeted runtime verification passed; CI/reviews pending. Issue: #561.

## Scope and invariant ownership

- Configuration: omitted memory/dream switches stay false; explicit global, profile, and workspace values retain precedence. Canonical suites: `internal/config/memory_v2_config_test.go`, `roles_test.go`, and `merge_test.go`.
- Daemon extractor: explicit no-candidate output and conventional empty wrappers succeed; valid lines survive malformed batches; child failure/timeout and extraction success remain distinct. Canonical suite: `internal/daemon/memory_runtime_test.go`.
- Inbox worker: partial candidates reach the controller while the extraction records failure. Canonical suite: `internal/memory/extractor/runtime_test.go`.
- Dream scheduling: disabled roles do not evaluate gates or begin consolidation; explicit live/workspace opt-ins remain usable. Canonical suites: daemon lifecycle and `internal/memory/consolidation/runtime_test.go`.
- Session compaction: pressure work is independent of persistent memory and remains controlled by `session.compaction.enabled`. The internal invocation distinguishes pressure summaries from session-end work, honors checkpoint-role opt-out, and refuses to archive uncovered events on disabled/failed summarization. Factory boot retains private checkpoint WAL storage and resume-only coverage without a public memory provider or session-end worker. Canonical suites: checkpoint summarizer/runtime, memory checkpoint coverage, session pressure compaction, and `TestDaemonE2EFactoryPressureCompaction`.
- Real runtime: the existing daemon memory E2E suite owns CLI/HTTP/UDS status, persisted candidates, no-candidate completion, failure listing, and zero default child work. Its full-feature config fixture now explicitly opts in; factory scenarios load factory switch values.

## Reproduction

The same `TestMemoryExtractorOutputContract` was run with a Go overlay containing the original
`memory_extractor_output.go` from the branch base. It reproduced the invalid `N` and `(` errors,
discarded both valid candidates around malformed JSON, and discarded a valid candidate after prose.
The updated parser passed these cases with `-race`. The timeout in the issue is a separate provider
execution deadline, not a JSON decoding consequence.

## Change impact

- Native tools: existing memory/extractor/dream IDs and schemas remain. Disabled defaults make memory runtime tools unavailable as before; extractor status/failure listing exposes extraction-stage failures through the existing payloads.
- Extensibility/hooks/config: no new keys, hooks, SDK shapes, or migrations. Session pressure compaction retains its independent control and its summary invocation marker is internal. `memory.enabled` and `roles.dream.enabled` default false. Extraction retains the existing persisted-message hook when enabled; dreaming checks role eligibility before its Time → Sessions → Lock gates.
- Workspace isolation: candidate workspace/agent routing stays unchanged; failure reports retain parent session, workspace, and agent identity. Effective role configuration honors the memory switch and workspace/profile role overlays. The daemon-level memory switch controls runtime registration.
- Official skill: `skills/compozy/references/memory.md` documents deliberate opt-in and extraction diagnostics; retrieval algorithms and startup tool-manual policy are untouched.
- Web/Docs: no Web type or component changes; existing settings controls retain their keys. Site memory/config references and release migration notes describe defaults, enablement, and mixed-output behavior.
- Compatibility: no state deletion or config rewrite. Explicit values survive; omissions adopt the requested default. Existing response shapes/keys remain, so no deprecation shim is introduced.

## Evidence and limits

- Config, extractor, and consolidation suites passed with `-race` (19.281 s, 3.001 s, 1.318 s).
- The capped daemon rerun passed all selected extractor/lifecycle/role/reentry cases except the unchanged boot barrier timeout. Its standalone capped retry passed in 12.418 s, retaining the five-second assertion. The controller independently observed the same host-load failure and clean standalone retry in #562.
- Canonical compaction checks passed with `-race -p=1 -parallel=4`: daemon 6.359 s, memory 1.697 s, session 4.603 s. The disabled role retains the existing checkpoint hint. The subsequent compatibility fix changes an uncovered disabled-summary request to return an error so the session cannot archive it.
- `go run ./cmd/compozy-codegen all` completed with no generated diff.

- Existing daemon memory E2E cases ran with real daemon/ACP subprocesses and SQLite under the shared machine-wide verification lock, using `-race -p=1 -parallel=4 -tags=integration`. Opted-in empty extraction passed (8.00 s), partial malformed output passed (9.34 s), and both deliberate dream routing cases passed. Existing CLI/HTTP catalog parity, next-session recall, live role apply, and crash-safe compaction integration also passed.
- The factory case initially stopped in fixture validation because its unused synthetic response was empty. Giving that fixture a valid no-candidate response allowed the same zero-work assertions to execute and pass (2.22 s; package 4.943 s). An earlier test compilation typo used `Type` instead of the existing `SessionInfo.SessionType`; it was corrected before runtime execution.
- The original completed lab used canonical root `/private/tmp/compozyqa-66cec21f3dea`: daemon socket 57 bytes, tmux socket 60 bytes, worst-case generated harness socket 88 bytes (limit 103). Its predecessor was torn down cleanly before replacement.

- The initial gate found one trailing blank line in the touched config test; it was corrected without changing an assertion.
- The pressure compatibility fix passed canonical memory/session checks (1.665 s / 4.132 s) and the daemon summarizer/lifecycle checks (9.930 s). Its store lifetime follows the existing catalog shutdown order after session/compaction teardown, including boot rollback.
- The first short lab completed targeted teardown with `clean: true`. The pressure compatibility re-walk uses a fresh short lab, allocated after that teardown.

The owner explicitly requested publication with the full gate running in CI. A clean full local gate is not claimed; current-head required CI remains a delivery requirement. Final pressure E2E and teardown evidence follows below. The ACP fixture run
will verify real daemon/subprocess/protocol behavior with deterministic model output, not live
provider reliability or timeout frequency.

## Factory pressure acceptance

`TestDaemonE2EFactoryPressureCompaction` passed with the actual daemon, ACP subprocesses, and SQLite (9.770 s; package 11.925 s). Factory memory/dream settings remain false. The test verifies one pressure-only checkpoint child, coverage before archive, retained historical facts, and degraded recovery after an ACP disconnect: four protocol attempts, three completed prompts, and two parent runtimes. The recovered prompt includes the checkpoint fact and excludes its archived raw event from replay. No extractor/dream/session-end memory child appears.

The initial test setup failures were corrected without changing the production contract: bind the checkpoint role to the fixture agent; use automatic recovery instead of attaching a deliberately stopped terminal session; count interrupted protocol attempts separately from completed prompt diagnostics. Summary failures observed during setup left the source events unarchived.

The pressure re-walk also passed the unchanged crash-safe compaction suite and the three default/opt-in extractor cases. Canonical test-convention checks passed. Pressure lab `/private/tmp/compozyqa-232d0fe5b103` completed targeted teardown with `clean: true`; logs and provider/event artifacts are retained in its QA output. No live-provider reliability claim is made.

## Review remediation

Greptile identified terminal-less extractor success and raw role status. The collector now requires a successful semantic terminal, retains partial diagnostic output on errors, and refuses candidate admission after interruption. Background role projections and invocations share the effective daemon/workspace memory gate; pressure summaries retain their internal exception. Existing unit suites cover these boundaries, and the memory E2E suite adds interrupted-output and public-role-status assertions. CI health fixtures now opt in explicitly where their invariant requires active memory; assertions remain unchanged. The scoped runtime test added in this round is pending the next CI head.

Review regression checks passed with `-race -p=1 -parallel=4`: daemon roles/extractor/checkpoints 6.990 s; API core memory health 13.388 s; UDS health 1.299 s; HTTP health 1.288 s; settings 1.071 s. The read-only test-shape checker reports existing direct-test shapes in untouched test bodies; comparison against the prior head found zero new findings.
