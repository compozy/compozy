---
id: ET-site-docs-reference-pages
area: ET
title: CLI command pages and API resource pages follow the docs reference templates
persona: Dora
journey: J-evaluate-compozy-beta
expected: /docs/cli/** matches the docs draft states cli and cli-group: the signature with copy, examples with their captions (one-line commands as a prompt line, longer examples with syntax colours), the options table with short flags and types, inherited options, the subcommand list, and the remaining generated sections as prose. /docs/api/<tag> follows the api state for all 31 resources: the endpoint index, then each endpoint with parameters and responses beside the request samples (tabs) and the response example. Pages stay generated from openapi/compozy.json and the Cobra sources.
entry_points: compozy.com /docs/cli/session/prompt; /docs/cli/session; /docs/cli/spawn; /docs/api/agents; /docs/api/sessions; draft web/src/drafts/docs (states cli, cli-group, api)
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/site-redesign/qa/R4/ET-site-docs-reference-pages/walk.jsonl; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-reference-pages/cli-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-reference-pages/cli-1440-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-reference-pages/cli-group-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-reference-pages/api-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-reference-pages/api-1440-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-reference-pages/api-390-site.jpg
last_report: docs/qa/reports/2026-11-10-site-redesign-r4.md
overlaps: ET-site-docs-api-reference-ui; ET-site-docs-shell; SITE-app-sdk-reference
---

E2E-007 of the site redesign spec, stories US-014 and US-015. Switch every sample tab on one endpoint and copy one sample.

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft.

QA result 2026-11-10 (R4 walk, `next start`, 1440 and 390, dark): pass. `/docs/cli/session/prompt` shows usage with copy (the clipboard holds `compozy session prompt <id> <message> [flags]`), 9 examples, 17 options, 3 inherited options and output formats; `/docs/cli/session` lists its 50 subcommands; `/docs/cli/spawn` resolves. All 31 `/docs/api/<tag>` pages answer 200 and follow the api state: endpoint index (12 shown, then "Show all"), then each of the 79 session endpoints with parameters and responses beside request samples; the cURL, TypeScript SDK, Go SDK and Python tabs switch and a sample copies. CLI pages match their drafts within 144px; API pages are much taller than the draft because they render every endpoint and the full request bodies with the library toolbar (decided in R3).
