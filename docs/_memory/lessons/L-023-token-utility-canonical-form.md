# L-023 — Design tokens belong in `@theme`, not in `:root` aliased through `@theme inline`

**Class:** Frontend / Design system / Build configuration
**Date discovered:** 2026-05-11 (token codemod sweep)
**Evidence sources:** `packages/ui/src/tokens.css` rewrite, `scripts/codemod/tokens-bare-utilities.mjs`,
2.222 callsite conversions across 303 files in `web/src/**` + `packages/ui/src/**`,
build-output verification (`web/dist/assets/index-*.css`),
prior `@tailwindcss/upgrade@latest` false-positive incident that corrupted Go
test strings and `w-[2px]` literals.

## Context

`packages/ui/src/tokens.css` originally shipped a two-layer pattern that looked
canonical but was a footgun in disguise:

```css
:root {
  --fg: #ececef;
  --row-hover: rgba(255, 255, 255, 0.022);
  --dur: 140ms;
}

@theme inline {
  --color-fg: var(--fg);
  /* --row-hover, --dur — silently omitted */
}
```

`@theme inline` generates utilities (`text-fg` works) but does NOT emit
`--color-fg` as a global CSS variable. The raw `--fg` in `:root` is the only
global var. Every new design token requires touching two layers; missing the
second layer silently degrades the utility surface.

The fallout was measurable. Across `web/src/**` and `packages/ui/src/**` we
counted **1,271 occurrences** of `<prefix>-(--<token>)` arbitrary-value
syntax — ~90 % targeting tokens that already had bare utilities, ~10 %
targeting tokens that had no utility because their `--color-*` / `--width-*`
alias was missing. The lint plugin's own `no-design-glaze-rgba` error message
even recommended `bg-(--row-hover)` arbitrary syntax as the canonical fallback,
training the team into the anti-pattern.

A `@tailwindcss/upgrade@latest` run was offered as the migration tool. It
detected almost none of the arbitrary-syntax conversions, but it **did**
value-match generic `w-[2px]` literals to a newly-introduced
`--spacing-pill-group-track-padding: 2px` token, replacing every unrelated
2 px width with `w-pill-group-track-padding`. It also corrupted strings inside
Go integration tests (`"start-1"` → `"inset-s-1"`) and prose inside `.md`
documentation (`bg-[rgba(255,255,255,0.07)]` → `bg-btn-default-hover` inside
narrative backticks).

## Operationalization

1. **One layer, one prefix.** Every design token that generates a utility
   lives directly inside `@theme { … }` with the canonical Tailwind v4
   namespace prefix (`--color-*`, `--text-*`, `--tracking-*`, `--leading-*`,
   `--radius-*`, `--duration-*`, `--ease-*`, `--shadow-*`, `--font-*`,
   `--font-weight-*`). Tailwind both generates the utility AND emits the
   token as a global CSS variable. Direct CSS uses `var(--color-fg)` /
   `var(--duration-base)` — short aliases (`--fg`, `--dur`, `--ease`) are
   removed.

2. **`:root` is only for non-utility tokens.** Vars consumed exclusively
   inside `backdrop-filter`, JS-resolved sizing, component-internal modal
   widths, PillGroup sizing/spacing — these go in `:root` and are referenced
   via arbitrary-value syntax (`w-(--width-modal-md)` inside Dialog,
   `min-h-(--height-pill-group-segment-md)` inside PillGroup). Keeping them
   out of `@theme` prevents third-party tooling from value-matching them
   against generic literals.

3. **Don't run `@tailwindcss/upgrade` on greenfield v4 projects.** The tool
   is built for v3 → v4 migrations. On an already-v4 codebase it produces
   false positives (value-matching, string corruption, prose mangling) that
   outweigh its sparse correct simplifications. The repo-owned codemod
   (`scripts/codemod/tokens-bare-utilities.mjs`) consults the live `@theme`
   declarations to build an OLD→NEW alias map, restricts replacement to
   `*.tsx` / `*.ts` under `web/src/**` and `packages/ui/src/**`, verifies
   the prefix matches the token's namespace, and respects an explicit
   runtime-var whitelist.

4. **Lint catches regressions.** `lint-plugins/compozy-design-system.mjs`
   ships `prefer-bare-token-utility`: any
   `<prefix>-(--<token>)` where `--color-<token>` (or the appropriate
   namespaced alias) exists in `@theme` is an error with the bare utility
   as the recommended fix. Runtime vars (`--anchor-*`, `--available-*`,
   `--accordion-*`, `--detail-inspector-*`, `--radix-*`, `--width-modal-*`,
   `--size-catalog-logo`, `--size-provider-logo-well`, `--size-pill-group-*`,
   `--height-pill-group-*`, `--space-pill-group-*`) are whitelisted.

## Anti-patterns

- **Adding `--color-foo: var(--foo)` in `@theme inline` while `--foo` lives
  in `:root`.** Two layers means two failure modes; the second-layer omission
  is what created the original gap.

- **Treating `bg-(--row-hover)` as a canonical surface utility.** It is the
  fallback when the token has no `@theme` entry. Now that every glaze token
  is in `@theme`, the bare utility (`bg-row-hover`, `bg-surface-glaze`, …)
  is canonical, and the arbitrary-value form is a regression.

- **Adding component-internal tokens (PillGroup segment heights, modal
  widths) to `@theme`.** They are not part of any cross-component scale.
  Exposing them invites third-party tooling to value-match unrelated
  literals against them — exactly the failure mode that replaced every
  generic `w-[2px]` with `w-pill-group-track-padding`.

- **Running `npx @tailwindcss/upgrade@latest` on this repo.** It is not a
  regression-safe codemod for a v4-native codebase; use the repo-owned
  script and the lint rule instead.

## Implementation references

- `packages/ui/src/tokens.css` — single `@theme` block, `:root` reserved
  for non-utility / component-internal vars.
- `scripts/codemod/tokens-bare-utilities.mjs` — codemod with dry-run /
  --write, alias map derived from live `@theme`.
- `lint-plugins/compozy-design-system.mjs::prefer-bare-token-utility` —
  guardrail rule with runtime-var whitelist.
- `DESIGN.md` §2.5 (Surface glaze utilities), §3a (Modal anatomy),
  §4 (Component anatomy).
- `web/src/styles.css`, `packages/site/app/global.css` — direct `var()`
  refs renamed to `--color-*` / `--duration-base` / `--ease-out`.
- `packages/ui/src/lib/owner-palette.ts` — resolves `--color-avatar-*`.
- `packages/ui/src/lib/utils.ts` — `customTwMerge` extended with every
  AGH `--text-*` token so `cn("text-fg", "text-detail-h1")` keeps both
  size + color rather than collapsing to one group.

## Generalization

Token systems should expose ONE name per concept. When a `:root` short
alias and an `@theme` namespaced alias coexist, you get drift in three
directions: callsites pick whichever was canonical when they were written;
new tokens land in only one layer; third-party tooling can match against
either, producing false positives that look like bug fixes. Pick the
namespace-prefixed canonical form, delete the short alias, codemod the
callsites, and let the lint rule keep the contract sealed.
