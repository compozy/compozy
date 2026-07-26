# Real-Scenario QA Playbook Catalog

Select exactly one startup project per real-scenario run. Bootstrap materializes
its workspaces, knowledge, agents, and open task tree; the operator posts one
in-persona kickoff; the runtime drives the work; the auditor verifies real
deliverables, collaboration, and disruption recovery.

## Selection

| Playbook | Pick when the change touches… | Stress profile | Required deliverables |
| --- | --- | --- | --- |
| [`northstar-pay`](playbooks/northstar-pay.md) | Network channels, peer messaging, multi-corridor coordination, regulated copy | High channel volume, partner timeouts, claim compliance | 2 tsx_page, 2 tsx_component, 1 go_service_stub, 2 ts_test, 1 shell_script, 1 runbook_md |
| [`devtool-oss-launch`](playbooks/devtool-oss-launch.md) | CLI, release pipelines, docs, or benchmark harnesses | Benchmark regression, signing failure, undocumented breaking change | 1 go_service_stub, 2 python_script, 1 shell_script, 1 tsx_page, 1 tsx_component, 1 ts_test, 1 runbook_md, 1 spec_md |
| [`consumer-saas-growth`](playbooks/consumer-saas-growth.md) | Persistence, tasks, segmentation, Web read models, or lifecycle automation | Silent telemetry loss, assignment skew, lifecycle misfire | 2 tsx_page, 1 tsx_component, 2 ts_module, 2 ts_test, 1 sql_migration, 1 runbook_md, 1 spec_md |

When the user does not select one, read the previous run's `PLAYBOOK_REF` from
its manifest and choose the next slug alphabetically. Rotation prevents one
scenario from becoming a memorized fixture.

## Base contract

Bootstrap derives `scenario-contract.json` minimums from the selected playbook:
agent/role/channel counts, open roots, review dependencies, task runs,
disruption probes, deliverable reuse, and required collaboration. Do not
hand-edit the derived contract. The auditor checks base C1–C14 plus playbook
C15–C18.

`real-scenario-qa` always resolves a playbook. A generic bootstrap without one
is infrastructure for other QA flows, not real-scenario evidence.

## Add a playbook

1. Add `references/playbooks/<playbook-ref>.md` with one canonical fenced
   `json` block at the end; prose above it is narrative only.
2. Conform to `references/playbook-schema.json` and validate with the read-only
   helper:
   `python3 .agents/skills/agh/real-scenario-qa/scripts/validate-playbook.py --repo-root . --playbook "<playbook-ref>"`
3. Add one selection row above.
4. Make every `kickoff_brief` and agent `system_prompt` in-persona and compliant
   with `references/forbidden-prompt-phrases.md`.
5. Require at least four non-Markdown deliverables. Deliver disruption probes
   through `knowledge_file`, `channel_message`, `task_event`, or
   `config_change`, never a direct agent prompt.
6. Smoke-test bootstrap with a temporary scenario and tear down its emitted
   manifest on every terminal path.

## Synchronization boundary

`northstar-pay` mirrors the scenario identities used by
`web/src/storybook/fintech-scenario.ts` and its Network, Knowledge, and
Workspace fixtures. Synchronization is review-driven: when those identifiers
move, update the playbook in the same change rather than importing Web fixtures
into the QA loader.
