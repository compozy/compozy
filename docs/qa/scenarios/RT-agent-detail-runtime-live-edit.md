---
id: RT-agent-detail-runtime-live-edit
area: RT
title: Agent detail live runtime selector mutation
persona: Bruno
journey: J-31
expected: The agent detail Overview Runtime card and settings runtime selectors render workspace-scoped effective provider·model·reasoning with inherited provenance while authored fields stay blank; an explicit selection submits a version-aware agent override, “Use project defaults” clears every authored runtime axis, and conflicts keep server truth recoverable.
entry_points: web /agents/$name?tab=overview; PUT /api/agents/:name
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-076; RT-078
---

Added by agent-details remediation 2026-07-12 for the new live runtime control on the detail header.

QA impact 2026-07-22: inherited project defaults are now visible in detail and settings before any agent override is authored. Status remains untested.

QA impact 2026-07-22: the live runtime selector moved from the agent-detail topbar into the Overview Runtime card as the Model category above Command. Status remains untested; next QA cycle owns live retesting.

QA impact 2026-07-22: extension-provided agents now persist live runtime selections through their effective authored definition, refresh the catalog immediately, and retain the selection after daemon restart. The bundled spec-cycle agents also appear under the Compozy category. Status remains untested.

QA impact 2026-09-28 (session-continue-fork review round 1, W3): Web settings saves and live runtime edits are full replacements, and they now carry the agent's authored `fallback_chain` (read-only `command_fingerprint` dropped) instead of erasing it. Regressions: `agent-detail-settings-search.test.ts` "Should carry the authored fallback chain through an unrelated settings edit" and `agent-runtime-control.test.tsx` "Should keep the authored fallback chain when the runtime changes". Real round trip against an isolated daemon built from the working tree: `agent create --fallback-route …` (two routes, one with `command=SEAT=2 codex --acp`), web draft code reads `GET /api/agents/rt-agent` and `PUT`s a prompt edit, and both the read and `AGENT.md` keep both routes with every field. Status remains untested for the browser walk.
