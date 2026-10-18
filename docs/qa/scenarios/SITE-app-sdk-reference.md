---
id: SITE-app-sdk-reference
area: SITE
title: Use generated SDK examples from the API reference
persona: Ana
journey: J-build-external-app
expected: The Build an app category is discoverable and each operation shows matching SDK samples in curl, TypeScript SDK, Go SDK, Python order, with supported browser operations derived from the generated manifest.
entry_points: /docs/apps; /docs/api/tasks; /docs/api/sessions; /docs/apps/browser
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Open the authored category and follow links to all five guides. At listTasks, copy the TypeScript and Go samples and call the same daemon; use the supported in_progress status. At sendSessionPrompt, verify examples use prompt and Prompt, then inspect an SSE operation using events and Next. Confirm unsupported Go operations omit the Go SDK tab instead of showing a nonexistent method. Compare the browser table with the current generated app_access allowlist. Record the actual pages and package/document versions; source-only projection tests are supporting evidence, not the rendered walk.

QA impact 2026-10-17: App SDK implementation adds this user-visible journey. Authored planning only;
no live execution verdict is recorded. Execute in the assigned isolated lab and attach current-head
evidence before marking it passed.
