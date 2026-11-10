---
id: ET-site-landing
area: ET
title: The compozy.com landing matches the approved draft on live numbers
persona: Dora
journey: J-evaluate-compozy-beta
expected: / renders the approved landing draft section for section and in order at 1440 and 390 inside the shared floating header (logo, Docs, Marketplace, Blog, Changelog, Download, star button; compacts on scroll with the progress hairline; links in a menu below md) and the one-line footer. The hero equals the COPY.md Hero Lock. Star count, release count and provider count come from live site data, never the draft constants; an unavailable GitHub count hides its number instead of inventing one. The install tabs offer the desktop download and every terminal method with copy.
entry_points: compozy.com /; draft web/src/drafts/landing (state default)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: REL-os-landing-proof; ET-site-download
---

E2E-001 of the site redesign spec (`.compozy/tasks/site-redesign/_tests.md`), stories US-001 and US-003.

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft. Reduced motion must keep every section readable.

Also check that the current section is marked with `aria-current="page"` on every marketing route and that each header and footer link navigates to its real route.
