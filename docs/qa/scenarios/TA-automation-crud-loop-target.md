---
id: TA-automation-crud-loop-target
area: TA
title: Create update disable and delete Loop-target automations
persona: Bruno
journey: J-24
expected: Job and trigger modals create valid Loop-target definitions, updates survive refresh, write-only webhook secrets are redacted from the request preview, disabled definitions do not fire, re-enabled definitions fire one real Loop run, and deletion removes only the chosen dynamic definition.
entry_points: web /jobs; web /triggers; web Loop Start bindings
qa_status: untested
bug_ids: BUG-20260713-loop-automation-shown-as-agent;BUG-20260713-loop-automation-start-mismatch-late;BUG-20260713-automation-delete-no-confirmation;BUG-20260713-workspace-trigger-loop-submit-inert;BUG-20260713-loop-watch-poll-error-stuck
fix_status: fixed
retest_status: pending
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-loop-job-target-fixed.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-loop-job-history-fixed.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-job-delete-confirmation-fixed.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-trigger-workspace-loop-fixed-created-final.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-trigger-system-stop-dispatch.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/trigger-loop-generation-zero-dispatch.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/trigger-deleted-after-generation-zero-proof.dom.txt;/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/trigger-loop-generation-zero-replay-0714.dom.txt
last_report: docs/qa/reports/2026-07-14-consumer-saas-growth.md
overlaps: LP-034;TA-065
---

Exercise schedule and event trigger types plus invalid/empty payload mappings.

2026-07-13: The target-aware fix passed a same-persona Job create/edit/Run-now replay: detail preserved typed Loop inputs and delegated history linked to `looprun-e39eb7e8d36ffa7b`. The replay found two residuals: incompatible Loop start kinds are offered as valid until backend rejection, and deletion happens on the first click without confirmation. Trigger create/edit/delete retest remains pending the residual fixes.

2026-07-13: Partial residual retest passed start-kind filtering and the complete safe Job delete lifecycle. Job offered only schedule-capable `software-delivery`; Trigger offered only `reviews-watch` for ordinary and webhook events. Job Delete required a dialog, Cancel preserved it, wrong-name stayed disabled, and exact-name confirmation removed it once. The same Trigger replay found BUG-20260713-workspace-trigger-loop-submit-inert: Workspace scope is not propagated into the nested Loop target, and enabled submit silently creates nothing.

2026-07-13: The Trigger residual is fixed and browser-verified. `qa-workspace-loop-trigger-fixed` was created from the modal with workspace `ws_06366aad69887872`, event `session.stopped`, filter `data.session_type=system`, Loop `reviews-watch`, and numeric input `pr=2`; the fresh detail surface read every value back and showed the expected zero-run baseline. Disable and re-enable both persisted immediately. Edit changed the fire limit 12 → 13, detail read it back, and a second edit restored 12 without losing scope, filter, target, or typed input. Delete opened the named confirmation, a wrong name kept the destructive action disabled, and Cancel preserved the enabled Trigger. The scenario remains open only for the matching real system-session event, run-history correlation, and final exact-name deletion.

2026-07-13: Matching dispatch passed exactly once. Stopping system session `sess-1e9a13013651c8b0` produced one Trigger history row and delegated `looprun-56929015a03ab48d` with the correct workspace/start correlation. The downstream watch poll failed deterministically, but the run remained Running at generation 0 for over two minutes and showed no cause until operator Stop marked it Failed; BUG-20260713-loop-watch-poll-error-stuck now blocks the integrated acceptance. Final Trigger deletion still waits for the fixed replay.

2026-07-14 fresh control: a newly created workspace Trigger matched one real stopped user session and delegated one Loop run. It terminalized at generation zero with the typed watch-source cause/recovery before the first refresh, stayed terminal after reload, and both disposable Trigger and session were deleted through their modals.

QA impact 2026-07-14: the Trigger request preview now redacts write-only webhook secret values while preserving the submitted draft. Planning update only; reset to untested without a QA replay.

QA impact 2026-07-14: Job Loop-target catalog lookup, scope transitions, validation, preview, and submission now share one workspace resolver. Reset remains untested pending global-explicit and workspace-rebind Browser controls.

2026-07-14 final-worktree control: global and workspace Jobs preserved Loop targets and typed inputs across create/read/update/list/delete; a mismatched workspace target was rejected. The complete Web E2E gate also passed typed Job and Trigger deletion. Retest promoted to pass.

QA impact 2026-07-18: package-backed Jobs and Triggers now accept enabled-only overlays through
Web, CLI/HTTP/UDS, and native automation tools while continuing to reject definition edits. Reset
to untested; no QA replay ran.

QA impact 2026-07-18: HTTP/UDS now rejects deletion of config- and package-managed Jobs/Triggers,
matching native tools, and reports the shared managed-resource cause instead of mislabeling package
definitions as config-backed. Dynamic definitions remain deletable.
