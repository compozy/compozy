---
id: LP-run-detail-story-redesign
area: LP
title: Read a live run as a plain-language story on the redesigned run detail
persona: Lea
journey: J-04
expected: The run detail answers Progress / Happening now / What happened / What happens next in plain language — goal + group bar from the latest generation, a live now-card linking the running node's task run, a newest-first story timeline with verbatim mono micro-labels, Usage (Time/Tokens/Cost estimate/Rounds with ∞ for unbounded caps) and About rails — while every operator fact (stop_when, verification, policies, watch spec, criteria, raw events, digest) stays reachable in the Inspect drawer; no template refs, CEL, fan-out/batch vocabulary, or status legend anywhere on the main surface, and controls render exactly per the §7 status matrix for all 11 statuses.
entry_points: web /loop-runs/:id; GET /loop-runs/:id; SSE /loop-runs/:id/events; topbar ⋯ Inspect
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-009;LP-014;LP-016;LP-044;LP-action-failure-detail
---

story: As an end user I open a running loop and understand where it stands, what it is doing right now, what already happened, and what comes next — without operator vocabulary — and as an operator I still reach every mechanical fact through Inspect.

design: docs/design/opendesign/loops/loop-run-detail.html + loop-run-detail-states.html (LOOP-RUN-REDESIGN-SPEC.md)

truthful-ui: every rendered value traces to a spec §4 field; cost always renders `~$` + `estimate`; the story derives only from replayed run events; no control renders for a transition the daemon rejects (§7).

evidence-seed: visual-contract bundles at .compozy/tasks/loop-run-redesign/evidence/visual/VC-01..05 (running / needs-approval / watching / paused / failed vs the canonical prototypes).

src: docs/design/opendesign/loops/LOOP-RUN-REDESIGN-SPEC.md
