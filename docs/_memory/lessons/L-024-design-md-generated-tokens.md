# L-024 — Generated design-token specs prevent documentation drift

**Class:** Frontend / Design system / Documentation tooling
**Date discovered:** 2026-05-12 (DESIGN.md token-drift refactor)
**Evidence sources:** `DESIGN.md`, `packages/ui/src/tokens.css`,
`packages/site/app/global.css`, `scripts/sync-design-md.mjs`,
`.agents/skills/agh/agh-design/SKILL.md`, `magefile.go`, and the prior L-022 /
L-023 design-token consolidation.

## Context

`DESIGN.md` had grown into a hand-authored token reference with more than one
thousand lines of prose and markdown tables. It duplicated values already
declared in `packages/ui/src/tokens.css` and repeated site extensions from
`packages/site/app/global.css`.

That made the document look authoritative while it was actually another drift
surface. The stale copy included obsolete token names (`--dur`, `--ease`,
`--highlight`), deleted eyebrow tiers, an old owner-palette path, conflicting
radii and surface ramps, and long prompt-guide examples with hardcoded color
values.

## Root cause

The design system had two human-maintained normative sources for the same data:
CSS tokens and markdown tables. Once token names moved to Tailwind v4 canonical
namespaces, the markdown was not mechanically tied to the CSS source. Every
manual update required remembering all tables, examples, skill instructions,
and agent prompt snippets that mentioned the old names.

## Rule

> `packages/ui/src/tokens.css` and `packages/site/app/global.css` are the token
> inputs. `DESIGN.md` is generated token data plus stable rationale. Do not
> hand-edit generated frontmatter or `<!-- BEGIN:tokens:* -->` regions.

If a token value, token name, or site clamp changes, update the CSS source and
run `make codegen`. `make codegen-check` must fail when `DESIGN.md` is stale.

## Operationalization

- `scripts/sync-design-md.mjs` parses the runtime `@theme` block and the site
  `@theme inline` block, then emits YAML frontmatter and generated markdown
  tables inside marker regions.
- `DESIGN.md` keeps human-authored sections for rationale, semantic component
  contracts, anti-patterns, site profile guidance, and references. It no longer
  owns token values by hand.
- `.agents/skills/agh/agh-design/SKILL.md` stays short and points agents at
  `tokens.css`, `DESIGN.md`, component recipes, and `COPY.md` instead of
  duplicating the full design spec.
- `magefile.go` wires `SyncDesignMD` into `Codegen` and `SyncDesignMDCheck`
  into `CodegenCheck`, so generated-token drift is part of the normal monorepo
  gate.
- The generator's `--audit-site` mode reports known site-token drift without
  mutating `packages/site/app/global.css`, keeping audit and remediation
  separate.

## Anti-patterns

- Hand-editing generated token tables in `DESIGN.md` to "just fix the docs."
  The next generator run will overwrite it, and the real source remains wrong.
- Adding a token to `tokens.css` without running `make codegen` and
  `make codegen-check`.
- Reintroducing long prompt-guide examples with raw hex, px recipes, or token
  aliases. Examples drift faster than component recipes; prefer Storybook or
  production components as the source.
- Duplicating anti-pattern lists into skills. Skills should route agents to the
  canonical spec and only keep the top-of-mind invariants needed before work
  starts.

## Implementation references

- `scripts/sync-design-md.mjs` — token parser, frontmatter emitter, markdown
  marker updater, and site audit.
- `DESIGN.md` — generated token frontmatter and marker regions plus stable
  rationale.
- `.agents/skills/agh/agh-design/SKILL.md` — slim design-skill dispatch contract.
- `magefile.go` — `SyncDesignMD` and `SyncDesignMDCheck` codegen wiring.
- `docs/_memory/lessons/L-022-eyebrow-canonical-source.md` and
  `docs/_memory/lessons/L-023-token-utility-canonical-form.md` — prior design
  drift lessons that this generator makes harder to regress.
