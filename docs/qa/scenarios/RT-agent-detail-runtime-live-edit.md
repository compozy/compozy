---
id: RT-agent-detail-runtime-live-edit
area: RT
title: Agent detail live runtime selector mutation
persona: Bruno
journey: J-31
expected: The agent detail Overview Runtime card and settings runtime selectors render workspace-scoped effective provider·model·reasoning with inherited provenance while authored fields stay blank; an explicit selection submits a version-aware agent override, “Use project defaults” clears every authored runtime axis, and conflicts keep server truth recoverable.
entry_points: web /agents/$name?tab=overview; PUT /api/agents/:name
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-076;RT-078
---

Added by agent-details remediation 2026-07-12 for the new live runtime control on the detail header.

QA impact 2026-07-22: inherited project defaults are now visible in detail and settings before any agent override is authored. Status remains untested.

QA impact 2026-07-22: the live runtime selector moved from the agent-detail topbar into the Overview Runtime card as the Model category above Command. Status remains untested; next QA cycle owns live retesting.

QA impact 2026-07-22: extension-provided agents now persist live runtime selections through their effective authored definition, refresh the catalog immediately, and retain the selection after daemon restart. The bundled dev-cycle agents also appear under the Compozy category. Status remains untested.
