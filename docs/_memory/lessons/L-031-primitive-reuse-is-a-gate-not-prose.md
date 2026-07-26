# L-031 — Primitive reuse is a gate, not prose

**Class:** Frontend / Design system / Process
**Date discovered:** 2026-07-10 (operator report "components keep getting recreated instead of reusing `@agh/ui`" → reuse audit on the `model-selector` worktree)
**Evidence sources:** First repo-wide run of `compozy-ui-reuse/no-shadow-ui-primitive` found **17 shadow definitions**; discovery audit of the doc/skill hot path; remediation sweep across `web/src` and `packages/site`. Touched files: `lint-plugins/ui-primitive-reuse.mjs`, `.oxlintrc.json`, `packages/ui/CLAUDE.md`, root `CLAUDE.md` (Design System), `DESIGN.md` §9, `.agents/skills/agh/agh-design/SKILL.md`, plus 25+ consumer files.

## Context

`@agh/ui` already had everything the docs said it needed: a complete hand-written primitive
inventory (`packages/ui/README.md`), semantic contracts in `DESIGN.md` §9, and two design skills
(`agh-design`, `ui-craft`) that were mandatory for all UI work. Duplication happened anyway. The
audit found why:

- The inventory was an **island** — zero references from root `CLAUDE.md`, `web/CLAUDE.md`,
  `DESIGN.md`, or either design skill. Nothing in the hot path told an agent it existed.
- `web/CLAUDE.md`'s structure map still documented a `components/ui/` shadcn directory that had
  been deleted — actively steering agents toward creating local primitives.
- `agh-design` told agents to "mirror the class structure and component anatomy in
  `packages/ui/src/components`" — correct for static HTML artifacts, but read as
  copy-not-import guidance for production code.
- Nothing failed when a duplicate landed. Every other design-system invariant (tokens, eyebrow,
  glaze, type tuples) had a deterministic gate; component reuse had only prose.

First enforcement run found 17 real shadows: `ActionResultBanner` **×4** (settings/providers,
vault, hooks-extensions, sandbox — the 4th copy was missed by a regex scan and only surfaced by
the AST rule), `OperationalLinksRow` ×3, local `Section`, `FieldLabel`, `Timeline`,
`ToolCallRow`, and 6 site components (`Avatar`, `CodeBlock` ×2, `KindChip`, `WireCard`,
`SidebarSectionLabel`). Several were the **inverse** failure: composites promoted into `@agh/ui`
as dormant code whose original callsites were never migrated — promotion without convergence
produces the same drift as recreation.

This is the L-022 arc repeated one level up: eyebrow typography also had docs, a primitive, and
an anti-pattern list, and only stopped regressing when `no-inline-eyebrow` became a lint error.

## Root cause

Reuse depended on **discovery** (inventory off the hot path), **recall under context pressure**
(skills carried no reuse step at authoring time), and **nothing binding at merge time**. Prose
rules decay in exactly the situations that produce duplication — a deep-in-task agent writing a
"small local helper". The one design-system invariant without a deterministic gate was the one
that kept regressing.

## Rule

> Consumer surfaces (`web/src/**`, `packages/site/**`) never define a function, class, or
> component-initialized const whose name `@agh/ui` exports. Import the primitive; a genuinely
> domain-distinct variant takes a domain-prefixed name (`SessionToolCallRow`, `MessageTimeline`,
> `BlogCodeBlock`, `DocsSidebarSectionLabel`). New generic primitives land in `packages/ui`
> (story + test in the same PR); silent shadowing is the only forbidden state. Enforced at
> `error` by `compozy-ui-reuse/no-shadow-ui-primitive`, which parses the surface contract
> `packages/ui/src/index.ts` — the inventory that cannot drift because it is the source.

## Operationalization

- `lint-plugins/ui-primitive-reuse.mjs` derives the banned set from `packages/ui/src/index.ts`
  at lint time (PascalCase value exports only; types, hooks, helpers, ALL_CAPS constants
  excluded). It throws loudly if the index moves — never weaken to a silent empty set.
- Promotion protocol: when a domain composite is promoted into `@agh/ui`, the same change must
  migrate or domain-rename every existing callsite. Dormant promoted code + live local copies is
  a lint failure from day one.
- Shadow triage is a two-branch decision: **compose/import** when the primitive covers the need;
  **rename with a domain prefix** when the component is genuinely domain-specific. Both are
  valid; keeping the colliding name is not.
- Hot-path docs point at the source, not at a copy: root `CLAUDE.md` Design System rule,
  `packages/ui/CLAUDE.md` (the contributor guide that replaced the README), `DESIGN.md` §9
  pointer, and the `agh-design` reuse gate all name `packages/ui/src/index.ts` as the inventory.

## Anti-pattern

- Answering "agents keep duplicating components" with more prose ("check the inventory first")
  in yet another doc. The agents that duplicated already had two mandatory skills loaded.
- Maintaining a hand-written inventory table as the canonical catalog — it drifts, and a drifted
  inventory teaches agents to distrust it. Point at `src/index.ts`; generated views are the only
  acceptable copies (L-024).
- Promoting a primitive into `@agh/ui` without migrating its origin callsites in the same change.
- Auditing duplication with regex over `function X(`/`const X =` — the 4th `ActionResultBanner`
  proved only AST-level matching tells the truth.

## Source

- `lint-plugins/ui-primitive-reuse.mjs` + `lint-plugins/__tests__/ui-primitive-reuse.test.mjs` —
  the rule and its oxlint end-to-end proof.
- `.oxlintrc.json` — `jsPlugins` registration + `compozy-ui-reuse/no-shadow-ui-primitive: error`.
- `packages/ui/CLAUDE.md` — package contributor contract (README.md deleted in the same change).
- Root `CLAUDE.md` Design System critical rule; `DESIGN.md` §9 inventory pointer;
  `.agents/skills/agh/agh-design/SKILL.md` production reuse gate.
- [L-022](L-022-eyebrow-canonical-source.md) — the precedent arc: canonical source + lint gate is
  what ends design-system drift, not documentation volume.
