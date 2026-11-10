---
id: ET-site-docs-sdk-reference
area: ET
title: The SDK Reference teaches the client in one language and looks up one method
persona: Ana
journey: J-build-external-app
expected: /docs/sdk matches the docs draft sdk state (install, the create-session-and-prompt example, the four client pages, the resource grid), and /docs/sdk/client, /prompt, /events and /errors match sdk-client. The TypeScript/Go switch persists across SDK pages in the same visit, including after a reload. /docs/sdk/<resource> matches sdk-resource and sdk-go: the method filter narrows the index, "show all" restores it, and each method shows the call shape, return type, options, HTTP route, app-token access and the sample. A deep link to one method (/docs/sdk/<resource>#<method>) lands on that method below the fixed header. SDK pages appear in the sidebar, in search, in llms.txt and as per-page Markdown.
entry_points: compozy.com /docs/sdk; /docs/sdk/client; /docs/sdk/sessions; a /docs/sdk/sessions#<method> deep link; /llms.txt; /llms.mdx/docs/sdk/sessions; draft web/src/drafts/docs (states sdk, sdk-client, sdk-resource, sdk-go)
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/site-redesign/qa/R4/ET-site-docs-sdk-reference/walk.jsonl; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-sdk-reference/sdk-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-sdk-reference/sdk-1440-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-sdk-reference/sdk-client-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-sdk-reference/sdk-resource-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-sdk-reference/sdk-go-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-sdk-reference/sdk-go-390-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-sdk-reference/deeplink-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-sdk-reference/deeplink-390-site.jpg
last_report: docs/qa/reports/2026-11-10-site-redesign-r4.md
overlaps: SITE-app-sdk-reference; ET-site-docs-shell; ET-site-docs-reference-pages
---

E2E-008 of the site redesign spec, stories US-017 and US-018. Walk: switch to Go on /docs/sdk, open a resource page and confirm Go is still selected, type a filter, clear it with "show all", then open a method deep link in a fresh tab. Methods and samples come from openapi/compozy.json and openapi/sdk-samples.json; a new operation must appear without a hand edit.

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft.

QA impact 2026-11-10 (review fixes R3): every method is in the server HTML; the index shows a preview and "Show all N methods" opens the rest. Options list the names each language's client takes. Method counts are pluralised in pages and `llms.txt`.

QA result 2026-11-10 (R4 walk, `next start`, 1440 and 390, dark): pass. The sdk, sdk-client, sdk-resource and sdk-go states match their drafts (height within 557px at 390 for the client guide, the rest closer). Go chosen on `/docs/sdk` stays selected on `/docs/sdk/sessions` and after a reload, with Go samples; the filter "list" narrows the index from 98 to 20 methods and clearing it restores 98; "Show all 98 methods" opens the rest; each method shows its call, options, HTTP route, app-token access and sample; a deep link to `#getGlobalSessionHistory` in a fresh tab lands with the title directly under the sticky language bar. SDK pages appear in the sidebar, in search (`/docs/sdk/sessions#listSessions`), in `llms.txt` (36 links) and as per-page Markdown.
