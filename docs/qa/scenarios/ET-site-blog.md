---
id: ET-site-blog
area: ET
title: The blog front page finds a post and the post reads comfortably
persona: Dora
journey: J-evaluate-compozy-beta
expected: /blog matches the blog draft states index, category, search and empty: cover story, three posters, a numbered index by month, and category, topic and search filters; every post shows its new cover. /blog/<slug> matches the post, callouts and launch states: opening, fact strip, numbered sections, the section rail with progress, code blocks, callouts, author card and "Keep reading". Tables stay inside the reading column without overflow. /blog/feed.xml is unchanged.
entry_points: compozy.com /blog; /blog/categories/<category>; /blog/introducing-compozyos; /blog/feed.xml; draft web/src/drafts/blog
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-site-landing
---

E2E-005 of the site redesign spec, stories US-009 and US-010. Exercise each filter, open a post from the index, and follow the section rail to a heading anchor.

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft. Reduced motion must keep every section readable.
