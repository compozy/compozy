---
name: agh-design
description: AGH visual-design authority for production UI, static artifacts, prototypes, and reviews. Use when creating or reviewing an AGH surface or changing tokens, typography, spacing, depth, icons, or motion. Do not use for capture-only verification; use agh-ui-screenshot.
---

# AGH Design

Use the canonical visual authorities before making AGH design decisions.

## Authority

1. `packages/ui/src/tokens.css`: canonical token source consumed by Tailwind v4.
2. `DESIGN.md`: rationale, generated token tables, anti-patterns, and semantic component contracts.
3. `packages/ui/src/index.ts`: the `@agh/ui` surface contract — the canonical primitive inventory.
4. `packages/ui/src/components/**/*.tsx`: canonical production recipes.
5. `COPY.md`: product voice, terms, and public claim rules.
6. `PRODUCT.md`: audience, register, design principles, and anti-references.

## Top-of-mind invariants

- Design for people running agent work, not an expert console — calm by
  default, deep on demand: the default read of a screen is plain-language and
  sparse; density, mono ids, and raw enums live on inspection surfaces.
- Dark mode only; warm-dark surface ramp; one `--color-accent` target per viewport.
- Signal color marks live state only, never taxonomy — rows and cards stay
  neutral at rest.
- Flat depth model; use `--shadow-overlay` for overlays and `--shadow-highlight` for active rims.
- Pull values from `--color-*`, `--text-*`, `--radius-*`, `--duration-*`, and `--shadow-*` tokens; do not hardcode production hex or one-off sizes.
- `<Eyebrow>` is the only uppercase label contract; do not inline typography tuples for labels.
- See `DESIGN.md` section 10 for the anti-pattern list and lint/test guardrails.

## Named visual contracts

When a task or spec names an OpenDesign artifact, mock, screenshot, or other
trusted visual reference, activate `agh-ui-screenshot` before implementation
and follow its Visual Contract Mode. Implement from the rendered reference,
not source inspection alone; an implementation-only screenshot never proves
parity. The reference is normative for visual language — layout, anatomy,
typography, tokens, motion — and lossy for everything else: content, data,
copy, brand marks, and which controls exist stay with runtime truth,
`COPY.md`, and the brand inventory, recorded as authorized differences.

## Static HTML artifacts

Inline or import actual values from `packages/ui/src/tokens.css`. Mirror the class
structure and component anatomy in `packages/ui/src/components` where possible.
Keep artifacts dark, flat, calm, and functional. Use literal CSS only to represent
exported token values; do not invent a parallel palette.

## Production code

Reuse gate — before authoring any component, map every generic UI need against
`packages/ui/src/index.ts` and import the primitive from `@agh/ui` instead of
re-implementing it. Redefining an exported name in `web/` or `packages/site/`
fails lint (`compozy-ui-reuse/no-shadow-ui-primitive`); genuinely
domain-specific variants take a domain-prefixed name. New generic primitives
land in `packages/ui` with story + test; domain composites in
`web/src/systems/<domain>/`.

Edit the owning surface: `web/`, `packages/ui/`, or `packages/site/`. Consume
CSS variables and bare Tailwind v4 token utilities. If `tokens.css` or
`packages/site/app/global.css` changes, run `make codegen` and then
`make codegen-check` so `DESIGN.md` stays synchronized.

## Completion Criterion

The change is complete when it uses the canonical tokens and primitive owner,
introduces no parallel palette or shadow primitive, respects the owning
surface's instructions, regenerated design artifacts have no drift when a
token source changed, and every named visual contract has a passing evidence
bundle with zero unresolved blocking divergence.

## Error Handling

- **`DESIGN.md` and runtime tokens disagree:** treat `packages/ui/src/tokens.css` as source, run codegen, and inspect the regenerated spec rather than editing generated regions.
- **No exported primitive fits:** decide whether the need is generic or domain-specific; add generic primitives to `packages/ui` and domain composites to the owning Web system.
- **A plausible mock implies unsupported runtime behavior:** remove the unsupported control or metric; daemon truth wins.
- **Runtime truth conflicts with a normative visual reference:** follow runtime truth, record the contract conflict, and reconcile or explicitly authorize the reference delta before claiming parity.
- **The reference shows placeholder art, demo data, or omits product content:** a prototype is lossy — keep the canonical owner (brand inventory, runtime truth, existing views), record the authorized difference, and never grow an `@agh/ui` brand primitive or delete product content to match a mock.
