---
id: ET-site-docs-sdk-reference
area: ET
title: The SDK Reference teaches the client in one language and looks up one method
persona: Ana
journey: J-build-external-app
expected: /docs/sdk matches the docs draft sdk state (install, the create-session-and-prompt example, the four client pages, the resource grid), and /docs/sdk/client, /prompt, /events and /errors match sdk-client. The TypeScript/Go switch persists across SDK pages in the same visit, including after a reload. /docs/sdk/<resource> matches sdk-resource and sdk-go: the method filter narrows the index, "show all" restores it, and each method shows the call shape, return type, options, HTTP route, app-token access and the sample. A deep link to one method (/docs/sdk/<resource>#<method>) lands on that method below the fixed header. SDK pages appear in the sidebar, in search, in llms.txt and as per-page Markdown.
entry_points: compozy.com /docs/sdk; /docs/sdk/client; /docs/sdk/sessions; a /docs/sdk/sessions#<method> deep link; /llms.txt; /llms.mdx/docs/sdk/sessions; draft web/src/drafts/docs (states sdk, sdk-client, sdk-resource, sdk-go)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: SITE-app-sdk-reference; ET-site-docs-shell; ET-site-docs-reference-pages
---

E2E-008 of the site redesign spec, stories US-017 and US-018. Walk: switch to Go on /docs/sdk, open a resource page and confirm Go is still selected, type a filter, clear it with "show all", then open a method deep link in a fresh tab. Methods and samples come from openapi/compozy.json and openapi/sdk-samples.json; a new operation must appear without a hand edit.

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft.
