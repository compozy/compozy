---
name: 13-docs-site
title: Documentation Site (packages/site, agh.network) — Real-LLM QA Plan
description: Behavior-first QA scenarios for the Fumadocs site at packages/site — Next.js 16 export build at agh.network, Fumadocs MDX runtime/protocol trees, Velite-powered blog/changelog, generated OpenAPI + CLI reference, search index, sitemap/robots/RSS, OpenGraph, theme + design-token contract, and the bun-side gates that ship the site through `make verify`. Closes the docs-site loop end-to-end so any change touching content/, app/, components/, lib/, or codegen targets is validated against the canonical DESIGN.md, COPY.md, and runtime-truth invariants.
type: final-qa-child
module: docs-site
parent: ../_parent.md
provider_lanes: [claude-code]
authoritative_runtime_truth:
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/packages/site/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/packages/site/AGENTS.md
  - /Users/pedronauck/Dev/compozy/agh/DESIGN.md
  - /Users/pedronauck/Dev/compozy/agh/COPY.md
  - /Users/pedronauck/Dev/compozy/agh/docs/_memory/glossary.md
references:
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/04-autonomy-kernel.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/11-api-cli-parity.md
---

# 13 — Documentation Site (packages/site, agh.network)

Sibling of `11-api-cli-parity.md` (which proves codegen drift in `openapi/agh.json` and `web/src/generated/agh-openapi.d.ts`) and of every backend child whose contract change must co-ship docs (per the root CLAUDE.md "No partial-surface completions" directive and the `cy-web-docs-impact` skill). This module proves **the docs site itself** — that the Fumadocs MDX runtime/protocol trees render with no broken anchors; that the API reference is a faithful projection of `openapi/agh.json`; that the CLI reference under `content/runtime/cli-reference/` is a faithful projection of the cobra JSON export via `make cli-docs`; that the Velite blog + changelog body content is real (no aspirational copy); that DESIGN.md tokens and COPY.md vocabulary are honored; that the search index, sitemap, robots.txt, and RSS feed shape match the runtime-truth contracts in `packages/site/lib/`; that the bun gate (`make bun-lint`, `make bun-typecheck`, `make bun-test`) is green; and that `make site-build` produces a deterministic static export ready for `agh.network`.

The CLAUDE.md and packages/site/CLAUDE.md invariants this child encodes:

- "**Pull tokens from `DESIGN.md` (repo root).** No invented colors, type, radii, spacing, or motion." (`packages/site/CLAUDE.md:7`)
- "**Pull product language from `COPY.md` (repo root).** Landing copy, blog/changelog, runtime/protocol narrative docs, site config, OpenGraph metadata, SEO descriptions, and public CTAs MUST follow the copy system before inventing new wording." (`packages/site/CLAUDE.md:8`)
- "**Hero positioning is locked**: headline 'An open workplace for AI agents.' with subhead … Open-workplace-first." (`packages/site/CLAUDE.md:9`; mirrors COPY.md §3 and the auto-memory entry "AGH Site & Documentation Project — hero relocked 2026-05-01").
- "**`packages/site` ships in same PR as backend contract changes** that affect documented APIs/CLI verbs (per `internal/api/contract` co-ship rule in root CLAUDE.md)." (`packages/site/CLAUDE.md:10`)
- "Document only behavior the runtime actually supports today. … API/CLI references are generated from `openapi/agh.json` and the cobra JSON export — do not paraphrase. If the generated reference is wrong, fix the source." (`packages/site/CLAUDE.md:54-55`)
- "Vocabulary follows `docs/_memory/glossary.md`. The canonical artifact name is `capability`, never `recipe`." (`packages/site/CLAUDE.md:56`)
- "The canonical artifact name is `capability`, never `recipe`, `workflow`, `procedure`, or `playbook` for current AGH behavior." (root `CLAUDE.md` Vocabulary & Product Strategy)

Every scenario below is written for the **real-claude-code** lane only where it must be (DOC-19 — "Try it" embed harness). Most scenarios are control-plane invariants that do not need a live LLM, so they run under **mock-acp** or as headless static-build assertions.

## 1. Module surface — site map and source-of-truth contracts

The site is a single Bun-managed Next.js 16 workspace under `packages/site/` with `output: "export"` (`packages/site/next.config.mjs:8`) so every route resolves at build time and `out/` is a static deployable. The content layer is two-headed:

1. **Fumadocs MDX** — `runtime` + `protocol` trees defined in `packages/site/source.config.ts:3-9` and loaded by `packages/site/lib/source.ts:65-75` (`runtimeDocs` at `/runtime/*`, `protocolDocs` at `/protocol/*`).
2. **Velite** — blog posts, blog authors, and changelog releases defined in `packages/site/velite.config.ts:25-79`, exposed to the app via the `#site/content` alias (`packages/site/vitest.config.ts:13`).

The static `app/` routes (`packages/site/app/`):

| Route                          | Source                                                             | Notes                                                                                                                                                       |
| ------------------------------ | ------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `/`                            | `packages/site/app/(home)/page.tsx:16-33`                          | Landing — mounts 12 sections starting with `<Hero />`. Hero copy locked in `packages/site/components/landing/hero.tsx:43-51`.                              |
| `/runtime/[[...slug]]`         | `packages/site/app/runtime/[[...slug]]/page.tsx:13-39`             | Catch-all docs renderer; resolves via `runtimeDocs.getPage(slug)`.                                                                                          |
| `/protocol/[[...slug]]`        | `packages/site/app/protocol/[[...slug]]/page.tsx:13-39`            | Catch-all docs renderer; resolves via `protocolDocs.getPage(slug)`.                                                                                         |
| `/blog`                        | `packages/site/app/blog/page.tsx:28-114`                           | Blog index; pulls `allPosts()` (`packages/site/lib/blog.ts:24-26`).                                                                                          |
| `/blog/[slug]`                 | `packages/site/app/blog/[slug]/page.tsx:52-117`                    | Single post; `generateStaticParams` enumerates every post.                                                                                                  |
| `/blog/categories/[category]`  | `packages/site/app/blog/categories/`                               | One page per `BLOG_CATEGORIES` (`packages/site/lib/blog.ts:3`).                                                                                              |
| `/blog/feed.xml`               | `packages/site/app/blog/feed.xml/route.ts:15-51`                   | Static RSS, `application/rss+xml; charset=utf-8`.                                                                                                            |
| `/changelog`                   | `packages/site/app/changelog/page.tsx:16-101`                      | Velite `releases` collection.                                                                                                                                |
| `/api/search`                  | `packages/site/app/api/search/route.ts:6-27`                       | Static GET; Fumadocs advanced index over `runtimeDocs` + `protocolDocs`.                                                                                     |
| `/sitemap.xml`                 | `packages/site/app/sitemap.ts:8-25`                                | Combines runtime + protocol pages, blog posts, blog category pages, plus `/`, `/blog`, `/changelog`.                                                         |
| `/robots.txt`                  | `packages/site/app/robots.ts:6-14`                                 | Allows `/`; emits canonical sitemap URL.                                                                                                                     |
| `/opengraph-image`             | `packages/site/app/opengraph-image.tsx:12-78`                      | 1200×630 PNG; embeds `siteConfig.description` and the locked headline.                                                                                       |
| Global metadata                | `packages/site/app/layout.tsx:27-72`                               | Title template `%s \| AGH`, theme color `#E8572A`, manifest `/site.webmanifest`, icons, OpenGraph, Twitter, robots index/follow.                              |

Content trees on disk:

- **Runtime docs** — `packages/site/content/runtime/`:
  - `index.mdx`, `how-to-use-these-docs.mdx`, `meta.json`
  - `core/` — 18 subfolders (sessions, agents, network, autonomy, memory, tools, skills, resources, automation, bridges, sandbox, workspaces, operations, configuration, extensions, hooks, getting-started)
  - `guides/` — `choose-an-operator-surface.mdx`, `coordinate-agents-over-network.mdx`, `debug-a-failed-session.mdx`, `index.mdx`, `meta.json`
  - `use-cases/` — `handoff-between-agents.mdx`, `index.mdx`, `prepare-a-project-workspace.mdx`, `release-readiness-sweep.mdx`, `review-a-change.mdx`, `meta.json`
  - `cli-reference/` — generated from `make cli-docs` via `go run ./cmd/agh doc --output-dir packages/site/content/runtime/cli-reference` (`Makefile:77-78`); contains 27 verb folders + `index.mdx`, `meta.json`, plus single-file leaves `agh.mdx`, `install.mdx`, `update.mdx`, `uninstall.mdx`, `version.mdx`, `whoami.mdx`, `spawn.mdx`.
  - `api-reference/` — generated from `openapi/agh.json` via `bun run scripts/generate-openapi.ts` (`packages/site/scripts/generate-openapi.ts:96-118`); preserves `index.mdx`, generates one `<tag>.mdx` per OpenAPI tag (currently 21 files: agent, agents, automation, bridges, bundles, daemon, extensions, hooks, memory, network, observe, resources, sessions, settings, skills, tasks, tools, toolsets, vault, workspaces — plus `index.mdx`, `meta.json`).
- **Protocol docs** — `packages/site/content/protocol/`: 13 top-level MDX (`overview.mdx`, `envelope.mdx`, `message-kinds.mdx`, `interactions.mdx`, `delivery.mdx`, `verification.mdx`, `peer-discovery.mdx`, `capability-discovery.mdx`, `ed25519-jcs.mdx`, `examples.mdx`, `conformance.mdx`, `implementation-status.mdx`, `index.mdx`) plus `guide/` and `meta.json`.
- **Blog** — `packages/site/content/blog/`: `posts/` (currently `introducing-agh-the-first-agent-network-protocol.mdx`), `authors/` (`pedronauck.yml`), `changelog/` (currently empty — the Velite `releases` collection is the canonical changelog source).

Generated artifacts that **must be in lockstep** with their generators (any drift here is a doc-site QA failure):

| Artifact (under `packages/site/`)               | Generator                                                                        | Source of truth                                                  |
| ----------------------------------------------- | -------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| `.source/`                                      | `bun run source:generate` (`fumadocs-mdx source.config.ts .source`)              | `content/runtime/`, `content/protocol/`                          |
| `.velite/`                                      | `bun run content:generate` (`velite build`)                                      | `content/blog/posts/`, `content/blog/authors/`, `content/blog/changelog/` |
| `content/runtime/cli-reference/**`              | `make cli-docs` (`go run ./cmd/agh doc --output-dir …` — `Makefile:77-78`)        | cobra command tree under `internal/cli/`                         |
| `content/runtime/api-reference/**`              | `bun run generate:openapi` (`packages/site/scripts/generate-openapi.ts:96-118`)  | `openapi/agh.json` (`packages/site/lib/openapi.ts:8`)            |
| `out/` (static export)                          | `make site-build` (`Makefile:74-75`) → `cd packages/site && bun run build`       | All the above                                                    |

The fonts allowed are precisely four:

- **Inter** — `packages/site/app/layout.tsx:8-12` (sans, body, UI, docs headings)
- **Playfair Display** — `packages/site/app/layout.tsx:14-19` (marketing display, `.site-home h1/h2` only — `packages/site/app/global.css:310-313`)
- **JetBrains Mono** — `packages/site/app/layout.tsx:21-25` (mono labels, badges, code)
- **NuixyberNext** — `packages/site/app/global.css:14-20` (`@font-face` for `/fonts/NuixyberNext-Regular.ttf`); used **only** via `<Logo variant="logo" />` from `@agh/ui` for the wordmark.

Per `DESIGN.md` §3 "Font Families" any other font on any page is a doc-site failure.

## 2. Existing coverage — do NOT duplicate

`packages/site/` already ships ~50 vitest specs in `packages/site/lib/*.test.ts` and component-adjacent tests in `packages/site/components/`. The set this QA child must NOT replicate:

- `packages/site/lib/internal-links.test.ts:171-220` — content link rot for every internal route + every hash anchor; flags ambiguous heading IDs; rejects raw `*.mdx` links.
- `packages/site/lib/runtime-api-reference.test.ts:36-64` — every OpenAPI tag has its `<tag>.mdx`; every used tag is partitioned into exactly one `API_SECTIONS` group.
- `packages/site/lib/runtime-manual-cli-examples.test.ts:151-204` — every manual `agh ...` shell example uses a command in the generated CLI reference; flags stale `agh session get`, `agh network peers --channel`, broken `agh spawn` flags; enforces `--ttl-seconds`, `--session`, `--channel`, `--kind`, `--body` shapes.
- `packages/site/lib/runtime-docs-truth.test.ts:60-119` — runtime-truth contracts: MCP resource kind aligns with `internal/config/mcp_resource.go`; resource error mapping aligns with `internal/api/core/errors.go`; SSE examples never go through `/events`; API reference declares `built from openapi/agh.json` and references `make codegen-check`; tool-invoke examples reference real `agh__*` tool IDs.
- `packages/site/lib/site-design-token-contract.test.ts:48-62` — every hex color across `app/`, `components/`, `content/`, `lib/`, plus `public/favicon.svg` and `public/site.webmanifest`, must exist in the canonical `packages/ui/src/tokens.css` palette.
- `packages/site/lib/site-copy-contract.test.ts:32-45` — bans first-person plural ("we", "our") in `app/changelog/`, `components/landing/`, `content/blog/posts/`.
- `packages/site/lib/landing-truth.test.tsx:67-103` — landing `PROVIDERS` aligned with `internal/config/provider.go` `builtinProviders`; landing source citations point at existing runtime routes; banned premature claims (`signed`, `verified identity`, `Ed25519`).
- `packages/site/lib/public-route-metadata.test.ts:48-138` — sitemap is canonical HTTPS, deduped, points at runtime + protocol + blog + categories; robots is canonical; RSS feed is parseable, channel + items shaped from generated posts.
- `packages/site/lib/public-search-index.test.ts:71-110` — `app/api/search/route.ts` invokes `createSearchAPI("advanced", …)` once with both `runtimeDocs` and `protocolDocs`, `id === url`, every page indexed.
- `packages/site/lib/public-copy-quality.test.ts`, `lib/public-secret-safety.test.ts`, `lib/public-link-safety.test.ts`, `lib/public-icon-accessibility.test.ts`, `lib/public-landmark-accessibility.test.ts`, `lib/public-heading-hierarchy.test.tsx`, `lib/public-visual-accessibility.test.ts`, `lib/public-internal-links.test.ts`, `lib/public-aside-accessibility.test.ts`, `lib/public-button-safety.test.ts`, `lib/public-search-index.test.ts`, `lib/public-error-handling.test.ts`, `lib/public-install-contract.test.ts`, `lib/public-media-quality.test.ts`, `lib/public-motion-safety.test.ts`, `lib/public-security-headers.test.ts`, `lib/public-route-metadata.test.ts`, `lib/public-assets.test.ts` — public-page hygiene gates.
- `packages/site/lib/content-*.test.ts` (heading, table, code-block, link-text, frontmatter, diagram, media, meta-navigation, related-navigation, outcome-doc, release-readiness, external-links, test-utils) — MDX content-quality gates (>= 13 specs).
- `packages/site/lib/runtime-*` extras: `runtime-authored-context-docs.test.ts`, `runtime-autonomy-docs.test.ts`, `runtime-docs-discovery.test.ts`, `runtime-hub-quality.test.ts`, `runtime-manual-api-routes.test.ts`, `runtime-tools-canonical-docs.test.ts` — runtime-tree truth gates.
- `packages/site/lib/blog-*` and `lib/landing-cli-snippets.test.ts`, `lib/section-layouts.test.tsx`, `lib/static-route-metadata.test.ts`, `lib/site-config.test.ts`, `lib/site-navigation.test.ts`, `lib/footer-config.ts`-paired tests — page composition + navigation gates.
- `packages/site/components/**/*.test.tsx` — component-level RTL specs (header, footer, blog primitives, mermaid, doc-page-masthead, etc.).
- `packages/site/lib/opengraph-image.test.tsx` — OG image renders the locked headline + description.

The gap real-scenario lane must close: every existing vitest spec stubs the runtime via `vi.mock("@/lib/source", …)` or reads files off disk. **None build the static `out/` artifact and crawl it; none diff a fresh `out/` against an immediately-following second build to prove byte determinism; none run `make site-build` against a contract that just regenerated; none probe the deployed-style preview from a headless browser at 390×844 to assert real responsive geometry; none drive a real-LLM "Try it" against a daemon.**

## 3. Gaps the real-scenario lane must close

1. **Hero copy is the canonical 2026-05-01 relock**: assert hero `<h1>` is exactly "An open workplace for AI agents." and the subhead matches `packages/site/CLAUDE.md:9` byte-for-byte (DOC-01).
2. **Sidebar resolution is exhaustive**: every `runtimeDocs.getPages()` and `protocolDocs.getPages()` URL renders a 200 in the static export and every internal link from those pages resolves (DOC-02).
3. **Search returns results for "session", "memory", "extension"**: assert `/api/search` answers each query with non-zero hits and at least one `tag: "Runtime"` and one `tag: "AGH Network"` result (DOC-03).
4. **API reference rendering is faithful to `openapi/agh.json`**: every `<tag>.mdx` exists, renders an APIPage block, and the page enumerates every operation in that tag (DOC-04).
5. **CLI reference rendering is faithful to the cobra export**: `make cli-docs` is idempotent, every cobra leaf has a generated MDX, and any agent-manageable verb listed in `internal/cli/root.go:65-123` is reachable from `/runtime/cli-reference/<verb>` (DOC-05).
6. **MDX live blocks compile**: `Mermaid`, `GuideCard`, `GuideGrid`, `OperatorNote`, `RouteList`, `RouteRow`, `Workflow`, `WorkflowStep`, and `APIPage` all render in static export (DOC-06).
7. **Theme contract**: every page passes the dark-only contract; `<html class="dark">` is hardcoded in the export; no shadows, no off-palette hex colors, fonts limited to Inter + JetBrains Mono + Playfair Display (`.site-home` only) + NuixyberNext (wordmark only) (DOC-07).
8. **Mobile viewport 390×844**: no horizontal overflow, no truncated nav (DOC-08).
9. **Accessibility headless audit**: keyboard nav, aria-labels, landmarks, color contrast (DOC-09).
10. **External links live**: every external href on the static export is non-rotting (HTTP 200/301/302) or marked `archive` (DOC-10).
11. **Image alt-text**: every `<img>`/`<Image>` in the export has a non-empty alt or empty alt for decorative images (DOC-11).
12. **Build determinism**: `make site-build` twice produces byte-identical `out/` modulo timestamp footers (DOC-12).
13. **Bun gate**: `make bun-lint`, `make bun-typecheck`, `make bun-test` all pass on a clean checkout (DOC-13).
14. **COPY.md adherence (vocabulary)**: scrape `content/`, `components/landing/`, `app/blog/`, `app/changelog/`, and the rendered `out/` for `recipe`, `workflow`, `procedure`, `playbook` referring to current AGH artifacts (DOC-14).
15. **DESIGN.md adherence (CSS)**: scrape generated CSS in `out/_next/static/css/*.css` for `box-shadow`, `drop-shadow`, off-palette hex, and disallowed font families (DOC-15).
16. **SEO + OpenGraph**: every page in `out/` has stable `<title>`, `<meta name="description">`, `og:image`, and there is no duplicate meta within a page (DOC-16).
17. **Sitemap + robots**: `/sitemap.xml` lists every runtime/protocol page, every blog post, every category, every static route; `/robots.txt` is sane (DOC-17).
18. **`@agh/ui` consumption**: every shared primitive imported from `@agh/ui` in the site is documented (or has a story under `packages/ui`) (DOC-18).
19. **Real-LLM "Try it" embed (or harness)**: prove that today the site does **not** run live prompts client-side; document the test-mode harness that proves the embed surface (or its absence) is honest (DOC-19).
20. **Codegen co-ship doc**: regenerating `openapi/agh.json` triggers `bun run generate:openapi` and the API reference tree updates lockstep; no orphan `<tag>.mdx` (DOC-20).
21. **CLI codegen co-ship doc**: `make cli-docs` regenerates the cobra MDX tree; flag/verb removal is reflected in the site without orphan files (DOC-21).
22. **`out/` deployable**: serve `out/` with a static server and assert every route returns 200 with `Content-Type: text/html` and `text/css`, `image/*`, etc. as expected; `_headers` (Cloudflare-style) honored (DOC-22).

## 4. Operating model — provider matrix and bootstrap

| Mode               | When                                                                                              | Driver                                                                                                                                                  |
| ------------------ | ------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `mock-acp`         | **Default for this child.** Every static-build, codegen-drift, link-crawl, and CSS-scrape scenario | No daemon needed. Operates on `packages/site/out/` after `make site-build` and on `openapi/agh.json` + cobra export.                                    |
| `real-claude-code` | **Only DOC-19** when proving (or refuting) a live-prompt embed surface                            | Real `claude-opus-4-8` ACP subprocess against an isolated daemon; the site does NOT today host an interactive embed, so the scenario asserts that. |

Bootstrap and isolation discipline (mandatory):

- One isolated `AGH_HOME`, daemon HTTP port, UDS socket path, `tmux-bridge` socket, and `PROVIDER_HOME`/`PROVIDER_CODEX_HOME` per scenario that needs a daemon (DOC-19) — per `agh-worktree-isolation` skill and `agh-qa-bootstrap`.
- For everything else: a clean monorepo checkout, `bun install` at root, `make codegen` green so `openapi/agh.json` is current, and `make cli-docs` green so the cobra tree under `content/runtime/cli-reference/` is current. No daemon required.
- For headless-browser scenarios (DOC-01, DOC-02, DOC-08, DOC-09, DOC-22): serve `packages/site/out/` with `npx serve out -p <unique-port>` (or `bun x serve`) bound to `127.0.0.1:<port>` and drive Playwright/Puppeteer against it. Different port per parallel run.
- Sequential codegen calls only — never run `make codegen` and `bun run generate:openapi` in parallel; the second reads `openapi/agh.json` produced by the first (per Workflow Rules "Never parallelize config writes against one isolated QA home").

## 5. Preconditions (apply to every scenario)

- Worktree clean: `git status` empty modulo intentional staged QA edits.
- `make verify` is green on the SUT branch (per the Critical Rules) — that closes the bun gate (`bun-lint`/`bun-typecheck`/`bun-test`/`web-build`) plus Go tests, fmt, lint, build, boundaries.
- For static-build scenarios: `make codegen` and `make cli-docs` both green, so `openapi/agh.json` and `content/runtime/cli-reference/**` are in lockstep with their sources.
- For static-build scenarios: `make site-build` green, producing `packages/site/out/`.
- For headless-browser scenarios: a static server bound to `127.0.0.1:<port>` serving `packages/site/out/`. Playwright (Chromium) installed via `bunx playwright install chromium`.
- For DOC-19 (real-LLM): direct `claude` auth comes from the effective Claude
  home for the lane (operator `HOME` by default; isolated `PROVIDER_HOME`
  only for explicit isolated-home scenarios); `agh provider show claude`
  reports the expected ACP command.

Per-scenario evidence layout under `.artifacts/qa/<run-id>/doc-XX/`:

- `doc-XX-report.md` (Worked / Failed / Blocked / Follow-up)
- `doc-XX-summary.json` (machine-readable)
- `doc-XX-output.log` (combined stdout/stderr)
- Per-scenario raw HTML/CSS captures, link-crawl logs, screenshots, and diffs as named below.

## 6. Cleanup (applies to every scenario)

- Stop any background static server (`pkill -f "serve out -p <port>"` or kill PID from manifest).
- Stop any background Playwright runner.
- For DOC-19: `agh daemon stop` (or kill PID from manifest).
- Archive any HTML/CSS/screenshot captures alongside the report bundle.
- Tear down the worktree only after evidence artifacts are written.

## 7. Mandatory scenarios

### DOC-01 — Hero is the canonical 2026-05-01 relock; first paint within budget

```yaml qa-scenario
id: doc-01-hero-relock
title: Production-style preview of `/` renders the locked hero copy and paints within budget
theme: docs-site.landing
coverage:
  primary:
    - landing.hero.locked
    - copy.hero.canonical
  secondary:
    - design.flat-depth
    - perf.first-paint
risk: high
live: false
provider: mock-acp
preconditions:
  - `make site-build` is green
  - Static server bound to 127.0.0.1:<port> serving packages/site/out/
docs_refs:
  - /Users/pedronauck/Dev/compozy/agh/COPY.md
  - /Users/pedronauck/Dev/compozy/agh/packages/site/CLAUDE.md
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/components/landing/hero.tsx:43-51
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/(home)/page.tsx:16-33
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/layout.tsx:27-72
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/opengraph-image.tsx:60-72
steps:
  - Build: `make site-build` (`Makefile:74-75`) and capture exit code.
  - Serve: `bunx serve packages/site/out -p $PORT` in the background; wait until 200 on `/`.
  - Navigate Chromium headless to `http://127.0.0.1:$PORT/`.
  - Assert the page <h1> text is exactly `An open workplace for AI agents.` (byte-equal, no trailing whitespace).
  - Assert the lead paragraph contains the locked subhead (substring match per `packages/site/CLAUDE.md:9`).
  - Capture First Contentful Paint via Performance Timing API; assert FCP <= 2500 ms on the loopback static server.
  - Capture full-page screenshot to `doc-01-home.png`.
expected:
  - <h1> matches the relock string exactly.
  - Lead paragraph matches the locked subhead.
  - FCP within budget.
  - No console errors logged in headless browser DevTools.
  - The OpenGraph image at `/opengraph-image` (statically exported PNG) embeds the same hero string at `packages/site/app/opengraph-image.tsx:61` (byte-grep the rendered PNG metadata is not required; assert the source string in the build output `out/opengraph-image*` is present).
evidence:
  - `doc-01-home.html`, `doc-01-home.png`, `doc-01-fcp.json`
  - `doc-01-hero-text.txt` (the exact rendered <h1> innerText)
failure_signatures:
  - <h1> deviates from the relock string → marketing copy regression; touches `components/landing/hero.tsx:44`.
  - Subhead missing or paraphrased → COPY.md drift; CLAUDE.md hero lock violated.
  - FCP > 2500 ms on loopback → asset regression; check landing hero `hero-bg.webp` size and Remotion player bundle.
  - Console errors → Remotion or `@remotion/player` import drift; check `components/landing/hero-player.tsx`.
cleanup:
  - Kill static server PID; remove `out/` only after evidence is written.
```

### DOC-02 — Sidebar resolution is exhaustive; no broken anchors

```yaml qa-scenario
id: doc-02-sidebar-exhaustive
title: Every runtime + protocol sidebar entry resolves to a 200 page; every internal link and hash anchor resolves
theme: docs-site.navigation
coverage:
  primary:
    - docs.sidebar.exhaustive
    - docs.linkrot.zero
risk: high
live: false
provider: mock-acp
preconditions:
  - DOC-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/source.ts:65-75
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/runtime-navigation.ts:117-132
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/internal-links.test.ts:171-220
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/runtime/[[...slug]]/page.tsx:13-39
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/protocol/[[...slug]]/page.tsx:13-39
steps:
  - Enumerate every runtime + protocol URL by reading the build's `.source/` cache (`runtime.toFumadocsSource()` and `protocol.toFumadocsSource()` outputs are persisted under `.source/`); fall back to crawling `/sitemap.xml` if `.source/` is unavailable.
  - For every URL `U`, request `http://127.0.0.1:$PORT$U` over HTTP and assert status 200 and `Content-Type` starts with `text/html`.
  - Parse the rendered HTML; collect every `<a href>` whose href starts with `/runtime/`, `/protocol/`, `/blog`, `/changelog`, or `/`. Resolve each against the served origin and assert each resolves to 200.
  - For every href containing a `#fragment`, find the `id="<fragment>"` (or matching heading slug) on the target page; assert presence and uniqueness.
expected:
  - Zero 404s across the discovered URL set.
  - Zero broken hash anchors.
  - The runtime sidebar layout matches `runtime-navigation.ts` ordering: Core Concepts (with Foundation/Capabilities/Workspace/Settings groups), CLI Reference, API Reference.
evidence:
  - `doc-02-urls.json` (the enumerated URL set)
  - `doc-02-broken-links.json` (must be empty)
  - `doc-02-broken-anchors.json` (must be empty)
failure_signatures:
  - 404 on a generated URL → `make site-build` did not include the route; check `generateStaticParams` in `app/runtime/[[...slug]]/page.tsx:41-43` and `app/protocol/[[...slug]]/page.tsx:41-43`.
  - Broken anchor → MDX heading rename without updating internal references; this also catches `internal-links.test.ts` regressions.
cleanup:
  - Kill static server.
```

### DOC-03 — Search returns results for "session", "memory", "extension"

```yaml qa-scenario
id: doc-03-search-coverage
title: `/api/search` answers `q=session`, `q=memory`, `q=extension` with non-empty results from both runtime and protocol indexes
theme: docs-site.search
coverage:
  primary:
    - docs.search.coverage
  secondary:
    - docs.search.dual_tag
risk: medium
live: false
provider: mock-acp
preconditions:
  - DOC-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/api/search/route.ts:6-27
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-search-index.test.ts:71-110
steps:
  - GET `http://127.0.0.1:$PORT/api/search?q=session`. Capture JSON.
  - GET `http://127.0.0.1:$PORT/api/search?q=memory`. Capture JSON.
  - GET `http://127.0.0.1:$PORT/api/search?q=extension`. Capture JSON.
  - For each response, assert `length(results) > 0` and at least one result with `tag === "Runtime"` and at least one with `tag === "AGH Network"` (where the term occurs in both trees).
  - Inspect the returned `url` field; resolve each to a 200 in the same static server.
expected:
  - All three queries return non-empty lists.
  - "session" returns Runtime results (sessions docs) AND AGH Network results (interactions/delivery talk about sessions).
  - "memory" returns Runtime results (memory docs); AGH Network may return zero — accept if zero, but Runtime must be non-empty.
  - "extension" returns Runtime results (extensions docs); AGH Network may return zero.
evidence:
  - `doc-03-search-session.json`, `doc-03-search-memory.json`, `doc-03-search-extension.json`
failure_signatures:
  - Zero results for "session" → search index regression in `app/api/search/route.ts`.
  - All results missing the `Runtime` tag → `runtimeDocs.getPages()` came up empty; check `.source/` generation.
  - Result `url` does not resolve → search index has stale URLs; codegen drift in `lib/source.ts`.
cleanup:
  - Kill static server.
```

### DOC-04 — API reference rendering: every endpoint visible

```yaml qa-scenario
id: doc-04-api-reference-faithful
title: Every OpenAPI tag has a rendered MDX page; every operation in `openapi/agh.json` is reachable from one of those pages
theme: docs-site.api-reference
coverage:
  primary:
    - docs.api_reference.tag_coverage
    - docs.api_reference.operation_coverage
  secondary:
    - codegen.openapi.lockstep
risk: high
live: false
provider: mock-acp
preconditions:
  - `make codegen` and `make codegen-check` both green
  - `bun run generate:openapi` has run as part of `make site-build`
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/scripts/generate-openapi.ts:96-118
  - /Users/pedronauck/Dev/compozy/agh/packages/site/content/runtime/api-reference/meta.json
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/runtime-api-reference.test.ts:36-64
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/openapi.ts:8
  - /Users/pedronauck/Dev/compozy/agh/openapi/agh.json
  - /Users/pedronauck/Dev/compozy/agh/internal/api/spec/spec.go:144-208
  - /Users/pedronauck/Dev/compozy/agh/internal/api/spec/spec_test.go:1219-1232
steps:
  - Read `openapi/agh.json` and enumerate every used tag and every operation under `paths.*.<method>`.
  - For each tag `T`, assert `packages/site/content/runtime/api-reference/<tagSlug(T)>.mdx` exists and renders an `APIPage` block.
  - Crawl the rendered page at `http://127.0.0.1:$PORT/runtime/api-reference/<tagSlug>` and assert each operation under `T` is referenced (operationId, method, or path appears in the rendered HTML body).
  - Cross-check against the `internal/api/spec.Operations()` registry: assert the count of distinct operationIds matches `internal/api/spec/spec_test.go:1219-1232` (currently 202).
expected:
  - Every used tag has its MDX file (asserted by existing `runtime-api-reference.test.ts`, re-confirmed at the rendered HTML level).
  - Every operationId appears at least once in the rendered tree.
  - The API reference index page (`/runtime/api-reference`) declares "built from `openapi/agh.json`" and references `make codegen-check` (per `runtime-docs-truth.test.ts:95-104`).
evidence:
  - `doc-04-tag-list.json` (tags from openapi)
  - `doc-04-mdx-list.json` (tag MDX files present)
  - `doc-04-operation-coverage.json` (operationId → rendered page url)
  - `doc-04-missing.json` (must be empty)
failure_signatures:
  - Missing MDX for a used tag → `bun run generate:openapi` did not run during build; or `cleanGenerated` removed the file (`scripts/generate-openapi.ts:16-23`).
  - OperationId missing from the rendered page → `fumadocs-openapi`'s `generateFiles` skipped the operation (extensions, vendor extensions, or schema validation error in the spec).
  - Operation count diverges from `internal/api/spec/spec_test.go:1219-1232` (202) → codegen drift; same failure mode as API-07 in `11-api-cli-parity.md`.
cleanup:
  - Kill static server.
```

### DOC-05 — CLI reference rendering: every cobra leaf documented

```yaml qa-scenario
id: doc-05-cli-reference-faithful
title: `make cli-docs` regenerates the cobra MDX tree; every CLI verb listed in `cli/root.go` is reachable from `/runtime/cli-reference/`
theme: docs-site.cli-reference
coverage:
  primary:
    - docs.cli_reference.verb_coverage
    - codegen.cli.lockstep
  secondary:
    - docs.cli_reference.idempotent
risk: high
live: false
provider: mock-acp
preconditions:
  - SUT branch checked out
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/Makefile:77-78
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/root.go:65-123
  - /Users/pedronauck/Dev/compozy/agh/packages/site/content/runtime/cli-reference/meta.json
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/runtime-manual-cli-examples.test.ts:89-98
steps:
  - Run `make cli-docs`; capture stdout, stderr, exit. Assert exit 0.
  - `git status -- packages/site/content/runtime/cli-reference/` — assert empty (deterministic regen).
  - Re-run `make cli-docs` immediately; assert `git status` still empty (idempotent).
  - Build the CLI tree by walking `cli.NewRootCommand()` (write a one-shot Go binary under `internal/cli/cli_tree_main.go` for the duration of the run, then delete) and emit every leaf verb path (e.g. `agh session new`, `agh task run claim`).
  - For each leaf verb path, assert a corresponding MDX file exists under `packages/site/content/runtime/cli-reference/` (folder + `.mdx` mapping per `runtime-manual-cli-examples.test.ts:89-98`).
  - Crawl `http://127.0.0.1:$PORT/runtime/cli-reference/<verb-path>/` for the first three verb paths chosen at random; assert 200 + the rendered page contains the verb's `## agh <verb>` heading.
expected:
  - `make cli-docs` is idempotent (zero diff on second run).
  - Every cobra leaf verb has its MDX file.
  - Hand-edited examples elsewhere in `content/` reference only verbs in this generated set (already enforced by `runtime-manual-cli-examples.test.ts` — re-confirmed end-to-end).
evidence:
  - `doc-05-cli-tree.json` (cobra leaves)
  - `doc-05-mdx-list.json` (CLI MDX files present)
  - `doc-05-cli-docs-output.log`
  - `doc-05-cli-docs-rerun-diff.txt` (must be empty)
failure_signatures:
  - `make cli-docs` non-zero → `cmd/agh doc` regression.
  - Idempotent re-run produces a diff → CLI doc generator emits non-deterministic ordering or timestamp.
  - Cobra leaf without MDX → CLI verb added without regenerating site tree (violates the `internal/CLAUDE.md` "No partial-surface completions" rule and `packages/site/CLAUDE.md:46` "do not hand-edit those files; edit the cobra command source instead").
cleanup:
  - Delete the one-shot CLI-tree binary.
  - `git status -- packages/site/content/runtime/cli-reference/` empty before report write.
```

### DOC-06 — MDX live blocks compile in static export

```yaml qa-scenario
id: doc-06-mdx-blocks
title: Every custom MDX block (Mermaid, GuideCard, GuideGrid, OperatorNote, RouteList, RouteRow, Workflow, WorkflowStep, APIPage) renders in the static export
theme: docs-site.mdx-blocks
coverage:
  primary:
    - docs.mdx.custom_blocks
  secondary:
    - docs.mermaid.dark
risk: medium
live: false
provider: mock-acp
preconditions:
  - DOC-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/mdx-components.tsx:24-37
  - /Users/pedronauck/Dev/compozy/agh/packages/site/components/docs/mdx-blocks.tsx
  - /Users/pedronauck/Dev/compozy/agh/packages/site/components/docs/mermaid.tsx
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/global.css:230-309 (agh-mermaid token overrides)
steps:
  - Grep `packages/site/content/` for usages of each custom block (`Mermaid`, `GuideCard`, `GuideGrid`, `OperatorNote`, `RouteList`, `RouteRow`, `Workflow`, `WorkflowStep`).
  - For each block, pick at least one MDX page that uses it; navigate Chromium to that page in the static server.
  - For `Mermaid`: assert an `svg.agh-mermaid-svg` element rendered with non-zero width/height. Assert it inherits `--color-text-primary`, `--color-surface`, etc., from the `.agh-mermaid` overrides in `app/global.css:230-309`.
  - For `APIPage`: assert at least one example response body block + one parameter table renders on `/runtime/api-reference/sessions/`.
  - For `GuideCard`/`GuideGrid`/`OperatorNote`/`RouteList`/`RouteRow`/`Workflow`/`WorkflowStep`: assert each renders the expected ARIA role/landmark plus the canonical CSS class.
expected:
  - Every custom block resolves to a non-error DOM subtree.
  - No `<pre data-mdx-error>` or fallback fragment appears.
  - Mermaid SVGs honor the dark token overrides; no light-theme bleed.
evidence:
  - `doc-06-block-coverage.json` (block → page → status)
  - `doc-06-mermaid-style.json` (CSS variables observed on the rendered svg)
failure_signatures:
  - A block renders as a literal MDX component error → `mdx-components.tsx` mapping missing or import path broken.
  - Mermaid not styled → `app/global.css:230-309` overrides regressed.
cleanup:
  - Kill static server.
```

### DOC-07 — Theme contract: dark-only, flat depth, locked palette + fonts

```yaml qa-scenario
id: doc-07-theme-contract
title: Every page is dark-only; no shadows; only DESIGN.md hex tokens and Inter / JetBrains Mono / Playfair Display (site-home only) / NuixyberNext (wordmark only) fonts appear
theme: docs-site.design-system
coverage:
  primary:
    - design.dark_only
    - design.no_shadow
    - design.palette
    - design.fonts
  secondary:
    - design.flat-depth
risk: high
live: false
provider: mock-acp
preconditions:
  - DOC-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/DESIGN.md
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/layout.tsx:78-95
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/global.css:14-31,46-63,310-313
  - /Users/pedronauck/Dev/compozy/agh/packages/ui/src/tokens.css
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/site-design-token-contract.test.ts:48-62
steps:
  - Build: confirm `out/` exists.
  - Static-scrape every CSS file under `packages/site/out/_next/static/css/*.css`. Concatenate.
  - Assert zero matches for `box-shadow` (excluding `box-shadow:none` if it appears as an explicit reset).
  - Assert zero matches for `drop-shadow` and `text-shadow`.
  - Extract every `#[0-9a-fA-F]{6,8}` hex from the concatenated CSS; assert each is also present in `packages/ui/src/tokens.css` (case-insensitive). The existing `site-design-token-contract.test.ts` enforces this on source files; this scenario re-runs the assertion on the **emitted** CSS bundle.
  - Extract every `font-family` declaration; assert each name is one of: `Inter`, `Inter Variable`, `JetBrains Mono`, `Playfair Display`, `NuixyberNext`, system fallbacks (`-apple-system`, `BlinkMacSystemFont`, `sans-serif`, `serif`, `monospace`, `ui-monospace`, `Courier New`).
  - Inspect rendered DOM on `/`: assert `<html class="dark …">` (per `app/layout.tsx:80-83`).
  - Inspect rendered DOM on `/runtime/`, `/protocol/`, `/blog/`, `/changelog/`: same `class="dark"` and no `prefers-color-scheme: light` adaptation.
  - Inspect every page that uses Playfair Display: confirm scope is `.site-home h1, .site-home h2` (per `app/global.css:310-313`) — Playfair must NOT appear on `/runtime/`, `/protocol/`, `/blog/`, `/changelog/` document headings.
  - Inspect NuixyberNext usage: confirm it appears only inside the wordmark `<Logo variant="logo" />` from `@agh/ui` — never in document body or headings.
expected:
  - Zero shadow declarations.
  - 100% of hex colors map to canonical tokens.
  - 100% of font-families are in the allowed set.
  - Dark mode is hardcoded; no light-mode toggle present (`baseOptions.themeSwitch.enabled = false` per `lib/layout.shared.tsx:16`).
evidence:
  - `doc-07-css.txt` (concatenated emitted CSS)
  - `doc-07-shadow-violations.json` (must be empty)
  - `doc-07-color-violations.json` (must be empty)
  - `doc-07-font-violations.json` (must be empty)
  - `doc-07-html-class.json` (per route)
failure_signatures:
  - Shadow declared → DESIGN.md flat-depth violation; check Tailwind `shadow-*` use in components.
  - Off-palette hex → `site-design-token-contract.test.ts` would have caught it at source — emission-level violation means a third-party stylesheet leaked in.
  - Disallowed font → font import added without DESIGN.md update.
  - Light-mode bleed → Fumadocs neutral.css override removed.
cleanup:
  - Kill static server.
```

### DOC-08 — Mobile / responsive at 390×844

```yaml qa-scenario
id: doc-08-mobile-390
title: At 390×844 (iPhone 14), no horizontal overflow, sidebar collapses, nav remains usable
theme: docs-site.responsive
coverage:
  primary:
    - responsive.mobile
  secondary:
    - design.flat-depth
risk: medium
live: false
provider: mock-acp
preconditions:
  - DOC-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/global.css:315-332 (mobile docs body)
  - /Users/pedronauck/Dev/compozy/agh/packages/site/components/site/home-header.tsx:110-116 (mobile nav)
  - /Users/pedronauck/Dev/compozy/agh/packages/site/components/site/docs-header.tsx
steps:
  - Configure Chromium with viewport 390×844 (DPR 3).
  - Visit `/`, `/runtime/`, `/protocol/`, `/blog/`, `/changelog/`, `/runtime/cli-reference/`, `/runtime/api-reference/sessions/`.
  - For each, assert `document.documentElement.scrollWidth <= window.innerWidth + 1` (no horizontal overflow).
  - Assert the bottom nav row in `home-header` (`components/site/home-header.tsx:110-116`) is visible and scrollable horizontally if needed (`overflow-x-auto`).
  - For docs routes, assert the sidebar collapses (Fumadocs notebook layout) and a sidebar-trigger button is reachable.
  - Capture screenshots per route to `doc-08-<route>.png`.
expected:
  - Zero horizontal overflow.
  - Mobile nav reachable on every route.
  - Sidebar trigger renders.
evidence:
  - `doc-08-overflow.json`, `doc-08-<route>.png`
failure_signatures:
  - Horizontal overflow → fixed-width content (table, code block, image) without overflow handling.
  - Sidebar trigger missing → notebook-layout regression.
cleanup:
  - Kill static server.
```

### DOC-09 — Accessibility: keyboard nav, aria-labels, landmarks, contrast

```yaml qa-scenario
id: doc-09-a11y
title: Keyboard navigation works; landmarks present; icons carry aria-labels; color contrast clears AA
theme: docs-site.accessibility
coverage:
  primary:
    - a11y.landmark
    - a11y.keyboard
    - a11y.icon_label
    - a11y.contrast
risk: high
live: false
provider: mock-acp
preconditions:
  - DOC-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/layout.tsx:85-91 (Skip-to-content link)
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-icon-accessibility.test.ts
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-landmark-accessibility.test.ts
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-visual-accessibility.test.ts
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-aside-accessibility.test.ts
steps:
  - Run axe-core (or `@axe-core/playwright`) headless against `/`, `/runtime/`, `/protocol/`, `/blog/`, `/changelog/`, one runtime doc page, one protocol doc page, one CLI reference page, one API reference page.
  - Assert axe returns zero violations of severity `serious` or higher.
  - Tab through `/`: assert the first tab focuses the "Skip to content" link (`app/layout.tsx:85-91`); pressing Enter scrolls to `#main-content` (per the landing `<main id="main-content" …>` at `app/(home)/page.tsx:18`).
  - Tab to GitHub link in header (`components/site/home-header.tsx:90-105`); assert focus ring is visible (`focus-visible:ring`).
  - Inspect every `<svg>` icon: assert either `aria-label`, `aria-hidden="true"`, or wrapping `<a aria-label>`.
  - Inspect every `<main>`, `<nav>`, `<header>`, `<footer>`: assert exactly one `<main>` per page; `<header role="banner">` and `<footer role="contentinfo">` either implicit or explicit.
  - Color contrast: pick the secondary text token `--color-text-secondary` (`#8E8E93`) on the canvas (`#141312`) and assert the computed contrast ratio meets WCAG AA (>= 4.5 for normal text or >= 3.0 for large text). The DESIGN.md token system is dark-only and was tuned for AA — this scenario verifies the tuning still holds in production CSS.
expected:
  - Axe: zero serious violations.
  - Skip-to-content focus path works.
  - Every icon has a label or is decorative.
  - Each route has exactly one `<main>` with `id="main-content"`.
evidence:
  - `doc-09-axe-<route>.json`
  - `doc-09-skip-to-content.txt`
  - `doc-09-icon-audit.json`
  - `doc-09-contrast.json`
failure_signatures:
  - Axe serious violation → DESIGN.md / accessibility regression.
  - Skip link not first focusable → layout regression.
  - Icon without label → component import regressed `aria-hidden`/`aria-label`.
cleanup:
  - Kill static server.
```

### DOC-10 — External link rot (or marked archive)

```yaml qa-scenario
id: doc-10-external-link-rot
title: Every external href in the static export resolves to 200/301/302, or is explicitly marked `archive`
theme: docs-site.external-links
coverage:
  primary:
    - docs.linkrot.external
risk: medium
live: false
provider: mock-acp
preconditions:
  - DOC-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/content-external-links.test.ts
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-link-safety.test.ts
steps:
  - Crawl every HTML page under `packages/site/out/`.
  - Collect every `<a href>` whose href starts with `http://` or `https://`.
  - Deduplicate the set; HEAD-request each (5s timeout, 3 retries with backoff). Treat 301/302 as alive after one hop (record the hop).
  - For any 4xx/5xx after retries, check whether the href appears in an MDX block annotated `archive` (e.g. `<a data-archive="true">` or a sibling `OperatorNote` indicating archived).
expected:
  - Every external href resolves to 200/301/302, or is documented as `archive`.
  - Zero `noreferrer noopener` regressions on `target="_blank"` links (already gated by `public-link-safety.test.ts`).
evidence:
  - `doc-10-external-links.json`
  - `doc-10-rotted.json` (must be empty)
failure_signatures:
  - 4xx/5xx for a non-archive href → external link rot; either fix the href or annotate as archive.
  - `target="_blank"` without `rel="noreferrer noopener"` → security regression in MDX.
cleanup:
  - Kill static server.
```

### DOC-11 — Image alt-text discipline

```yaml qa-scenario
id: doc-11-image-alt
title: Every image in the static export has a non-empty alt; decorative images use empty alt only when wrapped in aria-hidden context
theme: docs-site.images
coverage:
  primary:
    - a11y.alt_text
  secondary:
    - design.images
risk: medium
live: false
provider: mock-acp
preconditions:
  - DOC-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-media-quality.test.ts
  - /Users/pedronauck/Dev/compozy/agh/packages/site/components/landing/hero.tsx:31 (aria-hidden background)
steps:
  - Crawl every HTML page under `packages/site/out/`; collect every `<img>` and the parent's `aria-hidden` attribute.
  - For each `<img>`:
    - If `alt` attribute is missing → fail.
    - If `alt=""` and the parent (or an ancestor up to `<main>`) does NOT carry `aria-hidden="true"` → fail (decorative usage must be explicit).
    - If `alt=""` and an ancestor is aria-hidden → pass (decorative).
  - Cross-reference Velite blog cover declarations in `lib/blog.ts:7-14` and ensure every featured cover has an `alt` field.
expected:
  - Zero images without alt.
  - Decorative images are explicitly aria-hidden.
evidence:
  - `doc-11-images.json`, `doc-11-alt-violations.json`
failure_signatures:
  - Missing alt → MDX or component regression; check `<Image>` props or `<img>` usage.
  - Decorative without aria-hidden → ambiguity that breaks screen readers.
cleanup:
  - Kill static server.
```

### DOC-12 — Build determinism

```yaml qa-scenario
id: doc-12-build-determinism
title: `make site-build` twice produces byte-identical `out/` modulo file mtime
theme: docs-site.build
coverage:
  primary:
    - build.determinism
risk: medium
live: false
provider: mock-acp
preconditions:
  - Worktree clean
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/Makefile:74-75
  - /Users/pedronauck/Dev/compozy/agh/packages/site/next.config.mjs
  - /Users/pedronauck/Dev/compozy/agh/packages/site/package.json:11 (`prebuild` runs codegen + source + content)
steps:
  - `rm -rf packages/site/out packages/site/.next packages/site/.source packages/site/.velite`.
  - Run `make site-build` (run #1). Capture exit + duration. Snapshot `out/` to `out-1/` via `cp -R`.
  - `rm -rf packages/site/out packages/site/.next packages/site/.source packages/site/.velite`.
  - Run `make site-build` (run #2). Snapshot `out/` to `out-2/`.
  - Compute SHA-256 of every file in `out-1/` and `out-2/`. Diff the resulting hash maps.
  - Allow only mtime differences (`stat --format='%Y' …`) — content must be identical.
expected:
  - 100% of files with byte-identical content between runs.
  - The diff is empty modulo file mtime (which is unavoidable on disk).
evidence:
  - `doc-12-hashes-1.json`, `doc-12-hashes-2.json`, `doc-12-diff.json`
failure_signatures:
  - File content differs between runs → non-determinism in MDX compilation, content hashing, OG image generation, or RSS feed (timestamp leak).
  - `prebuild` failed in run #2 → codegen regression that depends on the previous build's outputs.
cleanup:
  - Remove `out-1/`, `out-2/` after evidence write.
```

### DOC-13 — Bun gate green on a clean checkout

```yaml qa-scenario
id: doc-13-bun-gate
title: `make bun-lint`, `make bun-typecheck`, `make bun-test` all pass on a fresh `bun install`
theme: docs-site.bun-gate
coverage:
  primary:
    - bun.lint
    - bun.typecheck
    - bun.test
  secondary:
    - bun.test.site_specific
risk: high
live: false
provider: mock-acp
preconditions:
  - Fresh `bun install` at repo root completed
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md (Bun workspaces — monorepo-wide)
  - /Users/pedronauck/Dev/compozy/agh/packages/site/package.json
  - /Users/pedronauck/Dev/compozy/agh/packages/site/vitest.config.ts
  - /Users/pedronauck/Dev/compozy/agh/vitest.config.ts (root projects)
steps:
  - Run `make bun-lint`. Capture exit + stderr. Assert exit 0 (zero tolerance — root CLAUDE.md).
  - Run `make bun-typecheck`. Capture exit + stderr. Assert exit 0.
  - Run `make bun-test`. Capture exit + stderr. Assert exit 0.
  - From the bun-test output, parse the `site` project's per-spec results; assert every `packages/site/lib/*.test.ts` and `packages/site/components/**/*.test.tsx` is reported.
expected:
  - All three commands exit 0.
  - The `site` project in `bun-test` runs every site spec; no spec is `skipped` unintentionally.
evidence:
  - `doc-13-bun-lint.log`, `doc-13-bun-typecheck.log`, `doc-13-bun-test.log`
  - `doc-13-site-spec-coverage.json` (run vs. on-disk count)
failure_signatures:
  - Lint failure → oxlint/oxfmt regression.
  - Typecheck failure → tsgo regression; check `pretypecheck` ran the OpenAPI + source + content generators.
  - Test failure → behavioral regression; resolve before final-qa hand-off.
cleanup:
  - Nothing.
```

### DOC-14 — COPY.md adherence: vocabulary scrape

```yaml qa-scenario
id: doc-14-copy-vocab
title: Scrape MDX prose and rendered HTML for `recipe`, `workflow`, `procedure`, `playbook` referring to current AGH artifacts; flag every occurrence
theme: docs-site.copy
coverage:
  primary:
    - copy.glossary.canonical
risk: high
live: false
provider: mock-acp
preconditions:
  - DOC-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/COPY.md
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md (Vocabulary & Product Strategy section)
  - /Users/pedronauck/Dev/compozy/agh/docs/_memory/glossary.md
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/site-copy-contract.test.ts
steps:
  - Concatenate every MDX file under `packages/site/content/runtime/` and `packages/site/content/protocol/` (excluding `cli-reference/` and `api-reference/` which are generated and may legitimately mention `workflow` as part of the OpenAPI tag name `tasks` → `Workflow` icon).
  - Concatenate every rendered HTML page under `packages/site/out/` (excluding `/runtime/cli-reference/` and `/runtime/api-reference/` for the same reason).
  - For each banned term `recipe`, `procedure`, `playbook`, `workflow`, count occurrences. The term `workflow` is allowed when used as the OpenAPI tag `Workflow` (capital W) on icon labels — exclude that exact context.
  - For each occurrence, assert it does NOT refer to a current AGH artifact (e.g., "this capability is a workflow" → fail; "this is not a workflow engine" → pass per COPY.md §2 "What AGH Is Not").
expected:
  - Zero violations referring to current AGH artifacts.
  - Allowed: explicit "AGH is not a workflow engine" framings (per COPY.md §2).
evidence:
  - `doc-14-mdx-occurrences.json`
  - `doc-14-html-occurrences.json`
  - `doc-14-violations.json` (must be empty after manual review)
failure_signatures:
  - "Recipe", "procedure", "playbook" referring to AGH capabilities → COPY.md drift; canonical artifact name is `capability`.
  - "Workflow" referring to a capability → same drift; capabilities are interpretive, not deterministic programs (RFC 002 / `docs/_memory/glossary.md`).
cleanup:
  - Nothing.
```

### DOC-15 — DESIGN.md adherence: emitted-CSS scrape

```yaml qa-scenario
id: doc-15-design-css-scrape
title: Scrape `out/_next/static/css/*.css` for shadow declarations, off-palette hex, and disallowed font families
theme: docs-site.design
coverage:
  primary:
    - design.no_shadow
    - design.palette
    - design.fonts
risk: high
live: false
provider: mock-acp
preconditions:
  - DOC-12 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/DESIGN.md
  - /Users/pedronauck/Dev/compozy/agh/packages/ui/src/tokens.css
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/global.css:14-31,46-63,310-313
steps:
  - Concatenate every CSS file under `packages/site/out/_next/static/css/`.
  - Run the same scrape as DOC-07, but on the EMITTED CSS (DOC-07 also covers the source files via the existing `site-design-token-contract.test.ts`).
  - Bonus: assert the only `@font-face` declaration is `NuixyberNext` (`app/global.css:14-20`); the other three fonts (Inter, JetBrains Mono, Playfair Display) come from `next/font/google` and are inlined as data URIs.
expected:
  - Zero shadow declarations.
  - Zero off-palette hex.
  - Zero disallowed font-families.
  - Exactly one `@font-face` declaration (NuixyberNext).
evidence:
  - `doc-15-emitted-css.txt`
  - `doc-15-shadow-violations.json`
  - `doc-15-color-violations.json`
  - `doc-15-font-violations.json`
  - `doc-15-fontface-list.json`
failure_signatures:
  - Same as DOC-07, plus an unexpected `@font-face` → font import added without DESIGN.md update.
cleanup:
  - Nothing.
```

### DOC-16 — SEO + OpenGraph: stable title, description, og:image; no duplicates

```yaml qa-scenario
id: doc-16-seo-og
title: Every static-export page has a stable `<title>`, `<meta name="description">`, and `og:image`; no duplicate metadata within a page
theme: docs-site.seo
coverage:
  primary:
    - seo.title
    - seo.description
    - seo.og_image
  secondary:
    - seo.canonical
risk: medium
live: false
provider: mock-acp
preconditions:
  - DOC-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/layout.tsx:27-72
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/site-config.ts:18-63 (createPageMetadata)
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/runtime/[[...slug]]/page.tsx:45-57
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/protocol/[[...slug]]/page.tsx:45-57
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/blog/[slug]/page.tsx:32-50
steps:
  - Crawl every HTML page in `packages/site/out/`.
  - For each page, parse the `<head>` and assert:
    - Exactly one `<title>` tag with non-empty text.
    - Exactly one `<meta name="description">` with non-empty content.
    - Exactly one `<meta property="og:image">` with non-empty content (default `/opengraph-image`, may be overridden per-page e.g. blog post cover).
    - Exactly one `<link rel="canonical">` whose href starts with `https://agh.network`.
    - Exactly one `<meta name="twitter:card">`.
  - Aggregate all titles + descriptions across the export; flag any duplicate `(title, description)` pair on routes that should be unique (e.g., `/runtime/core/sessions/lifecycle` should not share metadata with `/runtime/core/sessions/permissions`).
expected:
  - Zero pages missing any of the required meta.
  - Zero duplicates across distinct content routes.
  - Title template `"%s | AGH"` (root layout) is honored on every non-home page.
evidence:
  - `doc-16-meta.json` (per route)
  - `doc-16-missing.json` (must be empty)
  - `doc-16-duplicates.json` (must be empty)
failure_signatures:
  - Missing title → `generateMetadata` regression in a route.
  - Duplicate meta → page passing a static config that overrides `createPageMetadata`.
  - Canonical URL points at non-https or non-`agh.network` origin → site-config regression.
cleanup:
  - Nothing.
```

### DOC-17 — Sitemap and robots.txt sanity

```yaml qa-scenario
id: doc-17-sitemap-robots
title: `/sitemap.xml` lists every page; `/robots.txt` is canonical and points at the sitemap
theme: docs-site.sitemap
coverage:
  primary:
    - sitemap.completeness
    - robots.canonical
risk: medium
live: false
provider: mock-acp
preconditions:
  - DOC-01 preconditions
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/sitemap.ts:8-25
  - /Users/pedronauck/Dev/compozy/agh/packages/site/app/robots.ts:6-14
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-route-metadata.test.ts:48-78
steps:
  - GET `/sitemap.xml` from the static server. Parse XML.
  - Parallel-enumerate the expected URL set: `/`, `/blog`, `/changelog`, every `runtimeDocs.getPages()` URL, every `protocolDocs.getPages()` URL, every blog post permalink, every category page (`/blog/categories/<category>`).
  - Assert sitemap URL set ⊇ expected set (and conversely, no extra URLs).
  - Assert every URL in the sitemap is HTTPS, on the `agh.network` origin, terminates with `/` (per `canonicalPath`).
  - GET `/robots.txt`. Assert it allows `/` for `*` and the `Sitemap:` line points at `https://agh.network/sitemap.xml`.
expected:
  - Sitemap URL set == expected set.
  - Sitemap URLs are canonical HTTPS.
  - robots.txt is sane.
evidence:
  - `doc-17-sitemap.json` (parsed entries)
  - `doc-17-expected.json`
  - `doc-17-robots.txt`
failure_signatures:
  - Missing URL → sitemap generator regression in `app/sitemap.ts`.
  - Extra URL → stale entry leaking from removed content.
  - Non-HTTPS or non-`agh.network` URL → `siteConfig.url` drift.
cleanup:
  - Nothing.
```

### DOC-18 — `@agh/ui` consumption is documented or storied

```yaml qa-scenario
id: doc-18-agh-ui-consumption
title: Every `@agh/ui` primitive imported by the site has either a story under `packages/ui/` or a documentation page
theme: docs-site.ui-kit
coverage:
  primary:
    - ui.storybook_adjacent
risk: low
live: false
provider: mock-acp
preconditions:
  - SUT branch checked out
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/CLAUDE.md (Cross-References)
  - /Users/pedronauck/Dev/compozy/agh/packages/ui/
  - /Users/pedronauck/Dev/compozy/agh/packages/site/components/landing/install-section.tsx:4 (`buttonVariants`)
  - /Users/pedronauck/Dev/compozy/agh/packages/site/components/site/home-header.tsx:4 (`Logo`, `buttonVariants`, `cn`)
  - /Users/pedronauck/Dev/compozy/agh/packages/site/components/site/site-footer.tsx:1 (`Logo`, `cn`)
steps:
  - `grep -rE 'from "@agh/ui(?:/[a-z-]+)?"' packages/site/app packages/site/components packages/site/lib` → enumerate the set of imported symbols.
  - For each symbol, locate either:
    - A Storybook story under `packages/ui/src/**/*.stories.tsx` (or wherever the package places stories), OR
    - A documentation page under `packages/site/content/` that demonstrates the primitive.
  - Flag any imported symbol with neither.
expected:
  - 100% of imported `@agh/ui` symbols are documented or storied.
evidence:
  - `doc-18-imports.json`
  - `doc-18-coverage.json`
  - `doc-18-undocumented.json` (acceptable if non-empty but documented as follow-up)
failure_signatures:
  - Symbol imported but neither storied nor documented → undocumented surface; track as follow-up. Not blocking unless the symbol is a public-facing UI element.
cleanup:
  - Nothing.
```

### DOC-19 — Real-LLM "Try it" embed harness (or honest absence)

```yaml qa-scenario
id: doc-19-tryit-real-llm
title: If the site offers a "Try it" embed, prove it connects to a real daemon and runs a real Claude Code prompt; otherwise prove the test-mode harness is documented and the site does NOT execute live prompts
theme: docs-site.embed
coverage:
  primary:
    - embed.real_llm
    - embed.honesty
  secondary:
    - truthful_ui
risk: high
live: true
provider: real-claude-code
preconditions:
  - Bootstrap manifest with isolated `AGH_HOME`, daemon HTTP port, and the
    effective Claude home for the lane documented (`HOME` by default,
    `PROVIDER_HOME` only for explicit isolated-home scenarios)
  - Daemon up: `agh status -o json` reports `status="running"`
  - Direct `claude` auth available in the effective Claude home for the lane
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/components/landing/hero-player.tsx (Remotion Player)
  - /Users/pedronauck/Dev/compozy/agh/packages/site/remotion/hero/composition.tsx
  - /Users/pedronauck/Dev/compozy/agh/packages/site/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/03-acp-sessions.md (sibling for real prompt path)
steps:
  - Audit `packages/site/app/`, `packages/site/components/`, `packages/site/lib/` for any client-side fetch to a daemon endpoint (`/api/sessions/.../prompt`, `/api/sessions/.../stream`, `agh.network` itself, or a staged daemon URL). Use `grep -rE 'fetch\(|EventSource|/api/sessions|/api/observe' packages/site/`.
  - **Today's expected outcome**: zero client-side daemon prompt calls (the only `Player` is Remotion's local `HeroChatComposition`; there is no live ACP).
  - If audit finds a "Try it" embed (e.g., a future feature):
    - Drive a headless browser to the page hosting the embed.
    - Type a known-good prompt ("Read README.md and summarize the title in one sentence").
    - Assert the embed receives an SSE stream from the daemon and renders text in real time.
    - Cross-reference the daemon's `events.db` for the matching session.
  - If audit finds none (the truthful current state):
    - Assert the hero animation is a Remotion `<Player>` composition (`components/landing/hero-player.tsx:46-60`), not a live LLM stream.
    - Assert the composition runs locally (no network call) — verify by intercepting Network requests in the headless browser; expect zero requests to `127.0.0.1:<daemon-port>` or `agh.network/api/`.
    - Assert the README/landing copy never claims interactive embeds today.
    - Document the test-mode harness for any future embed: a future scenario template that drives the embed against an isolated daemon with `AGH_WEB_API_PROXY_TARGET` exported (per CLAUDE.md "Isolated Web QA must export `AGH_WEB_API_PROXY_TARGET`").
expected:
  - Truthful UI: today, the site renders a Remotion preview, NOT a live LLM stream. The QA report explicitly states "no live embed today; honest preview".
  - If a future embed is added: it MUST connect to a real daemon and produce a real SSE event sequence (mirroring API-02 in `11-api-cli-parity.md`).
evidence:
  - `doc-19-fetch-audit.txt` (output of the grep)
  - `doc-19-network-trace.har` (browser HAR for the homepage)
  - `doc-19-report.md` (Worked / Failed / Blocked / Follow-up)
failure_signatures:
  - Site contains a hidden client-side LLM call without operator awareness → truthful-UI violation; immediate blocker.
  - "Try it" embed claimed in copy but not implemented → COPY.md / runtime-truth divergence.
cleanup:
  - `agh daemon stop` (or kill PID from manifest).
```

### DOC-20 — Codegen co-ship doc: API reference updates lockstep

```yaml qa-scenario
id: doc-20-api-codegen-coship
title: Edit `internal/api/contract/responses.go`, run `make codegen` then `bun run generate:openapi`, prove the API reference tree updates lockstep with no orphan tag MDX
theme: docs-site.codegen
coverage:
  primary:
    - codegen.api_reference.lockstep
  secondary:
    - api.contract.coship
risk: high
live: false
provider: mock-acp
preconditions:
  - Worktree clean; `make codegen-check` green
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/contract/responses.go
  - /Users/pedronauck/Dev/compozy/agh/openapi/agh.json
  - /Users/pedronauck/Dev/compozy/agh/packages/site/scripts/generate-openapi.ts:96-118
  - /Users/pedronauck/Dev/compozy/agh/packages/site/content/runtime/api-reference/
steps:
  - Save backups of `internal/api/contract/responses.go`, `openapi/agh.json`, and the entire `packages/site/content/runtime/api-reference/` tree.
  - Add a new field to a contract struct (e.g. `DaemonStatusPayload.SchemaTestField string`).
  - Run `make codegen`. Assert exit 0; assert `openapi/agh.json` updated.
  - Run `bun run generate:openapi` (from `packages/site/`). Assert exit 0.
  - Inspect `packages/site/content/runtime/api-reference/`: assert (a) every previously-existing tag MDX still exists, (b) any new tag (if the contract change introduced one) has a fresh MDX, (c) `meta.json` reflects the updated tag set.
  - Build: `make site-build`. Assert exit 0.
  - Crawl `/runtime/api-reference/daemon/` (the tag impacted) and confirm the rendered page references the new field.
  - Revert all changes; rerun `make codegen` and `bun run generate:openapi`; assert `git diff` is empty in `internal/api/contract/`, `openapi/`, `packages/site/content/runtime/api-reference/`, and `web/src/generated/`.
expected:
  - Codegen + site reference regen are lockstep; no orphan files; `meta.json` accurate.
  - Final `git diff` is empty after revert.
evidence:
  - `doc-20-openapi-diff.txt`
  - `doc-20-api-ref-diff.txt`
  - `doc-20-final-git-diff.txt` (must be empty)
failure_signatures:
  - `make codegen` succeeds but `bun run generate:openapi` fails → fumadocs-openapi spec validation regression.
  - Orphan `<tag>.mdx` remains after a tag is removed → `cleanGenerated` (`scripts/generate-openapi.ts:16-23`) preserves files it shouldn't (the `PRESERVE` set is `index.mdx` only — anything else stale is a bug).
  - Final `git diff` non-empty → revert path broken; investigate.
cleanup:
  - Restore from backups; assert worktree clean.
```

### DOC-21 — CLI codegen co-ship doc: cli-reference updates lockstep

```yaml qa-scenario
id: doc-21-cli-codegen-coship
title: Add a new cobra subcommand under `internal/cli/`, run `make cli-docs`, prove the site CLI reference tree updates and removes orphan files when the command is renamed
theme: docs-site.codegen
coverage:
  primary:
    - codegen.cli_reference.lockstep
  secondary:
    - cli.contract.coship
risk: high
live: false
provider: mock-acp
preconditions:
  - Worktree clean
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/root.go
  - /Users/pedronauck/Dev/compozy/agh/Makefile:77-78
  - /Users/pedronauck/Dev/compozy/agh/packages/site/content/runtime/cli-reference/
steps:
  - Backup `internal/cli/root.go` and `packages/site/content/runtime/cli-reference/`.
  - Add a temporary subcommand: e.g. `agh whoami debug` (a no-op leaf under the existing `whoami` group). Wire it via cobra under the `whoami` parent.
  - Run `make cli-docs`. Assert exit 0; assert a new MDX appears at `packages/site/content/runtime/cli-reference/whoami/debug.mdx`.
  - Run `make site-build`. Crawl `/runtime/cli-reference/whoami/debug/`; assert 200 with a `## agh whoami debug` heading.
  - Rename the cobra command to `whoami debug-renamed`; run `make cli-docs` again; assert the old `debug.mdx` is removed and `debug-renamed.mdx` exists.
  - Revert: remove the temporary subcommand; run `make cli-docs`; assert `git diff -- internal/cli/ packages/site/content/runtime/cli-reference/` is empty.
expected:
  - New cobra subcommand → new MDX.
  - Renamed cobra subcommand → renamed MDX with no orphan.
  - Removed cobra subcommand → MDX gone; `git diff` empty after revert.
evidence:
  - `doc-21-cli-tree-before.json`
  - `doc-21-cli-tree-after-add.json`
  - `doc-21-cli-tree-after-rename.json`
  - `doc-21-final-git-diff.txt` (must be empty)
failure_signatures:
  - New subcommand without new MDX → `cmd/agh doc` regression; the doc generator is missing a leaf.
  - Rename leaves orphan MDX → `cmd/agh doc --output-dir` does not clean stale files; either the generator is meant to clean or `cli-docs` should `rm -rf` first; document the contract.
  - Final `git diff` non-empty → revert path broken.
cleanup:
  - Restore from backups; assert worktree clean.
```

### DOC-22 — `out/` is statically deployable; every route returns 200 with correct Content-Type

```yaml qa-scenario
id: doc-22-static-deployable
title: Serving `packages/site/out/` with a trivial static server returns 200 and correct Content-Type for every route enumerated by sitemap, and `_headers` is honored
theme: docs-site.deploy
coverage:
  primary:
    - deploy.static_export
  secondary:
    - deploy.headers
risk: high
live: false
provider: mock-acp
preconditions:
  - DOC-12 preconditions; `out/` present
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/packages/site/next.config.mjs (`output: "export"`, `trailingSlash: true`)
  - /Users/pedronauck/Dev/compozy/agh/packages/site/public/_headers
  - /Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-security-headers.test.ts
steps:
  - Serve `out/` with `bunx serve packages/site/out -p $PORT --no-clipboard --single`.
  - Read every URL from `/sitemap.xml`. For each:
    - GET the URL; assert status 200.
    - Assert `Content-Type` starts with `text/html` for HTML routes; `image/svg+xml` for `/favicon.svg`; `image/png` for `/apple-touch-icon.png`, `/icon-192.png`, `/icon-512.png`, `/opengraph-image`; `application/manifest+json` for `/site.webmanifest`; `application/rss+xml; charset=utf-8` for `/blog/feed.xml`; `application/xml` (or text/xml) for `/sitemap.xml`; `text/plain; charset=utf-8` for `/install.sh`.
  - Verify a representative HTML response carries the security headers configured in `public/_headers`: `Content-Security-Policy`, `Referrer-Policy: strict-origin-when-cross-origin`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Permissions-Policy`. Note that `bunx serve` does NOT apply `_headers` (that is Cloudflare-specific) — the assertion is that `_headers` is well-formed and matches `public-security-headers.test.ts` source-of-truth.
expected:
  - Zero non-200 routes.
  - Content-Type matches the resource kind.
  - `public/_headers` parses cleanly and matches the source-of-truth test.
evidence:
  - `doc-22-route-status.json`
  - `doc-22-content-types.json`
  - `doc-22-headers.txt`
failure_signatures:
  - 404 on a sitemap-enumerated URL → static export regression; route not generated.
  - Wrong Content-Type → server misconfiguration OR a route accidentally exporting binary as text.
  - `_headers` malformed → deploy regression; Cloudflare will silently drop the file.
cleanup:
  - Kill static server.
```

## 8. Reporting protocol

Every scenario MUST produce a Worked / Failed / Blocked / Follow-up report at `.artifacts/qa/<run-id>/doc-XX/doc-XX-report.md` (per the openclaw QA pattern). The aggregate report at `.artifacts/qa/<run-id>/13-docs-site-summary.md` MUST list:

- Total scenarios, scenarios passed, scenarios failed, scenarios blocked.
- Total runtime + protocol pages enumerated; total operationIds enumerated; total cobra leaves enumerated.
- Cross-reference every failure to one of the gap items in §3.
- Every external link rotted (DOC-10) becomes a doc-content follow-up issue.
- Every undocumented `@agh/ui` symbol (DOC-18) becomes a doc/storybook follow-up issue.
- Truthful-UI status (DOC-19): "no live embed today" or "live embed proven against daemon `<id>`".

## 9. Pass criteria (module-level)

The 13-docs-site module is **green** when:

- DOC-01..DOC-22 all reach status `Worked` (or `Worked-with-Follow-up` for DOC-18 if a small number of `@agh/ui` symbols are documented as follow-ups).
- `make verify` is green on the SUT branch (root CLAUDE.md Critical Rule).
- `make codegen-check` and `make cli-docs` are both green and idempotent.
- The static export under `packages/site/out/` is byte-deterministic across two clean runs.
- The hero copy is the canonical 2026-05-01 relock per COPY.md.
- DESIGN.md tokens, fonts, and flat-depth model are honored end-to-end.
- The truthful-UI status is documented (DOC-19).

## 10. Open questions / follow-ups

- **`agh.network` deploy parity**: this child runs against a local static server (`bunx serve out/`). The deployed `agh.network` runs behind Cloudflare Pages, which honors `public/_headers`. A follow-up scenario should run a smoke test against the deployed origin once a release is cut, asserting the same DOC-16 / DOC-22 contracts hold (separate from this pre-release child since deployment is post-merge).
- **Real-LLM embed**: when the team ships an interactive "Try it" embed, DOC-19 must be promoted from "honest absence" to a real-prompt scenario mirroring `API-02` in `11-api-cli-parity.md`.
- **Storybook adjacency (DOC-18)**: `packages/ui` may not yet host stories. If stories are not the chosen documentation path, document each consumed primitive in `content/runtime/` instead and update DOC-18's pass criteria.
- **Velite `changelog/` collection is empty today**: when the first Velite-backed `release` MDX lands, add a DOC-23 scenario asserting the `/changelog` page renders the release entry, the `ChangelogTocRail` shows the version, and the `cliff.toml` metadata aligns (per the looper canonical CI patterns).
- **Mermaid in dark mode (DOC-06)**: if Mermaid output ever shows a light-theme bleed, add a regression scenario locking down the `agh-mermaid` token overrides at the rendered-svg level.

## 11. Source map (file:line citations)

Authoritative references used by this child (every citation is repo-absolute):

- Hero copy lock: `/Users/pedronauck/Dev/compozy/agh/packages/site/components/landing/hero.tsx:43-51`, `/Users/pedronauck/Dev/compozy/agh/packages/site/CLAUDE.md:9`, `/Users/pedronauck/Dev/compozy/agh/COPY.md` §2-3.
- Site config: `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/site-config.ts:1-7`, `/Users/pedronauck/Dev/compozy/agh/packages/site/app/layout.tsx:27-72`.
- Fumadocs source: `/Users/pedronauck/Dev/compozy/agh/packages/site/source.config.ts:3-9`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/source.ts:65-75`.
- Velite: `/Users/pedronauck/Dev/compozy/agh/packages/site/velite.config.ts:1-85`.
- App routes: `/Users/pedronauck/Dev/compozy/agh/packages/site/app/(home)/page.tsx:16-33`, `/Users/pedronauck/Dev/compozy/agh/packages/site/app/runtime/[[...slug]]/page.tsx:13-57`, `/Users/pedronauck/Dev/compozy/agh/packages/site/app/protocol/[[...slug]]/page.tsx:13-57`, `/Users/pedronauck/Dev/compozy/agh/packages/site/app/blog/page.tsx:28-114`, `/Users/pedronauck/Dev/compozy/agh/packages/site/app/blog/[slug]/page.tsx:52-117`, `/Users/pedronauck/Dev/compozy/agh/packages/site/app/blog/feed.xml/route.ts:15-51`, `/Users/pedronauck/Dev/compozy/agh/packages/site/app/changelog/page.tsx:16-101`, `/Users/pedronauck/Dev/compozy/agh/packages/site/app/api/search/route.ts:6-27`, `/Users/pedronauck/Dev/compozy/agh/packages/site/app/sitemap.ts:8-25`, `/Users/pedronauck/Dev/compozy/agh/packages/site/app/robots.ts:6-14`, `/Users/pedronauck/Dev/compozy/agh/packages/site/app/opengraph-image.tsx:12-78`.
- Generators: `/Users/pedronauck/Dev/compozy/agh/packages/site/scripts/generate-openapi.ts:96-118`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/openapi.ts:1-12`, `/Users/pedronauck/Dev/compozy/agh/packages/site/package.json:7-19`, `/Users/pedronauck/Dev/compozy/agh/Makefile:69-78`.
- Custom MDX components: `/Users/pedronauck/Dev/compozy/agh/packages/site/mdx-components.tsx:1-37`, `/Users/pedronauck/Dev/compozy/agh/packages/site/components/docs/mdx-blocks.tsx`, `/Users/pedronauck/Dev/compozy/agh/packages/site/components/docs/mermaid.tsx`.
- Existing test contracts: `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/internal-links.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/runtime-api-reference.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/runtime-manual-cli-examples.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/runtime-docs-truth.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/site-design-token-contract.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/site-copy-contract.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/landing-truth.test.tsx`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-route-metadata.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-search-index.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-security-headers.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-media-quality.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-icon-accessibility.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-landmark-accessibility.test.ts`, `/Users/pedronauck/Dev/compozy/agh/packages/site/lib/public-link-safety.test.ts`.
- Design + copy: `/Users/pedronauck/Dev/compozy/agh/DESIGN.md`, `/Users/pedronauck/Dev/compozy/agh/COPY.md`, `/Users/pedronauck/Dev/compozy/agh/packages/ui/src/tokens.css`.
- Glossary: `/Users/pedronauck/Dev/compozy/agh/docs/_memory/glossary.md`.
- Sibling QA children: `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/04-autonomy-kernel.md`, `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/11-api-cli-parity.md`.
- QA pattern references: `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md`, `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md`.
