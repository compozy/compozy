# BUG-20261103-target-instance-id-accepted: Trigger accepts an instance ID as target and every delivery 404s

- Status: verified (fixed on feat/stays-on-hardening 4056dbcc2)
- Impact (user-side): Blocks-Completion
- Severity: Medium; Priority: P2
- Persona Affected: Ada
- Journey Step: Create a webhook trigger for a stays-on agent from the CLI
- Scenarios: TA-automation-last-run-agent
- Origin: stays-on hardening QA walk, soh/qa 583415ef5
- Report: docs/qa/reports/2026-11-03-stays-on-hardening-qa.md

## Reproduction

`compozy automation triggers create --event webhook --target-instance <ainst-…> …` first fails with "--prompt is required with --agent" although `--agent` was not passed. With `--prompt` added it succeeds, storing `agent_name: "ainst-…"` and `target_kind: agent_instance`. A signed delivery then returns HTTP 404 `store: execute sqlite write callback: trigger_target_instance_not_found`. Using the agent name works. Expected: either resolve the ID or reject it at create time; `--target-instance` alone should not demand a prompt or name `--agent` in its error.

## Re-walk 2026-11-03 on 4056dbcc2

Fixed (`docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/rewalk-trigger-create.txt`, `docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/rewalk-webhook.txt`).
- `--target-instance ainst-…` is rejected at create time: "trigger.agent_name \"ainst-…\" is an agent instance id; target a stays-on agent by its agent name".
- `--target-instance inbox` without `--prompt` creates the trigger.
- A signed delivery completed with `session_id: null`, and the check produced a proposal.
