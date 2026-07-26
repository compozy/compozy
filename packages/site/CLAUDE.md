# CLAUDE.md (packages/site)

Fumadocs documentation site at `agh.network` — Next.js 16, Fumadocs 16, Remotion (protocol illustrations), Velite (`/blog` + `/changelog` content layer), Bun-managed. (Root `CLAUDE.md` rules apply — this file adds site-specific ones.)

## Critical Rules

- **Tokens from `packages/ui/src/tokens.css` + generated `DESIGN.md`** — no invented values. Site-only layout/type extensions go in `packages/site/app/global.css` `@theme inline`. After changing runtime/site theme tokens run `make codegen` + `make codegen-check`; never hand-edit generated `DESIGN.md` regions.
- **Eyebrow markup is mandatory** for every uppercase label: `<Eyebrow>` from `@agh/ui` (children + `className` only) **or** the `eyebrow` utility class on structural elements; tone via `className`. Inlining `font-mono` + `uppercase` + `text-[…]` + `tracking-[…]` tuples is forbidden (that IS the utility), as are the removed `eyebrow-badge`/`eyebrow-micro` literals. Contract **Inter UC 11/600/-0.005em**. Full rule: `DESIGN.md` §3, `web/CLAUDE.md`, lesson `L-022`.
- **Product language from `COPY.md`** — landing copy, blog/changelog, runtime/protocol docs, site config, OpenGraph/SEO metadata, and CTAs follow the copy system; terms per `docs/_memory/glossary.md` (`capability`, never `recipe`).
- **Hero positioning is locked**: headline "An open workplace for AI agents." with subhead "AGH runs the agent CLIs you already use as durable sessions — with memory, autonomy, tools, and automation — connected on agh-network/v0 channels where they find each other, share capabilities, and close work with receipts." Open-workplace-first. Do not propose alternative hero copy without explicit user approval.
- **`packages/site` ships in the same PR as backend contract changes** that affect documented APIs/CLI verbs (per the `internal/api/contract` co-ship rule).
- **Test placement before any site test.** Name the invariant, owning layer, and canonical suite; update existing content/source/route/component suites first. No prose-string/snapshot/generated/file-existence tests unless that artifact is the product contract and no stronger gate exists.

## Build Commands

```bash
# Turbo-backed validation from the repo root:
make bun-typecheck / bun-test                       # full Bun workspace typecheck / test
bunx turbo run typecheck|test|build --filter=./packages/site   # focused @agh/site

# Generators + local dev:
cd packages/site && bun run source:generate         # Fumadocs MDX -> .source/
cd packages/site && bun run content:generate        # Velite MDX/YAML -> .velite/
cd packages/site && bun run dev  (or make site-dev)  # next dev (predev runs both generators)
make cli-docs                                        # regenerate CLI reference from cobra JSON export
```

`predev` and direct builds run `generate`; Turbo-backed build/typecheck/test reuse the cacheable `generate:openapi → generate:content` graph. `.source/`, `.velite/`, `out/`, `.next/`, `tsconfig.tsbuildinfo` are generated — never commit them.

## Skill Dispatch

| Domain                        | Required Skills                          | Conditional Skills            |
| ----------------------------- | ---------------------------------------- | ----------------------------- |
| Fumadocs page authoring       | `documentation-writer`                   | `context7`                    |
| Marketing / landing copy      | `copywriting` + `documentation-writer`   | `seo-audit` + `ui-craft`      |
| Site UI / components          | `agh-design` + `ui-craft` + `impeccable` | `agh-ui-screenshot`           |
| Diagrams (architecture, flow) | `mermaid-diagrams`                       |                               |
| Next.js / SSR / app router    | `next-best-practices`                    | `vercel-react-best-practices` |
| Tailwind v4 styling           | `tailwindcss`                            |                               |
| TanStack (when used in site)  | `tanstack`                               |                               |
| Site testing                  | `consolidate-test-suites` + `vitest`     | `testing-boss`                |

## Coding Style

- TypeScript strict (no `any` when the concrete type is known). Functional React components only — no `React.FC`; named exports; kebab-case files; `@/*` alias.
- MDX lives under `content/runtime/` + `content/protocol/` (Fumadocs) and `content/blog/` (Velite). CLI docs auto-generate under `content/runtime/cli/` — never hand-edit; edit the cobra command source.
- Blog layout: `content/blog/posts/<slug>.mdx`, `content/blog/changelog/<version>.mdx`, `content/blog/authors/<handle>.yml`. Frontmatter is zod-validated by `velite.config.ts` (broken frontmatter fails the build with line-numbered errors).
- Pages need `<title>` + meta via Fumadocs metadata helpers. Code blocks use the project syntax-highlight theme — no new variants.

## Truthful Docs > Plausible Docs

- Document only behavior the runtime supports today. When the AGH Network RFC differs from the daemon, docs follow the daemon and link the RFC as "future profile".
- API/CLI references are generated from `openapi/agh.json` + the cobra JSON export — never paraphrase; if the generated reference is wrong, fix the source.
- Changelog entries (`content/blog/changelog/*.mdx`) reflect real merged work — source `added`/`changed`/`fixed`/`breaking` from `git log` + PR descriptions, not aspirational copy.

## Testing

Use `consolidate-test-suites` before adding/moving a site test (record invariant, owning layer, canonical suite, verification command). A docs/site task needs a test _decision_, not automatic Vitest coverage — "no new test" is valid when source generation, route metadata, `make codegen-check`, build, or link checks already own the invariant. Validation MUST run through Turbo (`bunx turbo run test --filter=./packages/site` or `make bun-test`), never `cd packages/site && bun run test`. After changing `source.config.ts`, regenerate and re-run the focused typecheck.

## Cross-References

Root: `/CLAUDE.md`. Web runtime UI: `/web/CLAUDE.md`. Design tokens: `/packages/ui/src/tokens.css` → `/DESIGN.md`; site theme: `/packages/site/app/global.css`. Copy: `/COPY.md`. Memory/glossary/directives: `/docs/_memory/`.
