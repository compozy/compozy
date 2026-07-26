# L-022 — Eyebrow typography needs one canonical source

**Class:** Frontend / Design system
**Date discovered:** 2026-05-10 (DashboardCard inline-eyebrow audit while landing the redesign branch)
**Evidence sources:** Audit run ahead of the dashboard polish; the final consolidation collapsed
the prop matrix and the multi-utility CSS layer into a single Inter UC contract. Touched files:
`packages/ui/src/components/custom/eyebrow.tsx`, `packages/ui/src/tokens.css`, `DESIGN.md` §3 / §11,
`.agents/skills/agh/agh-design/SKILL.md`, `packages/ui/src/lib/utils.ts`,
`lint-plugins/compozy-design-system.mjs`, plus the cross-monorepo callsite sweep.

## Context

While reviewing why the `DashboardCard` "ACTIVE SESSIONS / DAEMON UPTIME / QUEUE DEPTH" labels
felt drifty, an audit surfaced **124 callsites** across the monorepo applying the JetBrains-Mono
uppercase eyebrow style. ~62 used the `<Eyebrow>` primitive and ~62 inlined the same idea by
hand. At least **five different tuples** were in active use:

- `text-[10.5px]` + `tracking-wider` (the original `<Eyebrow>` body)
- `text-eyebrow` (11 px) + `tracking-mono` (0.06em) (the token-aligned majority)
- `text-badge` (10 px) + `tracking-badge` (0.08em) (legacy chip tracking applied to eyebrows)
- `text-[11px]` + `tracking-mono` hard-coded inside chat/metric/wire-card primitives
- `text-[10px]` + `tracking-[0.08em]` arbitrary tuples in dialog labels

The drift was **triplicated in the spec layer too**: `DESIGN.md` table §3 line 147 said
`Inter | 10.5 px | 0.05em UC`, `tokens.css` declared `--tracking-mono: 0.06em`, and the
`agh-design` skill brief said `letter-spacing: 0.06em`. The visible eyebrow in the UI was
JetBrains Mono, not Inter — so the spec, the token, and the implementation all disagreed at
once.

`twMerge` made it worse: with the default class group config, `text-eyebrow` and `text-(--muted)`
collapsed into the same "text-color" group, so `cn("text-eyebrow", "text-(--muted)")` silently
dropped the size. Any consumer that relied on token-aligned size + token-aligned color saw the
size erased without warning.

The first fix added size + tone variants to `<Eyebrow>` and rewrote `<extendTailwindMerge>`. But
that fix preserved a multi-tier API (`case`, `family`, `tone`, `size`, `weight`) plus three CSS
utilities (`eyebrow`, `eyebrow-badge`, `eyebrow-micro`). With multiple variants live, callsites
still picked the "wrong" tier and the JetBrains-Mono contract leaked into surfaces that must use
the Inter UC contract. The design-system consolidation superseded the variant matrix with a single contract.

## Root cause

The eyebrow style had **no single source of truth**, and even after the first fix it still had
**too many variants**. Three artifacts held overlapping authority (spec table / CSS token /
component implementation), each authored at a different time, and the `<Eyebrow>` API was
expressive enough (case + family + tone + size + weight) that the wrong combination of props
still produced drift. Tier proliferation invited the same callsite-by-callsite re-authoring that
the primitive was supposed to prevent.

## Rule

> **One eyebrow primitive, one CSS utility, one Inter UC contract.** Every uppercase label
> across `web/`, `packages/site/`, and `packages/ui/`'s public consumer surface MUST render
> through `<Eyebrow>` (`@agh/ui`) — children + className only, no `case` / `family` / `tone` /
> `size` / `weight` props — or through the single `.eyebrow` utility on structural HTML
> (`<dt>`, `<label>`, `<th>`, breadcrumb wrappers). The canonical contract is **Inter UC
> 11 px / weight 600 (semibold) / letter-spacing -0.005em**, bound to `--text-eyebrow` and
> `--tracking-eyebrow` in `packages/ui/src/tokens.css`. Color is **not** baked into the
> contract: pass `text-(--muted)`, `text-(--subtle)`, `text-(--accent)`, or a signal token
> through `className` when a tone is needed.

Inlining `font-mono` + `uppercase` + a `text-*` + a `tracking-*` tuple in product `<span>`,
`<p>`, or `<div>` content is forbidden. The deleted `.eyebrow-badge` / `.eyebrow-micro` utility
classes are forbidden — `compozy-design-system/no-inline-eyebrow` flags them. Arbitrary values
like `text-[10.5px]` / `tracking-wider` are forbidden everywhere, including the design-system
implementation files.

## Operationalization

- `packages/ui/src/components/custom/eyebrow.tsx` is the single primitive. The render path is
  `<span data-slot="eyebrow" className={cn("eyebrow", className)} {...props}>{children}</span>`
  — no variant matrix. New visual variants do not exist; reach for a different primitive
  (`<Pill>`, `<MonoId>`, bare span) when the rendered shape isn't Inter UC 11/600/-0.005em.
- `packages/ui/src/tokens.css` declares one utility:
  ```css
  @utility eyebrow {
    @apply font-sans text-(length:--text-eyebrow) font-semibold uppercase;
    letter-spacing: var(--tracking-eyebrow);
  }
  ```
  `--text-eyebrow` resolves to `0.6875rem` (11 px); `--tracking-eyebrow` resolves to `-0.005em`.
  These tokens are part of the public token contract, but automated coverage must stay narrow:
  prefer the `no-inline-eyebrow` lint rule, `twMerge` behavior checks, and visual/story coverage.
  Do not create broad CSS-literal suites that duplicate `tokens.css`.
- `packages/ui/src/lib/utils.ts` extends `tailwind-merge` with the project's `font-size` group
  so `cn("text-eyebrow", "text-(--muted)")` no longer collapses the size into the color group.
  Any new `--text-*` token in `tokens.css` MUST be added to that group on the same change.
- `DESIGN.md` §3 holds the authoritative type ladder. The eyebrow row references the tokens by
  name. Drift between this row, `tokens.css`, the `agh-design` skill brief, and `<Eyebrow>` is
  treated as a code defect, not a documentation tweak.
- `lint-plugins/compozy-design-system.mjs::no-inline-eyebrow` rejects:
  - `font-mono` + `uppercase` tuples in JSX `className`.
  - `font-mono` + `uppercase` + arbitrary `text-[…]` / `tracking-[…]` tuples.
  - The literal tokens `eyebrow-badge` and `eyebrow-micro` (the deleted utilities).
    Exemptions: structural HTML primitives (`<dt>`, `<label>`, `<th>`, …), PascalCase components
    (typography passes through), test/story files, and the `packages/ui` design-system
    implementation surface.
- When auditing for drift, grep both `font-mono.*uppercase` and arbitrary
  `text-[Npx]` / `tracking-[Nem]` patterns AND the deleted utility names. The audit only tells
  the truth when all three forms are scanned.

## Anti-pattern

- Inlining `font-mono text-[10.5px] uppercase tracking-wider text-(--muted)` "just for one
  span" — every callsite that did this turned into a permanent drift point.
- Adding a new visual variant back into `<Eyebrow>` (a new `case`, a new `size`, a new `tone`).
  The primitive is intentionally prop-less now; size/tone variations live in the className the
  consumer passes (text-color utilities) or in a different primitive entirely.
- Adding `.eyebrow-foo` utilities to `tokens.css` to "carve out" a special size. The deleted
  `.eyebrow-badge` and `.eyebrow-micro` are the warning shot — re-introducing tier utilities
  reopens the drift surface that took two passes to close.
- Changing `--tracking-eyebrow` to "match the component" instead of fixing the component to
  match the token.
- Adding a new `--text-*` token to `tokens.css` without registering it in the
  `extendTailwindMerge` config — it will silently collide with `text-color`.
- Treating `DESIGN.md` and `tokens.css` as independent specs. They are two views of one
  contract; if they disagree, both are wrong until reconciled.

## Source

- `packages/ui/src/components/custom/eyebrow.tsx` — the single prop-less primitive
  (`children` + `className` only).
- `packages/ui/src/tokens.css` — `--text-eyebrow`, `--tracking-eyebrow`, and the single
  `@utility eyebrow` declaration.
- `packages/ui/src/lib/utils.ts` — `extendTailwindMerge` registration of project font-size
  tokens (kept around so future size tokens don't collide with text-color).
- `lint-plugins/compozy-design-system.mjs` — `no-inline-eyebrow` rule covering the inline tuple
  AND the deleted `eyebrow-badge` / `eyebrow-micro` utility-class literals.
- `DESIGN.md` §3 ("Type Ladder") and §11 ("Anti-patterns") — authoritative type ladder + Eyebrow
  rule + misuse register.
- `.agents/skills/agh/agh-design/SKILL.md` — brand brief reaffirming the single Inter UC contract.
- `web/CLAUDE.md` ("Critical Rules") and `packages/site/CLAUDE.md` ("Critical Rules") — surface
  guards for the rule.
