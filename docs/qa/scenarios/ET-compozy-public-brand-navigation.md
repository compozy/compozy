---
id: ET-compozy-public-brand-navigation
area: ET
title: Navigate the public CompozyOS brand and launch post
persona: Ada
journey: J-validate-compozy-hard-cut
expected: The public site, launch post, metadata, OpenGraph assets, sitemap, robots, RSS, llms output, and authored runtime guidance identify CompozyOS at https://compozy.com; command identifiers remain compozy and no public product copy uses the retired name.
entry_points: local packages/site root and metadata outputs; local /blog/introducing-compozyos; canonical https://compozy.com declarations
qa_status: untested
bug_ids: BUG-20260727-runtime-legacy-identity
fix_status: fixed
retest_status:
fix_commits: e4df8634
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/browser/compozy-home.png; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/browser/compozy-dev-home.png; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-e2e-web-final-2.log;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-053; APP-brand-channel-visibility
---

QA impact 2026-07-26: the public brand origin and launch-post topology changed. Planning flag
only; the next QA cycle owns browser and metadata retesting.

QA impact 2026-07-27: Task 11 replaced the launch-post narrative and landing/OG thesis without
changing the sole permanent same-domain slug redirect. The scenario remains `untested`; the next QA
cycle owns route, metadata, RSS, search, and social-card evidence.

QA impact 2026-07-27: the final identity hard cut removed the retired launch-post compatibility
redirect. The next QA cycle owns route and metadata verification.

QA impact 2026-07-29: the launch URL now names CompozyOS directly, and the homepage, blog metadata,
search, RSS, OpenGraph, and internal links must expose `/blog/introducing-compozyos` with no old
Network-first route. The scenario remains `untested`.

QA impact 2026-08-10: the product-language hard cut now covers site, Web display strings, CLI help,
release copy, SDK metadata, and generated references. Reset to `untested`; Task 07 owns the walk.

## 2026-09-11 article publication slice

Eight reviewed articles add blog routes without changing the existing brand or launch article.
The [publication report](../reports/2026-09-11-blog-publication.md) maps every source draft and
records factual review. This slice does not reclassify the historical full scenario.

Acceptance for this slice:

1. All eight routes open from the blog index and render one page H1, section anchors, author/date,
   readable tables, highlighted code, and related-reading links.
2. Desktop and mobile widths keep content usable without document-wide horizontal overflow;
   code/table overflow remains locally navigable when necessary.
3. Each route emits its own canonical, description, Article JSON-LD, and social image. RSS and
   sitemap include every new permalink.
4. Existing blog metadata, navigation, code-block, and structured-data suites pass against the
   generated content, followed by a production build and rendered checks.

Slice validation: passed against the local production build on September 11, 2026. See the
publication report for gate, metadata, browser, responsive, and command-shape evidence. The
historical full scenario and real provider execution are outside this editorial slice.

## 2026-09-11 editorial rework slice

The eight new articles were consolidated into six distinct reader tasks. The two older articles
remain in the archive. The [rework report](../reports/2026-09-11-blog-rework.md) owns editorial
research, article disposition, generated cover provenance, and current validation.

Acceptance for this replacement slice:

1. The blog index lists eight total articles and features the executable retry experiment.
   All six revised articles show their own generated abstract cover, title, and reading time.
2. `/blog/what-is-an-os-for-ai-agents/` permanently redirects to the session-recovery article;
   `/blog/orca-vs-openhands/` permanently redirects to the agent-evaluation article.
   The former `crewai-alternatives`, `langchain-alternatives-production-ai-agents`, and
   `langgraph-alternatives` URLs redirect directly to `git-repository-briefing-python`,
   `agent-framework-architecture`, and `ai-agent-retries-idempotency`, respectively.
3. Both desktop and 390px mobile layouts render loaded covers, one H1, useful section links,
   readable code/tables, and no document-wide overflow. Decorative covers have empty alt text.
4. All reader downloads are served and match the examples described by their articles. The
   retry lab and repository briefing collector execute with the documented local prerequisites.
5. Canonical metadata, Article JSON-LD, OpenGraph/Twitter covers, RSS, sitemap, and related reading
   use the six current article routes; retired routes do not remain in discovery surfaces.
   Article identity matches the canonical URL, author identity links to the visible author
   profile, and sitemap modification dates come from editorial metadata. Empty categories
   remain navigable but use `noindex, follow` and are absent from the sitemap.

Validation status is recorded in the rework report and the [SEO follow-up](../reports/2026-09-11-blog-seo.md).
This slice does not reclassify historical
brand journeys or claim real-provider execution or a comparative coding-agent benchmark.
