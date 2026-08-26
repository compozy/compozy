---
id: RT-session-spawn-removed
area: RT
title: Reject every deleted session spawn surface
persona: Bruno
journey: J-contain-and-audit-delegation
expected: The old CLI verb, HTTP and UDS route, native tool, schemas, and generated clients are absent while agent call remains the sole delegation surface.
entry_points: compozy spawn; HTTP and UDS POST /api/agent/spawn with {"agent":"reviewer","prompt":"review"}; compozy__session_spawn with {"agent":"reviewer","prompt":"review"}; internal/codegen/hostapi/catalog.json and native schema digests; openapi/compozy.json; web/src/generated/compozy-openapi.d.ts; packages/site/content/docs/cli; skills/compozy
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path; SITE-agent-comms-docs-area; ET-workspace-access-prompt-outcomes
---

Probe each former spawn entry point and confirm a normal not-found or unknown-command response with no compatibility alias.

"Absent" has to mean absent, not deprecated: no alias, no shim, no "formerly known as" hint pointing
at `compozy call`. The native catalog must not carry the tool or its schema digest, and the boot-time
bijective registration check must pass without it. Grep the shipped docs, the generated CLI and API
references and `skills/compozy/` for spawn vocabulary and confirm it is clean. Note that the internal
child-session engine is retained and still powers governed calls — this row is about the deleted
*public* surface, not about the mechanism.

Run `make codegen-check`, then sweep `openapi/compozy.json`,
`web/src/generated/compozy-openapi.d.ts`, `internal/codegen/hostapi/catalog.json`, the native tool
descriptor/schema-digest tests, `packages/site/content/docs/cli`, and `skills/compozy/` for
`session_spawn`, `compozy__session_spawn`, `/api/agent/spawn`, and the removed `compozy spawn` verb.
The checks must prove absence from generated and authored artifacts; searching implementation files
alone is insufficient.
