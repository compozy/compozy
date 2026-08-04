---
id: ET-site-docs-first-session
area: ET
title: Complete the documented first-session lifecycle
persona: Dora
journey: J-evaluate-compozy-beta
expected: A reader installs and bootstraps CompozyOS, starts the daemon, creates a session through cwd workspace inference, selects a durable prompt runtime, attaches with `session resume` only while live, stops the agent process, then sends a normal prompt that continues the same session and history.
entry_points: README Quick Start; /docs/getting-started/installation; /docs/getting-started/quick-start; /docs/sessions/lifecycle
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-04-durable-acp-sessions-docs/CH-durable-session-docs-continuity-migration.png; docs/qa/evidence/2026-08-04-durable-acp-sessions-docs/CH-durable-session-docs-continuity-lifecycle.png
last_report: docs/qa/reports/2026-08-04-durable-acp-sessions.md
overlaps: REL-beta-install-paths; ET-compozy-public-brand-navigation
---

QA pass 2026-08-04: the production-built site rendered the migration, Web UI, lifecycle,
resume, and CLI runtime set/clear pages with one consistent distinction: prompts restart stopped
work while preserving the same session and history, and `session resume` only attaches to a live
process. Direct page reloads passed, and browser error collection was empty.
