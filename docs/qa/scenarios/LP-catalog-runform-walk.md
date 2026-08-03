---
id: LP-catalog-runform-walk
area: LP
title: Find a loop in the catalog, start a run from the form, and read its contract on the detail page
persona: Lea
journey: J-recover-loop-node-failure
expected: The loops catalog opens as one roster split into Built-in and Custom groups, each carrying its own count, with search, a single latest-run status select, and a Rows|Cards switch. The status select offers the daemon's full vocabulary including `canceled`, so filtering for a status the workspace has none of lands on a truthful empty state with a Clear filters affordance rather than a fabricated row or a missing option; no `stop` verb or control appears anywhere on the page. Starting a run from a roster row opens the run form for that same loop: inputs are open and typed from the declared schema, limits are folded but still state their generations, budget state, and whether loop defaults apply, Dry run and Start run close the form column, and Start stays disabled until every required input is filled — submitting with a blank required input names the field and creates no run. The form states that it starts the loop over `http` and lists the other declared `start[]` kinds as text only, with nothing that edits the allowlist (ADR-018). When a run of the same loop is already live, the form says so with that run's id and a link to its page, and duplicates none of its controls. The loop detail page carries the contract (goal, definition of done, gate criteria, possible endings as plain text), a plain-language failure posture whose numbers trace to the effective lifecycle envelope and the node's authored timeout/deadline, the read-only steps DAG, and recent runs where a cooperatively canceled run reads `canceled` as a neutral terminal, never as a failure. Identity and status chrome stay the same entity across all three routes.
entry_points: web /loops; web /loops/:name/run; web /loops/:name; GET /api/workspaces/:ws/loops; GET /api/workspaces/:ws/loops/:name; GET /api/workspaces/:ws/loop-runs?loop=:name; POST /api/workspaces/:ws/loops/:name/run
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-operator-lifecycle-ui;LP-editor-authoring-walk;LP-cancel-vs-kill;LP-loop-run-deep-link
---

story: As an operator I find the loop I need, start it with the inputs it asks for, and read what it promises and how it has been ending — without leaving the three pages that make up arriving and using a loop.

design: docs/design/opendesign/loops/loops-catalog.html + loop-run-form.html + loop-detail.html

truthful-ui: the status filter is the generated `LOOP_RUN_STATUSES` vocabulary, not the facets of the loaded page, so a legal `?status=` value never disappears because the current roster lacks it. The failure-posture tiles are derived only from `effective_lifecycle` and authored node keys — a loop that declares no wait node gets no wait tile rather than a plausible sentence. `canceled` is the seventh terminal from ADR-008 and renders neutral; the run-level `stop` verb does not exist on any of the three surfaces. Start-binding authoring stays out (ADR-018 §3): declared kinds are read-truth text.

evidence-seed: visual-contract bundles at .compozy/tasks/loop-node-lifecycle/evidence/visual/task_10/vc-h1..vc-h4 (populated roster, arrive-and-use compose, contract + failure posture + canceled recent run, one-story continuity); Vitest WT-009..WT-010.

acceptance-walk: Use the Playwright-backed browser driver to filter the live roster by an absent legal status, start a real run from its matching row, reject a blank required input without creating a run, and read the same contract on detail. Probe the already-running notice, cancel the live run, and confirm neutral canceled state on fresh catalog, detail, structured CLI, and HTTP reads.

src: .compozy/tasks/loop-node-lifecycle/task_10.md
