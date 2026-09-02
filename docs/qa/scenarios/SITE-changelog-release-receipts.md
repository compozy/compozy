---
id: SITE-changelog-release-receipts
area: SITE
title: Site changelog mirrors complete published GitHub releases
persona: Bruno
journey: J-evaluate-compozy-beta
expected: '`/changelog` lists only published GitHub releases from `v0.3.0-beta.1` onward with the exact version as each title; every version page preserves the complete release notes and real categories, compares from the closest lower semantic version, links referenced pull requests, attributes unique human contributors, exposes release assets, and refreshes within five minutes.'
entry_points: compozy.com `/changelog`, `/changelog/<version>`, `/changelog/feed.xml`, GitHub Releases
qa_status: fail
bug_ids: BUG-20260901-site-search-catalog-files-missing
fix_status: pending
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-09-01-site-changelog-beta22/index-desktop.png; docs/qa/evidence/2026-09-01-site-changelog-beta22/detail-evidence-full.png; docs/qa/evidence/2026-09-01-site-changelog-beta22/search-error.png
last_report: docs/qa/reports/2026-09-01-site-changelog-beta22.md
overlaps:
---

The public changelog reads GitHub Releases directly. GitHub owns the canonical text and publication
state; the site no longer creates or commits a second MDX release receipt.

## Steps

1. Open `/changelog` and confirm every published tag from `v0.3.0-beta.1` onward appears once, newest first, with its exact version as the linked title and the categories from its GitHub release body.
2. Open the latest version page and compare it with the matching GitHub Release. Confirm the complete notes, headings, pull-request links, publish date, channel, and asset count match, then confirm the Compare action starts at the closest lower semantic version rather than a publication-date neighbor.
3. Confirm the pull-request evidence links to merged PRs, contributor identities are unique, and bot identities are omitted.
4. Open the release asset list and confirm its links resolve to the matching GitHub release downloads and source archives.
5. Confirm `/changelog/feed.xml`, site search, `sitemap.xml`, `/llms.txt`, and `/llms-full.txt` expose the current release and its dedicated page URL.
6. Confirm a pre-cutoff tag is absent and a newly published release appears within five minutes without a site-content commit.

QA 2026-08-03: Bruno walked the live-GitHub local production build from the index through
`v0.3.0-beta.3`, refreshed its deep link, opened PR and contributor destinations, expanded all 23
release assets, found the release through site search, opened RSS, and confirmed `v0.2.15` returns
the public not-found page. Desktop and 320 px captures passed after the release Markdown renderer
was corrected to preserve sequential HTML heading levels.
