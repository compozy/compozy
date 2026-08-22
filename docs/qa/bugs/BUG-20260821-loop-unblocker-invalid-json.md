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

Ada could not safely resume every Loop request by executing the command printed by
`compozy loop why`. The original command supplied `<json>` and a first correction replaced it with
`{}`. That empty object happened to satisfy a permissive request but fabricated operator data and
failed any `expect` or `respond_schema` that required fields or entity identifiers.

## Reproduction

- **Charter:** CH-loop-legibility-run-read-resume · **Tour:** Network Tour
- **Environment:** fresh isolated `northstar-pay` lab; daemon HTTP `127.0.0.1:57105`; isolated UDS

1. Publish and run the `qa-request-unblocker` Loop through the isolated daemon.
2. Wait for `select_target` to park with one pending ask.
3. Read `compozy loop why looprun-f2a20f63a3f26eba -o json`.
4. Execute the returned `blockers[0].unblocker` verbatim.

**Expected:** The printed command collects explicit operator JSON and submits exactly that value.
**Actual:** The original command carried `--payload \\<json\\>` and failed locally; the first fix
carried `--payload '{}'` and could auto-submit a schema-invalid or unsafe synthetic answer.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/request-unblocker-before.json`
- `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/request-unblocker-before-execution.txt`

## Fix

- **Root cause:** The briefing projector treated “valid JSON syntax” as equivalent to a valid human
  response. It owned no response value and therefore had no safe default to publish.
- **Fix commits:** `a53f470`, `b0eaf22`, plus the 2026-08-21 root-review remediation batch
- **Regression test:** `TestBriefingContract/Should_satisfy_UT-004_with_expired_request_truth_and_no_retry_field`
  and `TestLoopCommandShouldMapCLIVerbsToClient/Should_read_a_required-schema_response_explicitly_from_stdin`

## Verification

- **Retested:** 2026-08-21 in fresh targeted lab
  `compozy-loop-unblocker-operator-input-20260821-20260821-124157-149087-lab`.
- **Result:** A request whose schema required `environment` published an executable
  `--payload-stdin` command. The command prompted for JSON, waited for Ada's explicit
  `{"environment":"production"}` input, resumed the run, and produced matching terminal CLI and
  HTTP reads. Evidence: `qa/request-unblocker-required-schema-rewalk.md`.
