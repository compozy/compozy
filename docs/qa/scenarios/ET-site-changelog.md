---
id: ET-site-changelog
area: ET
title: /changelog shows every release on one connected timeline and reads one release in full
persona: Bruno
journey: J-evaluate-compozy-beta
expected: /changelog matches the changelog draft states list, betas, search, empty and unavailable. Every release sits on one vertical line with its full version beside its node; the beam fills on scroll; betas branch off and rejoin between the stable releases they sit between; breaking releases are marked; "load older" reveals the earlier releases without losing any. The pulse chart shows every release and opens one on click. /changelog/<version> matches the major, patch and small states and keeps every published section, the "Every merged change" PR links and the neighbours on the line. The RSS feed at /changelog/feed.xml is unchanged.
entry_points: compozy.com /changelog; /changelog/v0.5.5; /changelog/<a beta tag>; /changelog/feed.xml; draft web/src/drafts/changelog
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: SITE-changelog-release-receipts; REL-stable-changelog-hard-cut
---

E2E-004 of the site redesign spec, stories US-007 and US-008. Compare one release against its GitHub Release body: no category, PR link, contributor or asset may be dropped (SITE-changelog-release-receipts owns that receipt).

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft. Reduced motion must keep every section readable.
