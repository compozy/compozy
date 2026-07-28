---
name: ui-craft
description: >-
  Craft floor for user-facing UI that refuses AI slop. Use when designing or
  building a new visible surface (component, page, dashboard, form, dialog,
  empty/loading/error state), reviewing or refactoring an existing one, or
  shaping an AI/agent interface (chat, streaming, citations). Not for backend,
  infrastructure, CLI/TUI (use tui-design), or docs-only edits.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# UI Craft

A user interface is a contract with a real person. **Slop** is what breaks it: visual sameness, weak hierarchy, fake interactivity, half the states missing, raw hex outside the token system, "Welcome!" copy nobody asked for. AI-generated UI fails these same ways every run. This skill is the **floor** that stops it — every visible state explicit, every value from the design system, every interactive element reachable by keyboard, every word earning its place.

## Frame the surface before generating

Lock these before any pixel. They apply to every change — a one-line tweak, a new variant, a full page. "It's just a small change" is the most common slop excuse.

1. **Read the brand authorities** (normative — quote them, never re-invent): `DESIGN.md` (root, or `packages/ui/`, `apps/web/`, `docs/`, `.ai/`), token files (`tokens.css`, `theme.ts`, `tailwind.config.*`), frontend conventions (`web/CLAUDE.md`, `.cursorrules`), copy authority (`COPY.md`). None present → fall back to `references/visual-craft.md` defaults and record a finding: *"No DESIGN.md / token system; recommend scaffolding before further UI work."* Don't bootstrap DESIGN.md as a side effect.
2. **State the job** in one user-perspective sentence (*"The user opens this dialog to confirm irreversible deletion of a project"*). More than one job → split the surface.
3. **Write the scene sentence** (below).
4. **Pick the register** — Product or Brand (below).
5. **Declare the dials** — VISUAL_VARIANCE, MOTION_INTENSITY, INFORMATION_DENSITY (below).

### Scene sentence

Physical context that forces the visual answer: *who* uses this, *where*, under *what* ambient light, in *what* mood. A category ("observability dashboard", "marketing landing") points at a thousand average answers; a scene forces one.

- ✅ *"SRE glances at incident severity on a 27" monitor at 2am in a dim room — every element competes with a paged-alert sound."*

If the scene doesn't force the decision, add detail until it does. Most weak visual calls come from a missing scene. *(Adapted from `impeccable`, pbakaus/impeccable.)*

### Register: Product or Brand

Pick one; switching mid-surface is the source of half of all "this feels off" reactions. If undecidable, it is almost always **Product** — Brand is the exception.

| Register    | Lives in                                          | Color                                      | Type                              | Motion                                       |
| ----------- | ------------------------------------------------- | ------------------------------------------ | --------------------------------- | -------------------------------------------- |
| **Product** | App shells, dashboards, settings, forms, lists    | Restrained: tinted neutrals + 1 accent ≤10% | System fonts ok; fixed rem scale  | 150–250ms state feedback only; no decoration |
| **Brand**   | Landing pages, heroes, onboarding, marketing      | Committed: 1 saturated color carries 30–60% | Display + body pairing            | Full orchestration; entrance reveals welcome |

Recurring task by a known user → Product. First/deliberate impression or campaign → Brand. *(Adapted from `impeccable`.)*

### Tunable dials

Declare values before generating; pin them in the deliverable so review can re-anchor. Translate "more bold / calmer / tighter / looser" to a dial move — don't re-derive every choice.

| Dial                  | Default | Low (1–3)                          | High (8–10)                                    |
| --------------------- | ------- | ---------------------------------- | ---------------------------------------------- |
| `VISUAL_VARIANCE`     | 6       | System-aligned, closest familiar pattern | Breaks the category template (bento, asymmetric, branded color) |
| `MOTION_INTENSITY`    | 4       | 80–150ms utility feedback only     | Up to 500ms entrances, layered materials (blur/mask/clip-path) at ≥7 |
| `INFORMATION_DENSITY` | 5       | Spacious, marketing-leaning        | Dense/dashboard: tabular-nums, keyboard-first, tight spacing ramp |

*(Dial pattern adapted from `taste-skill`, Leonxlnx.)*

## The operating loop

Run every task through these steps in order; each ends on a checkable bar. The one-line bar is the floor — the linked file is the full contract, and **you must read it in full before generating or reviewing that dimension** (loaded just-in-time, never all at once). The inline bar is not a substitute for the file.

1. **Usability filter.** Run the surface through Nielsen's 10 heuristics + the load-bearing Laws of UX (Jakob, Hick, Miller, Fitts, Tesler); fix the one or two that most threaten *this* surface. → **Novel or audited surface: read `references/usability-foundations.md`.** Bar: no unjustified heuristic violation (visible system status, reversible exits, recognition over recall).
2. **Accessibility floor** (WCAG 2.2 AA, the floor not the ceiling). Bar: text contrast ≥4.5:1 (≥3:1 large / non-text / state indicators); `:focus-visible` ≥2px and ≥3:1 on every interactive element; full keyboard reach + operation; target ≥24×24 (≥44×44 touch); `prefers-reduced-motion` honored; color never the sole carrier of meaning. → **Any interactive widget: read `references/accessibility-floor.md`** for the ARIA patterns (dialog, combobox, menu, tabs, slider, listbox) and verification recipes. A floor item that can't be met is a blocker, not a trade-off.
3. **Visual craft — draw every value from the system.** Type from the scale, color from semantic tokens (`--color-bg-danger-subtle`, never raw hex), spacing snapped to the scale, radius/elevation/motion from their scales. → **No token system, or a scale needs extending: read `references/visual-craft.md`.** A real gap → propose a token (step 7), never inline-invent. → **Extending DESIGN.md / tokens / shadcn-Radix: read `references/design-system-integration.md`.**
4. **State matrix — every state, designed at once.** Fill `assets/state-matrix.md` for the component: default, hover, active, focus-visible, disabled, loading, empty, selected, error, success, plus domain states. → **Any button/input/dialog/nav/table/card/dropdown/tabs/popover/toast: read `references/component-patterns.md`.** A listed-but-undesigned state is `StateMatrixHoles`.
5. **Anti-slop scan.** Match the surface against the 14 patterns (scorecard below) and the literal blocklist. Then the **two-level slop test**: could a stranger guess the aesthetic from the *category alone* (first-order default), or from *category + your anti-references* (second-order — brutalist mono is still a default)? Both are trapped; escape by re-deriving from the scene. → **Auditing AI-generated UI: read `references/ai-slop-patterns.md`** (before/after + remediation) **and `references/anti-defaults.md`** (17 artifacts to refuse on sight). Every hit is stop-and-fix.
6. **Microcopy — every word earns its place.** Bar: CTAs are verb + object (`Delete project`, not `OK`); errors say what happened + why + how to recover, no blame; empty states explain what lives here + the one action; no AI vocabulary (`seamless`, `delve`, `elevate`, `unleash`), no empty greetings, no filler (`Please`, `In order to`). → **Any user-visible text: read `references/microcopy-quality.md`.**
7. **Verify, then document deltas.** Visually confirm at supported breakpoints; force each state and screenshot the non-trivial ones; keyboard-test the full interaction; contrast-check rendered values; toggle `prefers-reduced-motion`. New token/variant/pattern → append it to `DESIGN.md` **in the same change set** (silent drift otherwise). Run `assets/pre-ship-checklist.md` as the binary gate. → **Per-ARIA verification recipes in `references/accessibility-floor.md`; if CWV/fonts/images are in scope, read `references/performance.md`.** A surface that can't be rendered for verification is not "done" — say so.

Branch-specific pointers, loaded when the branch applies:
- **Animation / transitions:** `references/motion-patterns.md` (duration bands, easing curves, CSS/Framer/`@starting-style` code, reduced-motion templates).
- **Dark theme:** `references/dark-mode.md` (surface-lightness elevation, accent desaturation, dangerous pairs).
- **AI / agent surface** (chat, streaming, citations, approvals): `references/human-ai-ux.md` (Microsoft 18 + IBM 4 pillars).
- **"We need a [Linear/Notion]-like X" / reinventing a primitive:** `references/archetypes.md` (7 signature contracts).
- **Reviewing an existing surface:** deliver findings in the `assets/ui-audit-template.md` format.

## Reference router

Every row whose trigger applies is mandatory reading before generating that dimension, not an appendix.

| When                                                            | File                                    | Covers                                                              |
| --------------------------------------------------------------- | --------------------------------------- | ------------------------------------------------------------------ |
| Designing a novel surface / auditing usability                  | `references/usability-foundations.md`   | Nielsen 10, Laws of UX, progressive disclosure, mental models      |
| Any interactive widget (dialog, menu, combobox, tabs, slider)   | `references/accessibility-floor.md`     | WCAG 2.2 AA non-negotiables, ARIA patterns, verification recipes   |
| Buttons/inputs/dialogs/nav/tables/cards/dropdowns/tabs/toasts   | `references/component-patterns.md`      | States, keyboard contract, a11y, and slop per component            |
| Writing or reviewing user-visible text                          | `references/microcopy-quality.md`       | Banned vocabulary, per-surface tone, error/empty/CTA templates     |
| Auditing AI-generated UI                                        | `references/ai-slop-patterns.md`        | Before/after + remediation for the 14 patterns, detection prompts  |
| A choice feels like a category-average default                  | `references/anti-defaults.md`           | 17 literal artifacts to refuse on sight                            |
| Building an AI/agent surface                                    | `references/human-ai-ux.md`             | Microsoft 18 + IBM 4 pillars, streaming/citation/confidence UI     |
| Reading/extending DESIGN.md, tokens, shadcn/Radix, codegen rules| `references/design-system-integration.md`| DESIGN.md sections, token discipline, Figma/Code Connect          |
| No token system, or extending a scale                           | `references/visual-craft.md`            | Type/color/spacing/radius/elevation/motion defaults                |
| Adding or reviewing animation                                   | `references/motion-patterns.md`         | Duration bands, easing, CSS/Framer/`@starting-style`, reduced-motion|
| Implementing or reviewing dark mode                             | `references/dark-mode.md`               | Surface-lightness elevation, accent desaturation, dangerous pairs  |
| Unprofiled ship, CWV regress, heavy dep on a hot route          | `references/performance.md`             | CWV floors, 80ms threshold, font zero-CLS, pre-ship gate           |
| "We need a [Linear/Notion]-like X"                              | `references/archetypes.md`              | 7 archetypes, each a six-slot signature contract                   |

## Anti-slop scorecard

Match every surface. Hits are not nits; severity drives the response. Full symptom/fix per pattern lives in `references/ai-slop-patterns.md` (by number) and `references/anti-defaults.md`.

- **Critical** — blocks merge (accessibility, broken keyboard/focus contract, missing states).
- **Serious** — blocks review approval (training-data defaults, design-system drift, weak hierarchy).
- **Moderate** — polish later, *unless* ≥3 stack on one surface → treat as Serious.

| #  | Pattern             | Severity | | #  | Pattern            | Severity |
| -- | ------------------- | -------- |-| -- | ------------------ | -------- |
| 1  | VisualSameness      | Moderate | | 8  | GenericIllustration| Serious  |
| 2  | WeakHierarchy       | Serious  | | 9  | DesignSystemDrift  | Serious  |
| 3  | TextOverflow        | Serious  | | 10 | StateMatrixHoles   | Critical |
| 4  | FakeInteractivity   | Critical | | 11 | CenteredEverything | Moderate |
| 5  | EmojiSpam           | Moderate | | 12 | RandomRadii        | Moderate |
| 6  | GradientCrutch      | Serious  | | 13 | GhostFocus         | Critical |
| 7  | GlassmorphismAbuse  | Moderate | | 14 | MagicNumbers       | Moderate |

## Scripts

- `scripts/check-contrast.mjs` — WCAG AA/AAA + APCA Lc for a foreground/background pair; hex/rgb/oklch, zero-dependency. Wire into CI on token changes.
- `scripts/detect-token-drift.mjs` — scan source for raw hex/rgb/hsl/oklch outside token paths; reports `file:line` + suggestion. Must exit 0 to ship.
- `scripts/validate-metadata.py` — validate this skill's frontmatter against the agentskills.io spec (author tooling).

## Edge cases

- **Tokens conflict** between DESIGN.md and a token file → trust DESIGN.md; flag the divergence as a finding.
- **State matrix exceeds the surface** → prune to states the surface supports, justifying each pruning; never silently drop.
- **User insists on shipping a known hit** → name the anti-default, run the scene sentence with them; if no scene emerges, record the override as acknowledged slop debt. Never invent a justification for it.
