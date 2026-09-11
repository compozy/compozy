# Blog editorial rework

The subsequent [SEO audit](2026-09-11-blog-seo.md) updates six titles/descriptions, moves three
misleading comparison URLs with permanent redirects, and records the latest verification.
The route names and validation below describe the editorial checkpoint before that follow-up.

## Outcome and scope

Reworked the eight articles introduced by `9a792f51a720` into six distinct reader tasks.
Two redundant articles were consolidated with permanent redirects. The two earlier posts remain.
This is a local revision for review; no commit, push, deployment, or external publication was performed.

The initial problem was substantive, not just stylistic: repeated definitions and setup snippets,
comparison titles without comparative evidence, unfinished exercises, and no clear reader payoff.
The replacement set delivers a runnable experiment, a Git evidence collector, an evaluation kit,
an architecture worksheet, a review brief, and a recovery/handoff guide.

| Retained URL suffix | Revised article | Concrete reader outcome |
| --- | --- | --- |
| `autonomous-coding-agent-setup` | Give a Coding Agent a Task You Can Actually Review | Full task brief, behavior-first review exercise, acceptance record |
| `defining-agent-sessions-compozyos` | What Survives When an Agent Session Stops? | Diagnosis tree, supported recovery paths, continuation handoff |
| `cursor-vs-claude-code` | How to compare coding agents on your own repository | Shared baseline procedure, downloadable brief and scorecard |
| `crewai-alternatives` | Build a repository briefing worth reading | Executable Git collector, captured JSON, source-based review queue |
| `langchain-alternatives-production-ai-agents` | Before you replace your agent framework, find the failing boundary | Worked architecture decision, failure drill, downloadable worksheet |
| `langgraph-alternatives` | Your Agent Retried. Did It Publish Twice? | Runnable crash/retry lab, observed duplicate, receiver-side correction |

`/blog/what-is-an-os-for-ai-agents/` redirects to the session-recovery article. Its useful
ownership explanation is incorporated there. `/blog/orca-vs-openhands/` redirects to the
agent-evaluation article, which incorporates worktree versus runtime isolation. Retained slugs
preserve existing links even where keyword-led titles were replaced.

The revised source files contain roughly 1,800–2,000 words each including examples and metadata;
the original eight were roughly 1,180–1,370 by the same whitespace method. Added length buys
worked examples, explanations, outputs, and recovery—not more introductions. Word count is not
an acceptance criterion.

## Research and editorial decisions

Applied `exa-search`, `writing-tech-post`, and `imagegen`. Two research subagents read 22 raw
articles in `/Users/pedronauck/Dev/courses/pedronauck/research/tech-blog-posts`, including practical
AI workflows, reliability investigations, migrations, and tutorials. These are historical
captures from May 2026, used for writing analysis, not current product or popularity claims.
The second brain was not changed.

Exa discovery and extraction examined 12 selected DEV and Medium articles. DEV's public API
provided observable engagement. Examples included [Event Loop](https://dev.to/lydiahallie/javascript-visualized-event-loop-3dif)
(4,447 reactions, 4-minute platform estimate), [Promises](https://dev.to/lydiahallie/javascript-visualized-promises-async-await-5gke)
(4,079 reactions), and [Git concepts](https://dev.to/unseenwizzard/learn-git-concepts-not-commands-4gjc)
(3,809 reactions, 36-minute estimate), observed September 11. Length varies with the work delivered.

Medium did not expose reliable readership or clap totals to this retrieval. Exa text showed
750 responses on [the 2016 JavaScript dialogue](https://medium.com/hackernoon/how-it-feels-to-learn-javascript-in-2016-d3a717dd577f)
and 193 on [Software 2.0](https://karpathy.medium.com/software-2-0-a64152b37c35), with extraction/cache
limits recorded. Responses are not reads; claps are not unique readers. This is a selected
editorial sample, not a global most-read ranking.

Useful corpus references included [Michael Lynch's tutorial guidance](https://refactoringenglish.com/chapters/rules-for-software-tutorials/),
[Datadog's Postgres investigation](https://www.datadoghq.com/blog/engineering/debugging-postgres-performance/),
[Slack's context-management examples](https://slack.engineering/managing-context-in-long-run-agentic-applications/),
and [Cloudflare's ClickHouse investigation](https://blog.cloudflare.com/clickhouse-query-plan-contention/).
The recurring useful pattern was a concrete problem, examinable evidence, a mechanism, and a
consequence the reader can act on. Company prestige and engagement were not treated as proof
that every passage was accurate or worth imitating.

Research artifacts under `.codex/research/blog-rework-2026-09-11/` retain Exa requests/results,
public metric records, `popularity-editorial-report.md`, `corpus-editorial-report.md`, and each
writing agent's evidence note. Raw third-party extracts are local research material, not public
blog content. Article prose and reader artifacts are original.

## Covers and presentation

Six abstract covers were generated with the built-in `image_gen` tool, one call per asset.
Art direction: tactile graphite/ivory sculptures, warm dark background, restrained Compozy orange,
distinct visual ideas for inspection, continuity, comparison, signal extraction, retries, and
architecture. No text, logos, or synthetic product screenshots.

Final sources are in `packages/site/content/blog/covers/`: `reviewable-agent-work.webp`,
`session-continuity.webp`, `coding-agent-evaluation.webp`, `repository-briefing.webp`,
`agent-retries.webp`, and `agent-architecture.webp`. Each is 1672×941; all six total approximately
649 KB. Encoding changes file format only; the generated composition is preserved. Original PNGs
remain in Codex's generated-image directory. Exact prompts, source paths, and destination mapping
are recorded in `.codex/research/blog-rework-2026-09-11/cover-prompts.json`.

Velite owns hashed public cover copies. Existing `blogPostCover` and `next/image` now display
them on cards and article pages; the retry experiment is featured. Generated abstract art uses
empty alt text in the reading UI, while metadata retains the article-specific image identity.
No dependencies, tokens, or new generic UI primitives were introduced.

## Evidence and impact

- The retry lab actually terminated child processes after receiver commit and before local
  receipt. A naive retry produced two comments; receiver-enforced idempotency produced one.
  Changed content under the same operation key was rejected. Python 3.14.7; no network/model calls.
- The briefing collector ran against real disposable Git repositories. It checked explicit and
  divergent baselines, pinned endpoints, timestamps, bounds/omissions, unusual filenames,
  excluded tracked credential names, untracked/dirty content exclusion, and repeatable output.
  Captured JSON is shipped with the article. The worktree setup was also executed in a fixture.
- Runtime instructions were checked against current Cobra sources, session/permission code, and
  maintained documentation. Examples are distinguished from captured runtime execution.
- Review removed an unsupported commit-to-file attribution from the briefing specimen: the
  collector's aggregate outside-window count does not associate an individual path with a commit.
- Existing heading and metadata tests were updated for the intentionally changed editorial copy.
  The manual API reference scanner was corrected to consume external URLs whole, preserving
  detection of invalid local routes; a same-suite case covers the distinction. No article prose
  snapshot tests were added.

Cross-surface impact, using `docs/_memory/change-impact.md`: native tools, hooks/extensibility,
runtime config, workspace data isolation, and `skills/compozy/` are **not applicable — editorial
only**. Web runtime is unchanged. Site impact is cover presentation, index copy/metadata,
article discovery, two permanent URL redirects, and reader downloads. The owning scenario is
[ET-compozy-public-brand-navigation](../scenarios/ET-compozy-public-brand-navigation.md).

## Validation

- Changed content, UI diff, test adjustments, and downloadable examples were reviewed. The corpus
  reader performed a final editorial acceptance check; no unresolved editorial blockers remain.
- `make gate`: passed the affected `js-packages-site` lane with zero lint warnings/errors,
  successful typechecking, and **56 test files / 330 tests**. `make gate-status` reported
  `CURRENT-PASS`. Log: `.cache/blog-rework-gate.log`.
- `bunx turbo run build --filter=./packages/site`: passed after the final content/test corrections.
  Log: `.cache/blog-rework-final-build.log`.
- `npx react-doctor@latest --verbose --diff`: **100/100**, no issues across the changed React
  files. Log: `.cache/blog-rework-react-doctor.log`.
- Real HTTP checks against the final production build: all eight current articles return 200,
  with one correct H1, canonical, Article JSON-LD, valid section anchors, and available cover
  images. The six new covers are used by OpenGraph and Twitter metadata. Both retired routes
  return 308 to their replacements; requests without trailing slashes also reach the replacement.
  RSS and sitemap contain all eight current articles and neither retired route. All six downloads
  return 200 with byte-identical content. Evidence: `.cache/blog-rework-http.json`.
- Browser checks at 390px: all six revised routes have one H1, a decoded cover with empty alt,
  and document width equal to viewport width. Evidence: `.cache/blog-rework-mobile.json`.
  Desktop review at 1440px confirmed index hierarchy, distinct loaded cover cards, and the
  featured retry article. Copy-code feedback displayed `Copied`; no browser page errors were
  reported. Screenshots: `.cache/blog-rework-index-desktop.png`,
  `.cache/blog-rework-cards-desktop.png`, and `.cache/blog-rework-article-mobile.png`.
- Article-local docs/blog/download links resolve to current source files. The parent reran the
  retry lab and reproduced the published output. `git diff --check` passed.

The browser used for QA is closed after capture. A loopback-only site preview is intentionally
available at `http://127.0.0.1:43191/blog/` for the user's review; this is not a deployment.

Limits: no live coding-agent comparison, provider authentication/model execution, production
session interruption, or runtime scheduling was performed. The posts do not claim those results.
No repository-wide CI or deployment is implied by local content and site validation.
