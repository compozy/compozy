---
id: ET-session-slash-commands-inline
area: ET
title: Insert a session skill command anywhere in a draft
persona: Théo
journey: J-use-session-slash-commands
expected: The session composer opens its native command menu after slash at the start or after whitespace, limits inline results to effective skills, replaces only the active query, preserves surrounding text and Unicode, and keeps the authored prompt exact after reload.
entry_points: web session composer; web session deep link
qa_status: untested
bug_ids: BUG-20260805-command-menu-hook-order
fix_status: fixed
retest_status: pass
fix_commits: f54e62b;acbbb25
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/console-before-fix.txt;/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/console-after-fix.txt;/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/screenshots/05-inline-command-hook-fix.png;/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/screenshots/02-adjacent-duplicate-inline.png;/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/exact-prompt-history.json;/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/screenshots/05-exact-prompt-after-refresh.png;/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/screenshots/07-effective-skills-narrow.png;/Users/pedronauck/dev/qa-labs/compozy-session-slash-review-20260805-071845-995212-lab/qa-artifacts/qa/console-reviewed-head.txt
last_report: docs/qa/reports/2026-08-05-session-slash-commands.md
overlaps: ET-web-session-composer-text-entry;RT-session-message-reload
---

QA impact 2026-08-05: session slash discovery and inline skill insertion are new user-visible behavior. This scenario owns the Web command-menu interaction and exact trigger-range replacement; the existing composer text-entry and transcript reload scenarios remain adjacent canaries.

QA verdict 2026-08-05: passed after one governed fix. A fresh browser walk opened, closed, and reopened the native assistant-ui popover, inserted `/browser-qa` in the middle of `Revisão 😊 /bro before launch`, preserved the Unicode prefix and suffix exactly, and reported no page or console errors. The 320 px menu exposed the same Built-in, Agent, and Skills groups with no axe violations.

QA impact 2026-08-05 (review remediation): reset because directive whitespace rendering and composer draft synchronization changed. Re-walk adjacent repeated directives, reload persistence, and exact surrounding text on the reviewed head.

QA verdict 2026-08-05 (reviewed head): passed in fresh lab `session-slash-review-20260805-071845-995212`. The native assistant-ui menu inserted adjacent `/browser-qa` markers after an emoji, the runtime retained the authored text `Revisão 😊 /browser-qa /browser-qa antes   do lançamento`, deduplicated the repeated skill to one source-qualified invocation, and restored the exact submitted prompt after reload. At 320 px, the Skills section exposed `/browser-qa` and `/compozy` but not the agent-disabled `/hidden-skill`; page and console errors were empty.

QA impact 2026-08-05 (composer redesign): reset — the slash menu was rebuilt as one flat categorized list (no Built-in/Skills drill-down), rows gained icons, humanized titles, inline descriptions, and token/scope trailing meta, and selecting a skill now materializes an inline chip instead of raw text. Re-walk trigger detection at start/mid-text, inline skill restriction, exact text preservation around the chip, and reload persistence.
