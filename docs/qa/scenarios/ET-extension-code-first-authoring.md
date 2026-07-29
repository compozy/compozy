---
id: ET-extension-code-first-authoring
area: ET
title: Build a code-first extension from the embedded CLI templates
persona: Ada
journey: J-extension-policy-admin
expected: Running `compozy extension init hello -t tool-provider-go`, then `compozy extension build` and `compozy extension validate`, produces one immutable `dist/gen-<hash>` bundle whose generated manifest matches the SDK registrations and validates with no issues.
entry_points: `compozy extension init`; `compozy extension build`; `compozy extension validate`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-compozy-extension-contract-identity
---

Added by ext-improvs Task 03. Repeat the first-success path for all five embedded templates, confirm `build` never mutates an existing generation, and inspect structured output for the stamped SDK minimum version, positioned issues, and derived consent areas.
