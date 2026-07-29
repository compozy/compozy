---
id: ET-site-docs-first-session
area: ET
title: Complete the documented first-session lifecycle
persona: Dora
journey: J-evaluate-compozy-beta
expected: A reader installs and bootstraps CompozyOS, starts the daemon, creates a session from a repository through cwd workspace inference, attaches with `session resume` while the session is live, inspects durable history, and stops only as terminal cleanup.
entry_points: README Quick Start; /docs/getting-started/installation; /docs/getting-started/quick-start; /docs/sessions/lifecycle
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: REL-beta-install-paths; ET-compozy-public-brand-navigation
---

QA impact 2026-07-29: the documented journey no longer claims a stopped session can be resumed.
The next QA cycle must execute the exact commands with an isolated home and prove live attachment,
terminal stop behavior, workspace resolution provenance, and retained history.
