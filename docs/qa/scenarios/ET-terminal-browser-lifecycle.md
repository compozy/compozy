---
id: ET-terminal-browser-lifecycle
area: ET
title: Use persistent terminals from the browser
persona: Marina
journey: J-operate-integrated-terminal
expected: A project starts with an honest empty state; opening two terminals creates distinct tabs, switching and reloading preserves both, and closing the window does not end a running terminal that can be reattached later.
entry_points: Web dock Terminal app; /terminal; /terminal/{terminal_id}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Open Terminal in a project with no terminals and use the empty-state action.
2. Open a second terminal, switch between both tabs, and reload the browser.
3. Close the Terminal window while one command is still running, reopen the app, and reattach.
4. Confirm the terminal list, route, active tab, and dock badge remain truthful throughout.

