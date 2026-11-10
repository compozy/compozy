---
id: ET-site-changelog
area: ET
title: /changelog shows every release on one connected timeline and reads one release in full
persona: Bruno
journey: J-evaluate-compozy-beta
expected: /changelog matches the changelog draft states list, betas, search, empty and unavailable. Every release sits on one vertical line with its full version beside its node; the beam fills on scroll; betas branch off and rejoin between the stable releases they sit between; breaking releases are marked; "load older" reveals the earlier releases without losing any. The pulse chart shows every release and opens one on click. /changelog/<version> matches the major, patch and small states and keeps every published section, the "Every merged change" PR links and the neighbours on the line. The RSS feed at /changelog/feed.xml is unchanged.
entry_points: compozy.com /changelog; /changelog/v0.5.5; /changelog/<a beta tag>; /changelog/feed.xml; draft web/src/drafts/changelog
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/walk.jsonl; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/list-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/list-1440-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/betas-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/search-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/empty-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/major-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/patch-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/small-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/release-expanded-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/unavailable-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/unavailable-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-changelog/release-error-390-site.jpg
last_report: docs/qa/reports/2026-11-10-site-redesign-r4.md
overlaps: SITE-changelog-release-receipts; REL-stable-changelog-hard-cut
---

E2E-004 of the site redesign spec, stories US-007 and US-008. Compare one release against its GitHub Release body: no category, PR link, contributor or asset may be dropped (SITE-changelog-release-receipts owns that receipt).

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft. Reduced motion must keep every section readable.

QA impact 2026-11-10 (review fixes R2, R4): betas, search and loaded pages live in the URL (`?betas=1&q=…&page=2`) and survive opening a release and going back. Release pages do not skip heading levels. With GitHub unavailable, a release page shows the error page that says the notes could not be loaded, links GitHub Releases and offers Try again. Each pulse-chart bar opens the release drawn under the pointer, including releases a few hours apart.

QA result 2026-11-10 (R4 walk, `next start`, 1440 and 390, dark): pass. List, betas, empty and the three release states match their drafts (height within 85px; the small release adds "Release files" and "Compare with", which keep the published assets); search lists 9 releases where the draft snapshot also lists pre-cutoff v0.2.x releases the site does not publish. The beam fills on scroll; two breaking releases are marked; "Older releases" goes from 12 to 15 entries; v0.5.5 matches its GitHub Release (26 PR links, 30 assets) and v0.5.0 keeps all 23 PR references among 46 changes; `/changelog/feed.xml` is valid RSS 2.0 with 49 items from an unchanged route. GitHub-down states were walked on a throwaway copy of the build with `api.github.com` failing: the list shows its unavailable state; a release page answers 500 with the error page above (the console logs React's production server-render error, which is the intended throw). Defect fixed during the walk: neighbouring pulse bars overlapped, so at 390 a click on v0.5.5 opened v0.5.4; each bar now opens itself at both widths.
