# BUG-20261003-job-task-detail-hides-intent: Task jobs display an empty Agent target

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-24, inspect a saved scheduled task
- **Scenarios:** TA-052
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Bruno saves a job using Run task. Its preview and editor show the task title, description and
owner, but the detail labels it Agent: with no name and renders an empty Prompt. The operator
cannot inspect the intended work from the job's read surface, even though the daemon retained it.

## Reproduction

- **Charter:** CH-038 · Feature Tour · desktop 1512x862 / en-US / wifi-fast.
- **Environment:** Recovery Editorial / archive-editions, real daemon and production Web at 55651.
- **Build:** 62b58628b62a03048967a91543323d3cb427bb7b.

1. Open Jobs, choose Job and select Run task.
2. Enter Editorial November task handoff, a task title and description, and a future schedule.
3. Disable the definition and inspect the public live preview: it contains the task fields.
4. Create the job. Detail displays Agent: and an empty Prompt section.
5. Reload; the same incomplete detail remains.
6. Reopen Edit and independently read the job through HTTP/UDS/CLI: task fields are present.
7. Edit the description, save and reload: the editor keeps it but the detail still omits it.

**Expected:** the job read surface identifies its task and renders the same persisted intent and
owner as the create/edit preview.
**Actual:** the detail falls through the Agent presentation with empty agent/prompt fields.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/:

- automation-task-loop-task-preview.json/.png: public draft payload and task preview.
- automation-task-loop-task-reload.json and automation-task-loop-task-detail.png: persisted failure after reload.
- automation-task-loop-task-{http,uds,edited-readback}.json: independent saved task fields.
- automation-task-loop-bruno-ended.json: stopped 95-frame recording and complete debrief.

Dedup: BUG-20260713-loop-automation-shown-as-agent owns the older Loop branch. Its Loop-target
behavior passes the adjacent current walk; this finding is the separate task output path.

## Fix

- **Root cause:** AutomationDetailPanel renders the Loop/Agent projection but never checks the
  durable task body. The existing job-preview run digest already resolves task title/description
  defaults and owner identity correctly.
- **Approach:** reuse that pure run digest in the detail, compose the existing Section/PropertyRow
  primitives for its task body, and retain Agent/Loop behavior.
- **Governor:** a few Web files; no API, schema, data, permission or product-policy change.
- **Fix commit:** pending
- **Regression invariant:** persisted task intent and owner are readable without opening Edit.
- **Owning layer/suite:** Web job detail presentation, existing automation-detail-panel.test.tsx.
  Existing automation-job-form.test.tsx retains shared preview coverage; no standalone test file.

## Verification

The two new detail cases fail before the repair with the observed empty Agent label. After the
repair, 38 tests pass across the existing detail and form suites, preserving Agent/Loop behavior.
Root Turbo build, lint and typecheck also pass. React Doctor remains 93/100 with the same two
pre-existing automation-hook complexity warnings. No new diagnostic or suppression was added.

Fresh Bruno replay on a new current-build document displays the saved task intent with an
unassigned owner. Editing Owner to human:bruno, saving and reloading preserves the visible
identity and agrees with independent UDS. Agent and Loop job canaries still show their own
targets and content. Desktop and 390px compact screenshots were inspected; the 31-frame
recording is stopped.

Evidence: automation-task-detail-{red,green,web,doctor}.json/.log,
automation-task-detail-bruno-ended.json and automation-task-detail-bruno-{owner,compact}.png.
Required delivery gate and commit evidence: automation-task-detail-{gate,gate-status,audit,commit-proof}.json.
