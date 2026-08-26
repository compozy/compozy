---
id: ET-session-composer-skill-chip
area: ET
title: Skill commands render as inline chips in the composer
persona: Théo
journey: J-use-session-slash-commands
expected: Selecting a skill from the slash menu renders it inside the composer as an inline chip (icon plus Title Case label, info tone, no fill) followed by a space, with the caret continuing after it; the outgoing prompt text keeps the exact canonical token; one Backspace over the chip removes the whole token; arrow keys jump over the chip as a unit; a restored draft containing a known skill token re-materializes the chip; built-in and agent commands stay plain text.
entry_points: web session composer; web session deep link
qa_status: pass
bug_ids: BUG-20260826-namespaced-skill-label-collapses
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/browser-e2e
last_report: docs/qa/reports/2026-08-25-skill-sources.md
overlaps: ET-session-slash-commands-inline;ET-web-session-composer-text-entry
---

QA impact 2026-08-05: new user-visible behavior from the composer redesign — inline skill chips in the prompt editor (Lexical), mirroring the transcript's verified skill pills. The wire contract is unchanged: the daemon still receives the raw token text and returns skill_invocations; the chip is presentation only. Walk chip insertion at start and mid-text, atomic deletion, caret traversal, draft restore re-materialization, and confirm the transcript echo matches the sent token.

QA impact 2026-08-25 (skill sources): already `untested`, and this cycle touched the same composer menu. Skill rows from a non-Compozy source now render a discreet mono origin label in the trailing slot, and physical homonyms from two roots appear as separate rows with distinct qualified tokens. The chip contract itself is unchanged — the wire still carries the raw canonical token — so the risk is presentational collision between the origin label and the chip's own label, and a homonym whose chip cannot be told apart from its twin. Rides along in `CH-skill-session-suppression-matrix`.

QA execution 2026-08-26: production E2E-010 reproduced and fixed the homonym-label collision.
The picker now renders the bare `commit-hygiene` winner and its `agents:commit-hygiene` collision as
distinct rows while preserving the `agents` origin. Focused catalog projection tests and the rebuilt
browser flow passed; the screenshot is committed under `docs/qa/evidence/2026-08-25-skill-sources/`.
