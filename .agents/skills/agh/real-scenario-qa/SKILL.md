---
name: real-scenario-qa
description: Dogfoods AGH through an autonomous startup scenario with live providers, cross-surface observation, and strict evidence audit. Use for release or complex-integration QA. Do not use for smoke, static, mock-only, or unit-test work.
trigger: explicit
argument-hint: "[playbook-ref]"
---

# Real Scenario QA

Execute release-grade QA by running an entire fictional startup project on the AGH runtime and observing the result. The runtime drives the work; the observer never tells agents they are being evaluated. The auditor enforces real deliverables (compiled/parsed/runnable artifacts) and real collaboration (peer messages, review cycles, disagreement resolution).

The skill rejects any prompt that frames the work as QA. See `references/forbidden-prompt-phrases.md`.

## Required Inputs

- **playbook-ref** (optional): Slug of the playbook to run (e.g., `northstar-pay`, `devtool-oss-launch`, `consumer-saas-growth`). When omitted, rotate from the previous run's `PLAYBOOK_REF` recorded in `bootstrap-manifest.json`.

## Procedures

**Step 1: Select the Playbook**

1. Read `.agents/skills/agh/real-scenario-qa/references/playbook-catalog.md` in full.
2. Resolve the playbook ref:
   - If the user supplied a slug, validate it exists at `references/playbooks/<slug>.md`.
   - Otherwise, list `references/playbooks/*.md` (excluding `README`) and rotate from the previous `PLAYBOOK_REF`.
3. Record `PLAYBOOK_REF`.

*Done when:* one valid playbook is selected and differs from the previous run when rotation applies.

**Step 2: Bootstrap the Lab With the Playbook**

1. Activate `agh-qa-bootstrap` with scenario `$PLAYBOOK_REF` and `--playbook "$PLAYBOOK_REF"`; complete its full procedure instead of calling its helper directly.
2. Consume the canonical `BOOTSTRAP_MANIFEST` and its emitted paths. Never reconstruct provider, browser, proxy, audit, or teardown state here.
3. Confirm the selected playbook, agent registrations, open-task tree, knowledge files, required deliverables/collaboration, and populated charter all belong to the same healthy manifest. Register only `RUNTIME_WORKSPACE_PATH` with AGH; agents must not see the lab's `qa-artifacts/` or audit contracts.

*Done when:* bootstrap's completion criteria pass and the charter has no placeholders.

**Step 3: Activate Companion Skills**

1. Use `qa-report` with `qa-docs-path=docs/qa` to plan the playbook-product validation as session charters (persona + journey + tour + time-box on the playbook deliverables — never on QA itself). `QA_OUTPUT_PATH` remains the lab-side scratch root (journey log, observation, kickoff evidence); the living QA state lives in the repo's `docs/qa/`.
2. Use `qa-execution` with `qa-docs-path=docs/qa` to run those sessions against the lab (does the TSX page render? do scripts run? does the canary control respond?), driving the lab's base URL/daemon from the bootstrap env block.
3. Use `agh-worktree-isolation` only when concurrency was explicitly signaled by the user.
4. Use `systematic-debugging` and `no-workarounds` for any unexpected runtime behavior the observer captures.
5. Apply provider-home, Web-proxy, config-write, PID-registration, and teardown policy directly from the bootstrap manifest.

*Done when:* the living QA plan and execution target the bootstrapped lab, with concurrency isolation activated only when signaled.

**Step 4: Post the Operator Kickoff**

1. After runtime agents, sessions, channels, and the deterministic task ids from `.agh/tasks/open-tasks.json` exist under the shared `RUNTIME_WORKSPACE_PATH`, prepare task activation behind a scheduler barrier (mutating):
   `python3 .agents/skills/agh/real-scenario-qa/scripts/activate-playbook-tasks.py prepare --workspace "$WORKSPACE_PATH" --qa-output-path "$QA_OUTPUT_PATH" --manifest "$BOOTSTRAP_MANIFEST" --agh-bin "${AGH_BIN:-agh}"`
2. Render and validate the kickoff payload (mutating only the inspectable payload file):
   `python3 .agents/skills/agh/real-scenario-qa/scripts/post-operator-kickoff.py --workspace "$WORKSPACE_PATH" --playbook "$PLAYBOOK_REF" --qa-output-path "$QA_OUTPUT_PATH" --manifest "$BOOTSTRAP_MANIFEST"`
3. The helper aborts with exit code 2 if the rendered kickoff contains any phrase from `references/forbidden-prompt-phrases.md`. Rewrite the playbook's `kickoff_brief` when blocked.
4. Read `<WORKSPACE_PATH>/.agh/operator-kickoff.txt`. Deliver that text verbatim once and capture the provider stream:
   `agh session prompt <operator-session-id> "$(cat $WORKSPACE_PATH/.agh/operator-kickoff.txt)" -o jsonl > $QA_OUTPUT_PATH/qa/operator-kickoff.jsonl`
5. Confirm the successful post from its non-empty evidence (mutating), then release the queued task runs (mutating):
   `python3 .agents/skills/agh/real-scenario-qa/scripts/post-operator-kickoff.py --workspace "$WORKSPACE_PATH" --playbook "$PLAYBOOK_REF" --qa-output-path "$QA_OUTPUT_PATH" --manifest "$BOOTSTRAP_MANIFEST" --confirm-posted "$QA_OUTPUT_PATH/qa/operator-kickoff.jsonl"`
   `python3 .agents/skills/agh/real-scenario-qa/scripts/activate-playbook-tasks.py release --workspace "$WORKSPACE_PATH" --qa-output-path "$QA_OUTPUT_PATH" --manifest "$BOOTSTRAP_MANIFEST" --kickoff-evidence "$QA_OUTPUT_PATH/qa/operator-kickoff.jsonl" --agh-bin "${AGH_BIN:-agh}"`
6. Confirm the manifest reports `KICKOFF_POSTED=true`, `KICKOFF_TIMESTAMP` is set, task activation is `released`, and the scheduler is unpaused. Send no further prompt to any agent under test; a stall becomes a bug.

*Done when:* every declared task has one queued run behind the barrier, exactly one evidenced kickoff is confirmed, dispatch is released, and the observer has no path for a second agent prompt.

**Step 5: Observe the Runtime**

1. Run the observer (read-only) for the configured window:
   `python3 .agents/skills/agh/real-scenario-qa/scripts/observe-runtime.py --workspace "$WORKSPACE_PATH" --qa-output-path "$QA_OUTPUT_PATH" --duration-sec 1800 --stall-threshold-sec 300`
2. While the observer is tailing the journey log, capture cross-surface evidence WITHOUT directing agents:
   - CLI: `agh task list`, `agh agent list`, `agh channel list`, `agh session list` against the isolated daemon.
   - API: read endpoints that intersect the playbook's primary domain.
   - Web: open the AGH web app via `browser-use:browser` (or the `agent-browser` fallback) against `$AGH_WEB_API_PROXY_TARGET`. Capture DOM snapshot, URL, screenshot.
   - Runtime: confirm the journey-log keeps growing.
3. Record observer-only or out-of-band evidence with the mutating helper `.agents/skills/agh/real-scenario-qa/scripts/record-scenario-action.py`; never use it to fabricate runtime-owned actions.
4. When the observer reports stall (exit code 1), open `<QA_OUTPUT_PATH>/qa/observation-summary.json`, identify the silent agent / unstarted task, and proceed to Step 6 with that diagnosis. Do not attempt to "wake" the agent with a prompt.
5. When the observer completes cleanly (exit code 0), proceed to Step 6.

*Done when:* the observation window or an explicit stall completes with indexed CLI, API, Web, runtime, and provider evidence and no observer prompt after kickoff.

**Step 6: Audit, Diagnose, Fix, Re-Verify**

1. Maintain the dated report at `docs/qa/reports/<YYYY-MM-DD>-<playbook-ref>.md` through `qa-report`/`qa-execution`; index lab-side evidence by path rather than copying it into the repository.
2. Diagnose and fix real runtime defects with `systematic-debugging` and `no-workarounds`; fix playbook authoring defects in the playbook source and restart from Step 2.
3. After the last code change, run the single full `make verify` completion gate and record its fresh evidence in the dated report.
4. Run the mutating strict auditor last, passing the durable report explicitly:
   `python3 .agents/skills/agh/real-scenario-qa/scripts/audit-qa-evidence.py --qa-output-path "$QA_OUTPUT_PATH" --final-report "docs/qa/reports/<YYYY-MM-DD>-<playbook-ref>.md" --strict`
5. Auditor exit code 2 is a blocking failure. Read `qa-audit-report.json` and act per check. All durable bugs go to the repo's global registry as `docs/qa/bugs/BUG-<YYYYMMDD>-<slug>.md` (dedup against the registry first, per `qa-report`'s bug-registry rules) and are linked into the affected `docs/qa/scenarios/*.md` files:
   - **C15** forbidden phrase in a prompt → rewrite the playbook source (system_prompt or kickoff_brief), not the auditor or the regex list.
   - **C16** deliverable count short → file a runtime bug (which AGH agent failed to produce the artifact, why, what state shows the failure). Do not author the missing artifact yourself — the runtime is what's under test.
   - **C17** collaboration loop short → file a runtime bug describing which channel, agent, or review cycle did not complete. Cite journey-log timestamps.
   - **C18** stall → the registry bug is mandatory and must name the silent agent and stalled task.
6. Re-run the relevant scoped checks after each fix; after source freezes again, refresh the single full gate evidence and rerun the strict auditor.
7. Update affected scenario verdicts and append the bootstrap continuation block only when the same active loop will continue.

*Done when:* the dated report, fresh final gate, scenario verdicts, strict audit, and indexed evidence all describe the same execution with no blocker.

**Step 7: Tear Down the Lab (MANDATORY)**

1. Complete `agh-qa-bootstrap` Step 5 using the current manifest's exact `TEARDOWN_COMMAND`. This applies on every terminal verdict — PASS, FAIL, BLOCKED, or abort.
2. Cite `<QA_OUTPUT_PATH>/qa/teardown.json` (`"clean": true`) in the final summary. Survivors (exit 1) are a blocking failure.
3. Only exception: an explicitly continuing timed loop keeps the lab alive; the continuation that ends the loop inherits the teardown obligation. A stalled or aborted run tears down like any other — the stall evidence lives in files, not in live processes.

*Done when:* the current lab's `teardown.json` reports `"clean": true` and no owned process survives.

## Error Handling

- If bootstrap fails to load the playbook, validate the playbook against `.agents/skills/agh/real-scenario-qa/references/playbook-schema.json`; a real-scenario run never falls back to a generic charter.
- If the kickoff helper aborts on a forbidden phrase, rewrite the playbook's `kickoff_brief`. Do not edit `references/forbidden-prompt-phrases.md` to remove the rule.
- If task activation preparation fails, keep the owned scheduler barrier paused, inspect `qa/task-activation.json`, and retry with the same idempotency keys. Never post the kickoff with a partial task tree.
- If kickoff delivery or confirmation fails, keep dispatch paused. Retry only the same unconfirmed delivery when no provider evidence exists; once evidence exists, confirmation is the only valid next step. Release refuses an empty kickoff transcript or an unconfirmed manifest.
- If `observe-runtime.py` reports a stall, do NOT inject a prompt to wake the agent. The runtime stall IS the bug under test. File it in `docs/qa/bugs/` against the AGH runtime.
- If a required deliverable type cannot be parsed by the auditor (e.g., a TSX file with non-standard exports), fix the artifact in the workspace via the agent that authored it (re-prompting in-persona is fine; new operator prompts are not). If the agent cannot fix it, that is a runtime bug.
- If `browser-use:browser` is unavailable, follow the `agent-browser` fallback per the bootstrap browser policy. Do not silently drop the Web surface.
- If providers are unreachable, record the boundary in `provider-attempt.json`. The run verdict becomes BLOCKED, never PASS.
- If the auditor's `playbook_compliance` block reports zero counts despite agents working, confirm `WORKSPACE_PATH/.agh/playbook.json` exists and `journey-log.jsonl` is being written. Empty counts often mean the runtime is not wired to the journey log — that is a runtime bug.
