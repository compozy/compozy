# Documentation Site

Fumadocs/Next.js site at `compozy.com`, with Velite blog content and Bun workspaces. Root `CLAUDE.md` owns compatibility, test placement, and delivery.

## Content and UI

- Read `COPY.md` for public language and its Hero Lock; the landing hero stays verbatim. Canonical vocabulary comes from `docs/_memory/glossary.md`.
- Document supported runtime behavior. Label RFC-only behavior as a future profile. Generated API/CLI references come from `openapi/compozy.json` and Cobra sources; repair those sources instead of hand-editing generated pages.
- Co-ship affected site docs with backend contracts, including SD-013 compatibility/deprecation guidance. Changelog pages preserve full published GitHub Release notes from `v0.3.0-beta.1`, including categories, PR evidence, contributors, and assets.
- Reuse `@compozy/ui`; tokens and generated `DESIGN.md` own visual values. Site-only extensions live in `app/global.css` `@theme inline`. Token changes run `make codegen` and `make codegen-check`.
- Structural micro-labels use `Eyebrow` or the `eyebrow` utility; casing and typography follow its token contract.
- MDX docs live in `content/docs/`, network protocol docs under `content/docs/network/protocol/`, blog posts under `content/blog/posts/`, and authors under `content/blog/authors/`. Preserve existing metadata and syntax-highlighting conventions.
- Use kebab-case files, named exports, functional components, strict types, and `@/*` imports. Generated `.source/`, `.velite/`, `out/`, `.next/`, and `tsconfig.tsbuildinfo` are not committed.

## References and Validation

Use `fumadocs` or `next-best-practices` for framework-specific work, `documentation-writer` for substantial doc structure, `copywriting` for marketing, and `eng-design` for redesign. Load only the relevant procedure; ordinary prose edits need no whole skill stack.

Choose the existing content/route/component suite that owns changed behavior. Source generation, build, metadata, and link checks often cover editorial changes without a new Vitest test. Frontend validation runs through Turbo from the repo root:

```bash
bunx turbo run typecheck --filter=./packages/site
bunx turbo run test --filter=./packages/site
bunx turbo run build --filter=./packages/site
make cli-docs
make cli-docs-check
```

`make gate` selects required delivery lanes. The Turbo `generate:openapi → generate:content` graph owns generation; local `make site-dev` runs dev generators. After `source.config.ts` changes, regenerate and run the focused typecheck. Reuse current check results for unchanged inputs.

<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
