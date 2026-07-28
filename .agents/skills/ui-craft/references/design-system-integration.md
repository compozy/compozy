# Design System Integration

Read this when reading or extending DESIGN.md, working with semantic tokens, structuring shadcn/Radix composition, or writing rules-files for AI codegen. This reference covers how to keep AI-generated UI compatible with the design system rather than drifting away from it.

## DESIGN.md: The Project Source of Truth

`DESIGN.md` is a Google-Labs-originated convention: a root-level markdown file that captures a project's visual identity as durable, agent-readable context. The same role for design that `CLAUDE.md` plays for code conventions.

A canonical DESIGN.md contains, in order:

1. **Visual Theme & Atmosphere** — brand personality, target audience, emotional tone (the *why* before tokens)
2. **Color Palette & Roles** — primary, secondary, tertiary, neutral, plus semantic (success / warning / error). Each entry: hex + role + usage rule.
3. **Typography** — 9–15 levels (display / headline / body / label / caption × s / m / l) with role per level
4. **Spacing & Layout** — 4/8/12/16/24/32/48 scale, container widths, breakpoints
5. **Components** — `button-primary`, `card`, `input` etc. as `map<component, map<token, value>>` referencing earlier tokens
6. **Iconography & Imagery** — style, weight, radius
7. **Motion** — durations, easings
8. **Accessibility** — contrast ratios, focus, target sizes (WCAG 2.2 AA baseline)
9. **Don'ts** — explicit anti-patterns ("no gradients on chrome", "color never alone for state")

The product brief (2–3 sentences) before tokens is what makes the rest legible — it anchors *why* the tokens encode what they do.

### Reading DESIGN.md

When a UI task arrives:

1. **Locate.** Check repo root for `DESIGN.md`. Also check `packages/ui/`, `apps/web/`, `docs/`, and `.ai/`. The first hit is authoritative.
2. **Read it in full.** Don't skim. Tokens, the brief, and the don'ts all matter.
3. **Treat it as normative.** Tokens are not suggestions. Don'ts are not preferences.
4. **Cross-check with token files.** When DESIGN.md and `tokens.css` (or `theme.ts`, etc.) drift, DESIGN.md wins — flag the divergence for resolution.

### Updating DESIGN.md

When a UI change introduces a new variant, token, or component pattern:

1. **Propose, don't rewrite.** Append a focused diff to DESIGN.md in the same change set as the code.
2. **Justify the addition.** One sentence on why the existing tokens didn't fit.
3. **Update all relevant token files** to keep parity (`tokens.css`, `theme.ts`, Figma styles).
4. **Surface the change** in the PR description or commit message — design-system changes are reviewed differently than feature work.

### When DESIGN.md doesn't exist

If the project has no DESIGN.md and the work cannot wait:

- Fall back to `references/visual-craft.md` defaults
- Surface this as a finding in the deliverable: "Project has no DESIGN.md — recommend scaffolding before further UI work"
- Do not bootstrap a DESIGN.md as a side effect of a UI ticket — defer authoring to a dedicated DESIGN.md workflow or skill
- If forced to scaffold quickly, use the 9-section structure above as the template

## Token Discipline

Tokens are the contract between design and code. AI codegen breaks this contract by default. Discipline is required.

### Semantic > Primitive

**Primitive token** = raw value with a generic name:
```
--gray-200: #e5e7eb;
--blue-500: #3b82f6;
```

**Semantic token** = primitive wrapped in a purposeful name:
```
--color-border-default: var(--gray-200);
--color-action-primary: var(--blue-500);
```

Components reference **semantic tokens only**. Why:

- Dark mode flips semantic, primitives stay the same
- Brand recolor changes primitives, semantic vocabulary stays
- The semantic name documents the intent — code becomes self-explanatory
- LLMs are more likely to use a token correctly when the name describes the purpose

### Token Naming Convention

Patterns that hold up:

```
--<property>-<role>-<modifier>
```

Examples:
- `--color-bg-default`, `--color-bg-elevated`, `--color-bg-subtle`
- `--color-text-primary`, `--color-text-secondary`, `--color-text-disabled`
- `--color-border-default`, `--color-border-focus`, `--color-border-danger`
- `--color-action-primary`, `--color-action-primary-hover`, `--color-action-primary-pressed`
- `--space-1` through `--space-16` (numeric scale)
- `--radius-sm`, `--radius-md`, `--radius-lg`, `--radius-full`
- `--shadow-1` through `--shadow-5`
- `--duration-fast`, `--duration-medium`, `--duration-slow`

Avoid:
- `--color-1`, `--color-2` (no intent)
- `--button-primary-color` (component-specific token — only valid when the token is genuinely unique to that component)
- Mixing camelCase and kebab-case in the same system

### Component Tokens

When a component is complex (button has many states), it can define its own token layer that references semantic tokens:

```
--button-primary-bg:        var(--color-action-primary);
--button-primary-bg-hover:  var(--color-action-primary-hover);
--button-primary-bg-active: var(--color-action-primary-pressed);
--button-primary-text:      var(--color-text-on-action-primary);
```

This three-layer architecture (primitive → semantic → component) is where mature design systems land. Smaller projects can collapse to two layers.

### Drift Detection

Drift creeps in. Audit regularly:

- **Grep raw hex codes** in component code: `grep -rE "#[0-9a-fA-F]{3,8}"` — should return zero hits in components
- **Grep magic numbers** in spacing-related properties: any value not in the scale
- **Compare DESIGN.md and tokens.css** — they should agree
- **Run color-contrast checker** on every documented color pair — values can be correct individually but fail in combination

## Figma Layer Hygiene (for AI codegen via MCP / Code Connect)

If Figma is part of the design pipeline and AI is generating code from Figma, layer hygiene determines whether the output is usable.

### Layer naming
- Layers describe function, not appearance
- `button-primary` not `Rectangle 47`
- `header-nav` not `Frame 12`
- Numbered Figma defaults (`Frame 12`, `Rectangle 47`) signal undisciplined files — flag for cleanup before codegen

### Layout
- Auto Layout by default; absolute positioning is the exception
- Each component is a Frame with Auto Layout, padding, and gap from the design system
- Constraints set correctly for responsive (`fill container`, `hug contents`, `fixed`)

### Component variants
- One Figma component per UI component; variants for states (`default`, `hover`, `pressed`, `disabled`, `loading`)
- Variant properties map cleanly to code props
- Boolean variants for toggleable features (`with-icon`, `selected`)

### Slots
- Use Figma instance properties for swappable content
- Map cleanly to React `children` or slot patterns
- Document intent in the component description

### Tokens in Figma
- Colors, typography, spacing, effects all defined as styles/variables
- Naming matches the code token names (1:1 mapping)
- Figma file uses tokens — no raw hex in the design itself

### Code Connect (Figma → code mapping)
- Each Figma component links to the code component
- Variant → prop mapping documented
- AI codegen reads these links to produce correct imports and prop usage

### Figma MCP
- Provides structured JSON of the design to the LLM
- Layer names, variants, tokens, spacing all become readable context
- Without MCP, AI codegen falls back to visual interpretation — error rate spikes

## shadcn/ui + Radix Composition

For projects using shadcn (or similar Radix-based ecosystems), the discipline is:

### Radix primitives = unstyled, accessible behavior

Radix provides the accessibility, keyboard handling, focus management, and ARIA semantics. Never re-implement these by hand.

```tsx
import * as Dialog from '@radix-ui/react-dialog'
```

### shadcn = pre-styled, project-copied components

Components are copied into the project (not installed as a dependency). Edit freely. Tailwind classes use design-system tokens via `tailwind.config.js`.

### Composition pattern

```tsx
// In your design system layer
export const Dialog = (props) => (
  <RadixDialog.Root {...props} />
)

export const DialogContent = (props) => (
  <RadixDialog.Content className="..." {...props} />
)
```

This layer enforces the design system. Application code consumes the styled exports only. Never consume Radix primitives directly from application code — that path drifts.

### When to extend

When a new component is needed:
1. Check if Radix has the primitive — use it
2. Check if shadcn has a template — copy and adapt
3. Add to the design system layer with token-aware styling
4. Document in DESIGN.md if it's a new pattern

### When to deviate

Some Radix defaults are not ideal for every product:
- Focus visible styling can be replaced
- Animations can be customized
- Default keyboard contracts can be extended (not reduced)

Document deviations in the design system layer code with a comment explaining why.

## Tailwind Discipline

Tailwind is fast and dangerous. AI codegen with Tailwind is especially prone to drift. Rules:

### Use tokens, not arbitrary values

- ✅ `text-primary`, `bg-surface`, `p-4`, `rounded-md`
- ❌ `text-[#3b82f6]`, `p-[11px]`, `rounded-[7px]`

Configure Tailwind to surface token vocabulary:

```js
// tailwind.config.js
theme: {
  extend: {
    colors: {
      primary: 'var(--color-text-primary)',
      surface: 'var(--color-bg-default)',
      // ...
    },
    spacing: {
      // Maps to your --space-* tokens
    }
  }
}
```

### Variant ordering

Adopt a convention (recommend: prettier-plugin-tailwindcss) so component classes stay readable:

```
<layout> <spacing> <typography> <color> <border> <effect> <state>
```

### Component classes via cva or tv

For variant-heavy components, use `class-variance-authority` (or `tailwind-variants`). Don't string-concatenate classes inline:

```tsx
const buttonVariants = cva('inline-flex items-center justify-center', {
  variants: {
    intent: { primary: '...', secondary: '...', danger: '...' },
    size: { sm: '...', md: '...', lg: '...' }
  }
})
```

This pattern is also more legible for AI codegen — variants and tokens are explicit.

### `@apply` is fine for hot paths

For reused class clusters that drift across files, extract with `@apply` in a CSS layer. Don't fight the tool.

## Rules Files for AI Codegen

Most AI codegen pipelines (Cursor, Claude Code, Lovable, Copilot Workspace) read rules files. Use them to enforce the design system.

### File locations
- `.cursorrules` (Cursor)
- `.ai/rules.md` (generic convention)
- `CLAUDE.md` at project root (Claude Code)
- `web/CLAUDE.md` for frontend-specific scope

### Content
Rules files for UI scope should include:

- Pointer to DESIGN.md as the design source of truth
- Token vocabulary (sample of the most important tokens)
- "Never invent values" directive
- Component composition pattern (Radix + shadcn + design-system layer)
- Tailwind discipline rules
- Accessibility floor reference
- Microcopy voice rules

Example skeleton:

```markdown
# Frontend Conventions

## Design Authority
- Read DESIGN.md before any UI work
- Tokens defined in packages/ui/src/tokens.css are normative
- Component patterns in packages/ui/src/components/ are canonical

## Forbidden
- Raw hex codes in components
- Magic numbers in spacing
- Removing focus indicators without replacement
- Direct Radix imports in app code (use packages/ui/ wrappers)

## Required
- All interactive elements have visible focus
- All form inputs have programmatic labels
- All async actions have loading states
- All destructive actions have confirmation
```

### Keeping rules files honest
- Reference real files in the project, not abstractions
- Update when the design system evolves
- Audit them periodically — stale rules teach the AI wrong patterns

## Code Connect (Figma → Code mapping)

When Figma + code are both load-bearing:

1. Each Figma component has a Code Connect entry pointing to its code counterpart
2. Variant → prop mappings are explicit (`variant="primary"` in Figma maps to `intent="primary"` prop)
3. AI codegen reads the Code Connect mapping when generating component usage
4. Without Code Connect, AI codegen falls back to visual interpretation — error rate spikes

Set up:
- Install `@figma/code-connect`
- Add `<Component>.figma.tsx` files mapping each component
- Publish to Figma so MCP can serve the mapping

## Auditing the System Itself

Periodically audit the design system, not just usage:

- **Token drift:** raw values appearing in components
- **Component drift:** new patterns invented inline instead of in the system
- **Rules-file drift:** AI codegen rules stale relative to actual conventions
- **DESIGN.md drift:** the brand brief no longer matches the rendered product
- **Figma drift:** Figma file using tokens that don't exist in code (or vice versa)

A design system that lies hurts more than no design system at all.

## Sources

- Google Labs — DESIGN.md spec (canonical)
- VoltAgent — awesome-design-md, awesome-claude-design (community patterns)
- TheNewStack — From Prompt to Production: AI-Generated Design Systems
- UXDesign.cc — "Dear LLM, here's how my design system works"
- Figma — Code Connect documentation, MCP server documentation
- shadcn/ui — Composition patterns and CLI conventions
- Radix UI — Headless primitive documentation and accessibility reference
- Tailwind CSS — Token theming conventions, class-variance-authority docs
