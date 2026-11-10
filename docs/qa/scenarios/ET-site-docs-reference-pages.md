---
id: ET-site-docs-reference-pages
area: ET
title: CLI command pages and API resource pages follow the docs reference templates
persona: Dora
journey: J-evaluate-compozy-beta
expected: /docs/cli/** matches the docs draft states cli and cli-group: the signature with copy, examples with their captions (one-line commands as a prompt line, longer examples with syntax colours), the options table with short flags and types, inherited options, the subcommand list, and the remaining generated sections as prose. /docs/api/<tag> follows the api state for all 31 resources: the endpoint index, then each endpoint with parameters and responses beside the request samples (tabs) and the response example. Pages stay generated from openapi/compozy.json and the Cobra sources.
entry_points: compozy.com /docs/cli/session/prompt; /docs/cli/session; /docs/cli/spawn; /docs/api/agents; /docs/api/sessions; draft web/src/drafts/docs (states cli, cli-group, api)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-site-docs-api-reference-ui; ET-site-docs-shell; SITE-app-sdk-reference
---

E2E-007 of the site redesign spec, stories US-014 and US-015. Switch every sample tab on one endpoint and copy one sample.

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft.
