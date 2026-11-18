---
id: ET-site-blog
area: ET
title: The blog front page finds a post and the post reads comfortably
persona: Dora
journey: J-evaluate-compozy-beta
expected: /blog matches the blog draft states index, category, search and empty: cover story, three posters, a numbered index by month, and category, topic and search filters; every post shows its new cover. /blog/<slug> matches the post, callouts and launch states: opening, fact strip, numbered sections, the section rail with progress, code blocks, callouts, author card and "Keep reading". Tables stay inside the reading column without overflow. /blog/feed.xml is unchanged.
entry_points: compozy.com /blog; /blog/categories/<category>; /blog/introducing-compozyos; /blog/feed.xml; draft web/src/drafts/blog
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/site-redesign/qa/R4/ET-site-blog/walk.jsonl; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/index-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/index-1440-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/category-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/search-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/empty-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/post-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/post-1440-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/callouts-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/launch-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/post-tables-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/reduced-launch-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-blog/rewalk-anchor-1440-site.jpg
last_report: docs/qa/reports/2026-11-10-site-redesign-r4.md
overlaps: ET-site-landing
---

E2E-005 of the site redesign spec, stories US-009 and US-010. Exercise each filter, open a post from the index, and follow the section rail to a heading anchor.

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft. Reduced motion must keep every section readable.

QA impact 2026-11-10 (review fixes R2, R4): topic and search live in the URL (`?topic=…&q=…`) and carry over when the category changes. Section-rail and hash links land each heading at the draft's 112px offset.

QA result 2026-11-10 (R4 walk, `next start`, 1440 and 390, dark): pass. Index, category, search and empty match their drafts (height within 66px); posts are within 1% of the draft height (the remaining difference is code blocks a few pixels shorter each). Every visible cover loads (the hover previews are desktop-only); topic `coding-agents` plus search "agent" survive a switch to Engineering; the rail tracks reading progress and jumps to its headings; tables scroll inside the 342px reading column at 390; reduced motion keeps the launch post readable; `/blog/feed.xml` is valid RSS 2.0 with all 8 posts from an unchanged route. Defects fixed during the walk: headings landed 88px lower than the draft (scroll padding added to the draft's scroll margin), and body text used 1.75 line height where the draft renders 1.7.
