---
id: TA-web-automation-preview-toggle
area: TA
title: Automation live preview is a footer toggle, and a blocked save still says why
persona: Dora
journey:
expected: The job and trigger editors open on the form alone — there is no permanent right-hand preview rail, and the host is the 880px `lg` dialog rather than the 1180px `xl`. A ghost "Show live preview" button sits on the leading edge of the footer beside Cancel and the single primary; pressing it swaps the whole body for the preview (summary, next runs, run digest, rendered prompt, sample event, webhook endpoint, request payload) and relabels itself "Back to form". The button carries `aria-pressed` reflecting the current view. Cancel and the primary stay reachable in both views. Critically, a target that blocks saving must state why in the *form* view — `preview.targetIssue` renders as a warning Alert pinned to the top of the body, so a disabled primary is never a dead end with its only explanation hidden behind the toggle. The draft survives the swap in both directions, including cron/interval/at values, filter conditions, and reliability settings; the Reliability & limits disclosure keeps its open state across the round trip because that state is held above the swapped subtree. Schedule readouts stay inline in CronBuilder/ScheduleEvery/ScheduleAt, so the tightest editing feedback loop does not require opening the preview at all. Scope now sits in the toolbar band under the header, matching every other modal that has it, rather than as the first in-body section; the webhook-is-always-global rule moves into the body as a neutral Alert because it is runtime truth and cannot live in a one-line bar. The agent target is the shared searchable `AgentCommandSelect` reading the workspace catalog — the native `<select>` and free-text fallback are gone (`MODAL-STANDARD.md` § Component grammar forbids them), so a job or trigger can only name an agent the runtime actually has.
entry_points: web automation job editor (create + edit); web automation trigger editor (create + edit)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-web-entity-modal-shell; LP-017; LP-047; TA-052; TA-054; TA-055
---

story: As someone writing a scheduled job I want the form to have the whole width, and the preview available when I want to check my work — not permanently eating 468px I did not ask for.

The always-on rail forced the `xl` host and squeezed the form column to roughly 664px. Moving the preview behind a footer toggle returns the width to the form and lets the host drop to `lg`.

The known trade-off: side-by-side editing is gone, so tuning a cron while watching next-run times now needs a toggle. `preview.scheduleReadout` already renders inside the schedule controls themselves, which covers the highest-frequency case; the preview is for checking the assembled request. QA should confirm that trade-off is acceptable in practice and flag it if the round trip feels costly for cron authoring specifically.

The hidden view is unmounted, not `hidden` — a hidden-but-mounted preview would keep passing DOM queries while being invisible to a person.

src: web/src/systems/automation/components/automation-job-form.tsx; web/src/systems/automation/components/automation-trigger-form.tsx; web/src/systems/automation/components/automation-editor-dialog.tsx; web/src/systems/automation/components/reliability-controls.tsx

inventory: Needs QA
