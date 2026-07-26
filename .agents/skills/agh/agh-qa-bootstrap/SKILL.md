---
name: agh-qa-bootstrap
description: Bootstraps isolated AGH QA labs and emits the canonical manifest for downstream QA. Use when local QA needs daemon, workspace, provider, or playbook setup. Do not use for unit tests, planning-only work, or browser-only checks.
trigger: explicit
argument-hint: "[scenario-slug] [--playbook <ref>]"
---

# AGH QA Bootstrap

Provision infrastructure and scratch evidence for one QA pass. Bootstrap does
not prove runtime behavior; downstream QA owns journeys and verdicts.

## Required Inputs

- **scenario-slug** (optional): short lab context; defaults to `release-candidate`.
- **--playbook <ref>**: required when called by `real-scenario-qa`; omitted for generic `qa-execution` infrastructure.

## Procedure

**Step 1: Create or Resume One Lab**

1. Resolve the scenario slug and, for `real-scenario-qa`, validate the playbook under `.agents/skills/agh/real-scenario-qa/references/playbooks/`.
2. For a new pass, run the bootstrap helper (bootstrap, mutating):
   `python3 .agents/skills/agh/agh-qa-bootstrap/scripts/bootstrap-qa-env.py --scenario "<scenario-slug>" --repo-root . [--playbook "<ref>"]`
3. Reuse only when continuing the same active QA session or loop with its exact manifest:
   `python3 .agents/skills/agh/agh-qa-bootstrap/scripts/bootstrap-qa-env.py --scenario "<scenario-slug>" --repo-root . [--playbook "<ref>"] --reuse-manifest "<manifest-path>"`
4. Record the emitted `BOOTSTRAP_MANIFEST`; do not reconstruct environment values manually.

*Done when:* one fresh or same-session lab has one readable manifest and no unrelated lab was reused.

**Step 2: Verify the Handoff Contract**

1. Read `.agents/skills/agh/agh-qa-bootstrap/references/bootstrap-contract.md` in full.
2. Validate every required manifest field, env value, scratch-evidence file, provider-home policy, browser policy, audit command, and teardown command against that contract.
3. When a playbook was supplied, also validate the materialized playbook, agents, open-task tree, knowledge files, required deliverables/collaboration, and populated charter.
4. Treat scaffolding as empty evidence until downstream QA records real actions and the strict auditor passes.

*Done when:* manifest, env, paths, scenario contract, charter, playbook artifacts, and filesystem all describe the same isolated lab with no placeholders in a playbook run.

**Step 3: Hand Off to Downstream QA**

1. Pass `qa-docs-path=docs/qa` to `qa-report` and `qa-execution`; keep `QA_OUTPUT_PATH` as lab-side scratch evidence only.
2. Launch providers, Web, browser, and config operations exactly from the manifest contract. Preserve operator home for `native_cli + home_policy=operator`; use isolated provider homes for bound-secret or brokered lanes.
3. Export the manifest-derived `AGH_WEB_API_PROXY_TARGET` for Web QA and serialize config writes against one isolated home.
4. Register every long-lived process immediately under `<QA_OUTPUT_PATH>/qa/pids/`.
5. Require downstream QA to run the manifest's strict `AUDIT_COMMAND` before a behavior-first verdict.

*Done when:* downstream skills consume the canonical manifest, durable findings land under `docs/qa/`, scratch evidence stays in the lab, and every owned process is registered.

**Step 4: Preserve Continuation State**

1. Report the manifest path, lab root, `AGH_HOME`, base URL, dated run-report path, and reuse status.
2. For a continuing loop, append the exact machine-readable continuation block from the bootstrap contract.

*Done when:* the next continuation can identify the same lab from one exact manifest without discovery or guessing.

**Step 5: Tear Down on Every Terminal Path**

1. On PASS, FAIL, BLOCKED, or abort, run the manifest's exact `TEARDOWN_COMMAND`; only a confirmed continuation of the same loop may defer it.
2. The teardown must stop the daemon, tmux server, registered PIDs, lab-root processes, and lab-port listeners without touching unrelated labs.
3. Cite `<QA_OUTPUT_PATH>/qa/teardown.json` with `"clean": true`. Survivors are a blocking failure.
4. Reserve `make qa-reap` for intentional stale-lab recovery across the machine, not normal cleanup of one active run.

*Done when:* the current lab's teardown evidence is clean and no process owned by this pass remains alive.

## Error Handling

- **Requested reuse fails health checks:** use the fresh manifest emitted by the helper; do not revive stale state manually.
- **Provider reports global config errors:** compare the command's home policy with the manifest; isolated and operator-home lanes intentionally differ.
- **Web reaches the wrong daemon:** restart the Web process with the manifest-derived proxy target.
- **Teardown reports survivors:** inspect each `TEARDOWN_SURVIVOR`, stop only processes owned by the current lab, and rerun the same targeted teardown.
