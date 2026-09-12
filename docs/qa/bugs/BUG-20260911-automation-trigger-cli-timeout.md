# BUG-20260911-automation-trigger-cli-timeout: Manual job trigger reports failure after starting work

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea, triggering a local outline job during the J-26 ingress walk; Bruno owns the canonical automation scenario.
- **Journey Step:** Force an immediate run and obtain its identity.
- **Scenarios:** TA-053; partial adjacent observation during TA-095
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction

During CH-untested-030-26-lea / Feature Tour on a172d49ff, create a disabled, one-time workspace job for field-writer with the literal prompt `/goal draft Prepare a concise weekly field-notes outline with sections for completed work, open questions, and next steps.` Then run `compozy automation jobs trigger job-0e45c4661ff87d72 -o json`. The CLI exits 1 with `Client.Timeout exceeded while awaiting headers`, rather than returning the started run. Public `automation jobs history` immediately identifies run-f59779d73dcfabbf / sess-0eff8e0457915a01 as running. It later reports completed, attempt 1, 2026-09-11T21:03:31.395295Z through 21:05:00.114503Z. No retry was issued. The job was deleted after completion and the lab is cleanly stopped.

## Evidence and current boundary

Evidence under docs/qa/evidence/2026-09-10-qa-execution-unblock/: ta095-job-create.json, ta095-job-trigger.json, ta095-job-history.json, ta095-auto-history-current.json, ta095-auto-events-current.json, ta095-job-delete.json, goal-ingress-partial-proof.json. The agent used Cursor/grok-4.6/high/fast; Compozy created no Goal Run from its literal input. The observed failure is response delivery, not job cancellation. A user who retries the apparent failure could request another run.

Root cause is not yet diagnosed. Determine whether the trigger handler waits for agent completion beyond the CLI transport deadline and confirm the documented response contract before choosing a repair. Existing CLI automation/client and automation manager suites should own the regression; do not add a live provider dependency to automated tests. The user stopped further QA/fix work for PR delivery, so this finding remains open.
