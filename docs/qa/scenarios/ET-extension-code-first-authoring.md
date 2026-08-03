---
id: ET-extension-code-first-authoring
area: ET
title: Build a code-first extension from the embedded CLI templates
persona: Ada
journey: J-extension-kit-lifecycle
expected: Running `compozy extension init hello -t tool-provider-go`, then build and validate with declared agents, automation, and layouts, produces one immutable generation whose copied resources and generated manifest match the SDK definition and validate with no issues.
entry_points: `compozy extension init`; `compozy extension build`; `compozy extension validate`
qa_status: pass
bug_ids: BUG-20260802-scaffold-sdk-version;BUG-20260802-manifest-mcp-tool-handler
fix_status: fixed
retest_status: pass
fix_commits: 7866661;881a254
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
overlaps: ET-compozy-extension-contract-identity
---

Added by ext-improvs Task 03. Repeat the first-success path for all five embedded templates, confirm `build` never mutates an existing generation, and inspect structured output for the stamped SDK minimum version, positioned issues, and derived consent areas.

QA impact 2026-08-02: code-first `resources` now declares and copies agents, automation, and layouts
into the generated manifest. Reset to verify the complete kit rather than the earlier tool-only build.
