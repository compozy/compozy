---
id: ET-site-404-og-feeds-llms
area: ET
title: 404, share images, feeds and llms output survive the redesign
persona: Dora
journey: J-evaluate-compozy-beta
expected: An unknown URL renders the 404 in the landing type ramp on the dark surface with links to home and docs. The OG images for home, a docs page and a blog post render at 1200×630 in the new palette, type and logo, on unchanged routes; the home image's footer rail reads "OPERATING SYSTEM FOR AI AGENTS · compozy.com · KEEP EVERY AGENT RUNNING". /blog/feed.xml and /changelog/feed.xml are valid RSS with every item. /llms.txt lists every docs section including /docs/sdk/**, /llms-full.txt is complete, and /llms.mdx/docs/<page> returns that page's Markdown.
entry_points: compozy.com /does-not-exist; /opengraph-image; /og/docs/getting-started/installation/image.png; /og/blog/introducing-compozyos/image.png; /blog/feed.xml; /changelog/feed.xml; /llms.txt; /llms-full.txt; /llms.mdx/docs/getting-started/installation
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/site-redesign/qa/R4/ET-site-404-og-feeds-llms/walk.jsonl; .compozy/tasks/site-redesign/qa/R4/ET-site-404-og-feeds-llms/404-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-404-og-feeds-llms/404-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-404-og-feeds-llms/og-home.png; .compozy/tasks/site-redesign/qa/R4/ET-site-404-og-feeds-llms/og-docs.png; .compozy/tasks/site-redesign/qa/R4/ET-site-404-og-feeds-llms/og-blog.png; .compozy/tasks/site-redesign/qa/R4/ET-site-404-og-feeds-llms/llms.txt; .compozy/tasks/site-redesign/qa/R4/ET-site-404-og-feeds-llms/installation.md; .compozy/tasks/site-redesign/qa/R4/regression-sweep.json
last_report: docs/qa/reports/2026-11-10-site-redesign-r4.md
overlaps: ET-site-landing; ET-site-docs-sdk-reference
---

E2E-009 of the site redesign spec, story US-002. Also the overall regression walk: every marketing and docs surface at 1440 and 390 with no horizontal overflow and no console error.

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft.

QA result 2026-11-10 (R4 walk, `next start`, 1440 and 390, dark): pass. `/does-not-exist/` answers 404 with the landing type ramp on the dark surface and links to home and docs. `/opengraph-image`, `/og/docs/getting-started/installation/image.png` and `/og/blog/introducing-compozyos/image.png` are 1200×630 PNGs in the new palette, type and logo; the home rail reads OPERATING SYSTEM FOR AI AGENTS · compozy.com · KEEP EVERY AGENT RUNNING (blog posts share their cover as `og:image`, as on `main`). Both feeds are valid RSS 2.0 with every item (49 releases, 8 posts). `llms.txt` lists every docs section including 36 SDK links, `llms-full.txt` holds all 813 pages, and `/llms.mdx/docs/getting-started/installation` returns that page's Markdown. Regression sweep: 31 routes at both widths answered as expected with no overflow, no page error and no console error beyond the documented noise.
