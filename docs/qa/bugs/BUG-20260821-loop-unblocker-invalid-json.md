# BUG-20260821-loop-unblocker-invalid-json: Printed Loop request unblocker cannot be executed

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada, headless Loop operator
- **Journey Step:** J-operate-loop-run-headless, execute the runtime-published unblocker
- **Scenarios:** LP-run-read-agent-journey
- **Found:** 2026-08-21 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** n/a

## Summary

Ada cannot resume a Loop request by executing the command printed by `compozy loop why`. The
command supplies `<json>` as the payload, so the public CLI rejects its own runtime-published
unblocker before the request can be answered.

## Reproduction

- **Charter:** CH-loop-legibility-run-read-resume · **Tour:** Network Tour
- **Environment:** fresh isolated `northstar-pay` lab; daemon HTTP `127.0.0.1:57105`; isolated UDS

1. Publish and run the `qa-request-unblocker` Loop through the isolated daemon.
2. Wait for `select_target` to park with one pending ask.
3. Read `compozy loop why looprun-f2a20f63a3f26eba -o json`.
4. Execute the returned `blockers[0].unblocker` verbatim.

**Expected:** The printed command carries valid JSON and resolves the pending request.
**Actual:** The printed command carries `--payload \\<json\\>` and fails with
`cli: --payload must be valid JSON`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/request-unblocker-before.json`
- `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/request-unblocker-before-execution.txt`

## Fix

- **Root cause:** The briefing projector published a prose placeholder, then the first replacement
  used shell escaping that zsh parsed as syntax instead of one JSON argument.
- **Fix commits:** `a53f470`, `b0eaf22`
- **Regression test:** `TestBriefingContract/Should_satisfy_UT-004_with_expired_request_truth_and_no_retry_field`

## Verification

- **Retested:** 2026-08-21 in the isolated runtime lab
- **Result:** `qa/request-unblocker-rewalk-execution.txt` shows the runtime-published command with
  `--payload '{}'` executing verbatim and resuming the request.
