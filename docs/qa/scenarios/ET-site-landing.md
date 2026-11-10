---
id: ET-site-landing
area: ET
title: The compozy.com landing matches the approved draft on live numbers
persona: Dora
journey: J-evaluate-compozy-beta
expected: / renders the approved landing draft section for section and in order at 1440 and 390 inside the shared floating header (logo, Docs, Marketplace, Blog, Changelog, Download, star button; compacts on scroll with the progress hairline; links in a menu below md) and the one-line footer. The hero equals the COPY.md Hero Lock. Star count, release count and provider count come from live site data, never the draft constants; an unavailable GitHub count hides its number instead of inventing one. The install tabs offer the desktop download and every terminal method with copy.
entry_points: compozy.com /; draft web/src/drafts/landing (state default)
qa_status: partial
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/site-redesign/qa/R4/ET-site-landing/walk.jsonl; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/outline-1440.json; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/outline-390.json; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/default-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/default-1440-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/default-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/default-390-draft.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/nojs-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/nojs-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/reduced-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/reduced-390-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-landing/menu-open-390-site.jpg
last_report: docs/qa/reports/2026-11-10-site-redesign-r4.md
overlaps: REL-os-landing-proof; ET-site-download
---

E2E-001 of the site redesign spec (`.compozy/tasks/site-redesign/_tests.md`), stories US-001 and US-003.

Walk at 1440×900 and 390×844 on `next start` in the dark theme, comparing each state with its approved draft in `web/src/drafts/` (gallery render URL `/__proto/render/<slug>?chrome=0&state=<state>&theme=dark`). Record console errors (none expected besides the anonymous `api.github.com` 403 and the `/_vercel/*` 404s outside Vercel), horizontal overflow (must be 0) and every deviation from the draft. Reduced motion must keep every section readable.

Also check that the current section is marked with `aria-current="page"` on every marketing route and that each header and footer link navigates to its real route.

QA impact 2026-11-10 (review fixes R1–R4): the hero above the fold is visible in the served HTML (no JavaScript, reduced motion) and animates in once; images below the fold load lazily (4 image requests before the first scroll at 1440 and 390). The releases stat counts every published release (betas included) and the provider count is in the HTML; a count-up starts from zero only for a stat that is still below the fold at hydration. The install section lists three steps: pick a provider (`compozy install`), start the daemon, open a session. Copy buttons show Copied or Copy failed and return to idle.

QA result 2026-11-10 (R4 walk, `next start` on the production build, 1440×900 and 390×844, dark): partial. Held: section order and headings equal the draft outline at both widths; header links, `aria-current="page"` on every marketing route, compaction with the progress hairline, the menu below md (Escape closes it) and the one-line footer; the hero equals the COPY.md Hero Lock; provider count 15 and release count 78 come from live data (the release count is baked into the build HTML), the star count shows a dash when the anonymous GitHub call is refused; every install tab copies exactly the command shown; no overflow and no console error besides the documented noise. Two draft deviations are deliberate: the Supervise panels keep their height below `lg` (the draft collapses them to empty rows through `basis-0` in a column), and the install section has the three steps above. Not verified: the comparison table still shows "to verify" cells and a literal `<date>` that wait on the owner's decision; the landing cannot be marked passed until that content lands. Defects fixed during the walk: release count baked as a dash by local Turbo builds (token not passed through), face chips 5px taller than the draft. See the dated report.
