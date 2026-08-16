---
id: ET-agent-plugin-source-install
area: ET
title: Install an Agent Plugins package from a directory or git source
persona: Bruno
journey: J-extension-distribution
expected: The same schema-1.0.0 package installs from a local directory and a git ref without a format flag, reports `format: agent-plugin` plus its ingested and skipped components, and becomes visible through every extension read surface after the normal unverified-source consent.
entry_points: compozy extension install <local-path>|git:<url>@<ref> --allow-unverified --yes; POST /api/extensions/install over HTTP and UDS; compozy__extensions_install; compozy extension list|status|inventory
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-extension-published-source-installs; ET-web-extension-union-install
---

QA impact 2026-08-16: Agent Plugins became a third detected package layout on the existing extension
source union. Task 08 must acquire one fixture from a directory and a real git ref, compare the
structured payloads, and prove no format-selection flag or separate lifecycle appears.

