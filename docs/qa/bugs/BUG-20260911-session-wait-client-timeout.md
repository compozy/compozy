# BUG-20260911-session-wait-client-timeout: Session wait ends before its requested timeout

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno; Ada
- **Journey Step:** J-26 control observation; J-15 wait for exact session state
- **Scenarios:** RT-session-wait-state
- **Found:** 2026-09-11
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction

During real Cursor Goal continuation, run `compozy session wait sess-00a7a5caef013168 --until idle --timeout 45s -o json`. At30.027s the CLI exits69 with Client.Timeout exceeded while awaiting headers, before its requested server wait expires. The real worker later settles normally. Evidence: goal-controls-resume-wait.json, goal-controls-final-turns.json.

## Cause and bounded fix

WaitSession calls doJSON with the ordinary30s client. Existing blocking StopSession and streaming operations already use the dedicated long-lived client. Reuse that transport for wait; server-side bounded registration and caller cancellation retain lifetime ownership. No global timeout change or retry is needed.

## Cross-surface impact

Only CLI wait transport selection changes. HTTP/UDS/native server contracts, wait timeouts/outcomes, permission checks, hooks/config, workspace isolation, Web and persisted data are unchanged. Official skill already documents bounded and unbounded waiting, so no grammar or migration is needed. The canonical client transport regression failed before the fix and passes afterward, including caller cancellation. Go build passed. The conventions checker reports exactly the same ten pre-existing findings on the untouched baseline; the new subtests add none. Real replay and make gate passed.

## Verification

Fresh managed session sess-9c613f80b1953c84 returned idle immediately. Waiting for running with --timeout45s returned the server timeout payload and exit75 after45.03s; waiting for stopped remained live until a public stop at35s, then returned state-reached with exit0 after35.113s. The session is stopped. Only the CLI binary changed; the daemon retained its existing build because its wait implementation was unchanged. session-wait-retest-proof.json indexes these public outcomes. The race-enabled transport regression covers server timeout ownership and caller cancellation. make gate passed all affected lanes.
