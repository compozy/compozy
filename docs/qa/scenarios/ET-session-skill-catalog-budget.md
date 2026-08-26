---
id: ET-session-skill-catalog-budget
area: ET
title: Preserve every skill identity inside the prompt catalog budget
persona: Ada
journey: J-use-absorbed-skills-in-a-session
expected: When the effective skill catalog exceeds the prompt budget, the managed session receives one complete well-delimited catalog containing every enabled non-suppressed skill identity, with descriptions shortened before any identity or boundary is lost.
entry_points: managed Codex session startup prompt; compozy skill list -o json; compozy session commands <session-id> -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-skill-session-source-injection; ET-session-command-catalog-parity; ET-managed-session-skill-loading
---

QA impact 2026-08-26 (main rebase): added after the larger merged skill inventory pushed the
injected startup catalog past its prompt-section budget. Charter: `CH-session-skill-catalog-budget`.
