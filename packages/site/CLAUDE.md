# CLAUDE.md (packages/site)

Fumadocs documentation site at `compozy.com` — Next.js 16, Fumadocs 16, Velite blog content, Bun-managed. (Root `CLAUDE.md` rules apply — this file adds site-specific ones.)

## Critical Rules

- **Tokens from `packages/ui/src/tokens.css` + generated `DESIGN.md`** — no invented values. Site-only layout/type extensions go in `packages/site/app/global.css` `@theme inline`. After changing runtime/site theme tokens run `make codegen` + `make codegen-check`; never hand-edit generated `DESIGN.md` regions.
- **Eyebrow markup is mandatory** for every uppercase label: `<Eyebrow>` from `@compozy/ui` (children + `className` only) **or** the `eyebrow` utility class on structural elements; tone via `className`. Inlining `font-mono` + `uppercase` + `text-[…]` + `tracking-[…]` tuples is forbidden (that IS the utility), as are the removed `eyebrow-badge`/`eyebrow-micro` literals. Contract **Geist UC 11/600/-0.005em**. Full rule: `DESIGN.md` §3, `web/CLAUDE.md`, lesson `L-022`.
- **Product language from `COPY.md`** — landing copy, blog/changelog, docs, site config, OpenGraph/SEO metadata, and CTAs follow the copy system; terms per `docs/_memory/glossary.md` (`capability`, never `recipe`).
- **Launch hero lock:** ship `The only true OS for AI agents.` followed immediately by `A window on top of an agent isn't an OS. An OS runs the work, keeps the memory, sets the permissions, connects agents to each other — and lets you build on it. That's the test, and Compozy is the only one built to pass it.` The two are one contract; `COPY.md` owns the OS-first positioning and people-first register.
- **`packages/site` ships in the same PR as backend contract changes** that affect documented APIs/CLI verbs (per the `internal/api/contract` co-ship rule).
- **Test placement before any site test.** Name the invariant, owning layer, and canonical suite; update existing content/source/route/component suites first. No prose-string/snapshot/generated/file-existence tests unless that artifact is the product contract and no stronger gate exists.

## Build Commands

```bash
# Turbo-backed validation from the repo root:
make bun-typecheck / bun-test                       # full Bun workspace typecheck / test
bunx turbo run typecheck|test|build --filter=./packages/site   # focused @compozy/site

# Generators + local dev:
cd packages/site && bun run source:generate         # Fumadocs MDX -> .source/
cd packages/site && bun run content:generate        # Velite MDX/YAML -> .velite/
cd packages/site && bun run dev  (or make site-dev)  # next dev (predev runs both generators)
make cli-docs / make cli-docs-check                  # regenerate / verify the Cobra CLI reference
```

`predev` and direct builds run `generate`; Turbo-backed build/typecheck/test reuse the cacheable `generate:openapi → generate:content` graph. `.source/`, `.velite/`, `out/`, `.next/`, `tsconfig.tsbuildinfo` are generated — never commit them.

## Skill Dispatch

| Domain                        | Required Skills                          | Conditional Skills            |
| ----------------------------- | ---------------------------------------- | ----------------------------- |
| Fumadocs page authoring       | `documentation-writer`                   | `context7`                    |
| Marketing / landing copy      | `copywriting` + `documentation-writer`   | `seo-audit` + `ui-craft`      |
| Site UI / components          | `eng-design` + `ui-craft` + `impeccable` | `eng-ui-screenshot`           |
| Diagrams (architecture, flow) | `mermaid-diagrams`                       |                               |
| Next.js / SSR / app router    | `next-best-practices`                    | `vercel-react-best-practices` |
| Tailwind v4 styling           | `tailwindcss`                            |                               |
| TanStack (when used in site)  | `tanstack`                               |                               |
| Site testing                  | `eng-consolidate-test-suites` + `vitest` | `testing-boss`                |

## Coding Style

- TypeScript strict (no `any` when the concrete type is known). Functional React components only — no `React.FC`; named exports; kebab-case files; `@/*` alias.
- MDX lives under `content/docs/` (Fumadocs, single tree; protocol spec nests at `content/docs/network/protocol/`) and `content/blog/` (Velite). CLI docs auto-generate under `content/docs/cli/`, API docs under `content/docs/api/` — never hand-edit generated pages; edit the Cobra command source or `openapi/compozy.json`.
- Blog layout: `content/blog/posts/<slug>.mdx` and `content/blog/authors/<handle>.yml`. Frontmatter is zod-validated by `velite.config.ts` (broken frontmatter fails the build with line-numbered errors).
- Pages need `<title>` + meta via Fumadocs metadata helpers. Code blocks use the project syntax-highlight theme — no new variants.

## Truthful Docs > Plausible Docs

- Document only behavior the runtime supports today. When the Compozy Network RFC differs from the daemon, docs follow the daemon and link the RFC as "future profile".
- API/CLI references are generated from `openapi/compozy.json` + the Cobra command tree — never paraphrase; if the generated reference is wrong, fix the source.
- Changelog pages read published GitHub Releases from `v0.3.0-beta.1`; preserve their full notes, categories, pull-request evidence, contributors, and assets.

## Testing

Use `eng-consolidate-test-suites` before adding/moving a site test (record invariant, owning layer, canonical suite, verification command). A docs/site task needs a test _decision_, not automatic Vitest coverage — "no new test" is valid when source generation, route metadata, `make codegen-check`, build, or link checks already own the invariant. Validation MUST run through Turbo (`bunx turbo run test --filter=./packages/site` or `make bun-test`), never `cd packages/site && bun run test`. After changing `source.config.ts`, regenerate and re-run the focused typecheck.

## Cross-References

Root: `/CLAUDE.md`. Web runtime UI: `/web/CLAUDE.md`. Design tokens: `/packages/ui/src/tokens.css` → `/DESIGN.md`; site theme: `/packages/site/app/global.css`. Copy: `/COPY.md`. Memory/glossary/directives: `/docs/_memory/`.
