---
id: LP-web-request-answer-card
area: LP
title: Answer a parked Loop request from the run page
persona: Bruno
journey: J-supervise-loop-request
expected: The Needs-you region presents pending requests one question at a time — the daemon's prompt as the question with lane and deadline beside it, a "Question i of N" progress header with previous/next when several wait, schema-generated inputs (enum values as selectable choices, booleans as Yes/No), and the redacted context preview, full-context fetch, and node/generation identity behind a closed-by-default Details disclosure; only persisted decisions render; requests from different generations stay distinct; context failures expose retry for the exact request; invalid answers remain pending with inline errors; valid answers resolve from refreshed truth with no optimistic paint; and requests answered elsewhere or on terminated runs show the recorded outcome without a form.
entry_points: /loop-runs/$runId Needs-you card; parked panels; waits rail
qa_status: blocked-verify
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: docs/qa/evidence/2026-08-18-request-questionnaire/questionnaire-pending.png;docs/qa/evidence/2026-08-18-request-questionnaire/questionnaire-resolved.png;docs/qa/evidence/2026-08-18-request-questionnaire/questionnaire-enum-route.png;docs/qa/evidence/2026-08-18-request-questionnaire/e2e-scoped.txt;docs/qa/evidence/2026-08-18-loop-ui-polish/needs-you-pending.png;docs/qa/evidence/2026-08-18-loop-ui-polish/e2e-scoped.txt;/Users/pedronauck/dev/qa-labs/compozy-graph-eng-review-20260818-141718-102629-lab/qa-artifacts/qa/screenshots/loop-request-repeated-generation.png
last_report: docs/qa/reports/2026-08-18-graph-eng-review.md
overlaps: ""
---

story: As a Loop operator, I can answer an ask or decide a review from the run page and trust that every control I see is one the daemon actually authorized.

src: .compozy/tasks/graph-eng/task_08.md

2026-08-18 loop-ui-polish: request cards flattened into rows of the single Needs-you shell (nested card border/radius removed), duplicate state glyph dropped, required fields marked with the canonical RequiredMark. Behavior unchanged — E2E-020/021/022/031 passing; blocked-verify because the walk contract (/qa-execution) is operator-invoked — walk pending.

2026-08-18 request-questionnaire: requests redesigned into a one-question-at-a-time questionnaire (shadcn questionnaire reference) — "Question i of N" progress + prev/next navigation, prompt as the primary title, humanized field labels, enum/boolean answers as RadioCard choices (Yes/No for booleans), context + full-context fetch + node/gen identity moved into a closed-by-default Details disclosure, operator lock note and mono request footer removed, settled requests as compact outcome rows. Deep-link focus and engaged-key auto-selection preserved. E2E-020/021/022/031 + enum-value test updated to the navigation model and passing scoped; unit suites updated (loop-run-page, loop-request-model, loop-request-payload). qa_status stays blocked-verify — the /qa-execution walk is operator-invoked and pending for the new interaction contract.
