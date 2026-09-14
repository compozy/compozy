# AI-Slop Patterns

Use the matching pattern when reviewing an observed UI problem. Treat detection prompts and example fixes as hypotheses to inspect against the actual content, behavior, and design system. A familiar visual choice alone is not a defect.

Classify confirmed findings by user impact and the project's severity policy. Pattern names and counts do not establish release blockers. Consult `anti-defaults.md` for concrete design alternatives when a choice needs reconsideration; preserve accepted user and product decisions.

## 1. VisualSameness

**Symptom:** Every block on the page looks like every other block. Same card, same radius, same border, same shadow, same gray. The eye has nothing to grab onto. Hierarchy is communicated only by reading order.

**Why it happens:** LLMs default to safe, low-variance styling. Cards-everywhere is a learned pattern.

**Detection prompts:**
- Squint at the page. Can the primary action be located in < 1 second?
- Is there exactly one focal point per view, or three competing ones?
- Do secondary blocks look like primary blocks with slightly different content?

**Fix:**
- Establish weight hierarchy: size + spacing + contrast + position
- Demote secondary content to plain text or low-elevation surfaces
- Use cards only for entities that are independently actionable
- Lean on the type scale to create visual rank, not container chrome

**Example before:**
```
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│ [Card icon]  │ │ [Card icon]  │ │ [Card icon]  │
│ Heading      │ │ Heading      │ │ Heading      │
│ Subtitle...  │ │ Subtitle...  │ │ Subtitle...  │
│ [Button]     │ │ [Button]     │ │ [Button]     │
└──────────────┘ └──────────────┘ └──────────────┘
```
Three cards of equal weight, all asking for attention.

**Example after:**
```
PROJECTS

Acme Corp Migration                       Last run 2h ago    >
  Production deployment workflow

Internal Dashboard                        Last run 5d ago    >
  Staging only · 3 contributors
```
Plain rows with type hierarchy carrying the weight. The eye knows where to go.

## 2. WeakHierarchy

**Symptom:** The eye lands on the wrong element first. Decoration (icon, illustration, color block) outranks the primary action or content. Multiple actions look equally important.

**Why it happens:** LLMs treat "important" as "more styling." They add weight to everything, not just the primary thing.

**Detection prompts:**
- Show the surface to someone unfamiliar. What did their eye go to first?
- Could a screenshot be skimmed for the primary action without reading?
- Are there two filled buttons of similar size on the same surface?

**Fix:**
- One primary action per view. Visually dominant — size + color + position.
- Secondary actions: outlined or ghost — visible but quieter
- Tertiary: text links or icon-only with tooltip
- Demote decoration: neutral color, smaller scale, off the visual centerline

## 3. TextOverflow

**Symptom:** Long strings clip, wrap into 4-line cells, push siblings off-grid, or break responsive layouts. Long URLs and resource names are the common offenders.

**Why it happens:** LLMs test with short demo content. Reality has 47-character org names and German translations 35% longer than English.

**Detection prompts:**
- Test with the longest realistic string for each field (org names, project titles, breadcrumbs)
- Test with non-Latin scripts (CJK widths, RTL scripts)
- Test localized strings — German, French, Portuguese, Russian
- Resize the viewport to 320px width

**Fix:**
- Truncate with `text-overflow: ellipsis` + tooltip showing the full string
- Wrap explicitly when truncation loses meaning (multi-line breadcrumbs, addresses)
- Use `min-width: 0` on flex children that should truncate (CSS gotcha)
- Reserve space — don't let one field push others

## 4. FakeInteractivity

**Symptom:** Something looks clickable but isn't (or the opposite). No hover/focus/active states on actual buttons. Cursor stays `default` over an interactive surface. Card looks pressable but only the title is a link.

**Why it happens:** LLMs sometimes generate styling without behavior, or behavior without styling. Either creates a usability contract failure.

**Detection prompts:**
- Tab through the surface. Every visible interactive element should receive focus.
- Move the mouse over each element. Cursor should match expectation (`pointer` for clickable).
- Click in five random spots that "look clickable." Do they all work or is some pure decoration?

**Fix:**
- Match cursor + role + state styling to actual behavior
- Use `<button>` and `<a href>` for real interactivity, never `<div onClick>`
- Add hover and focus-visible states to everything truly interactive
- Remove decorative affordances on non-interactive elements (the underline that suggests a link)
- Card-as-link: entire card clickable, with focus ring on Tab, single primary action inside

## 5. EmojiSpam

**Symptom:** Emojis as decoration. Mascots in headings. Emojis replacing the design system's icon set. 🎉 on success. 🚀 on a launch. ⚡ on speed.

**Why it happens:** LLMs lean on emojis to inject personality with zero design effort.

**Detection prompts:**
- Are emojis used in any product chrome (nav, buttons, headers, dialogs)?
- Are emojis being asked to do the work of icons?
- Is the same emoji used inconsistently across the product?

**Fix:**
- Remove emoji from product UI unless culturally explicit or user-generated
- Use the icon set instead — consistent stroke, sizing, alignment
- Personality comes from copy, layout, and color — not from 🎉
- Marketing surfaces have more latitude, but the same emoji should appear consistently or not at all

## 6. GradientCrutch

**Symptom:** Gradient backgrounds carrying the entire visual identity. Hero with a purple-to-pink gradient. Buttons with gradients. Card backgrounds with gradients. Every primary action gradiated.

**Why it happens:** Gradients are visually "alive" and cheap to apply. LLMs reach for them when asked to make something "modern" or "vibrant."

**Detection prompts:**
- Count the gradients on the page. More than two is a smell.
- Remove all gradients mentally. Does the surface still feel like itself?
- Are gradients on chrome (buttons, cards) or on brand moments (hero, marketing)?

**Fix:**
- Lean on type + spacing + a restrained palette for product chrome
- Reserve gradients for brand moments — hero blocks, splash screens, marketing
- One gradient per surface, never two competing
- A solid color often serves better than a gradient that distracts

## 7. GlassmorphismAbuse

**Symptom:** `backdrop-filter: blur(...)` on every elevated surface. Sidebars are translucent. Modals are translucent. Headers are translucent. Text legibility damaged on busy backgrounds.

**Why it happens:** Glassmorphism became fashionable around 2021 and LLMs absorbed it as default "modern" styling.

**Detection prompts:**
- Count blurred surfaces. More than one is a smell.
- Is text on top of blur consistently readable? Test against a busy background.
- Does the blur communicate anything (focus, depth) or is it decoration?

**Fix:**
- Use blur on at most one elevated surface (the topmost modal or command palette)
- Keep text on solid backgrounds
- If blur is required, ensure the underlying tint provides ≥ 4.5:1 contrast for text
- Default to opaque elevated surfaces; reach for blur only with intent

## 8. GenericIllustration

**Symptom:** The same isometric 3D illustration in every empty state. People-pointing-at-laptops stock art. Generic clipart "vibes" disconnected from the product. Mascot characters with no relationship to the brand.

**Why it happens:** Empty states feel like they "need something" and LLMs (or designers using stock libraries) drop in placeholder art that adds nothing.

**Detection prompts:**
- Could this illustration appear in any product? Then it shouldn't appear here.
- Does the illustration explain the data category or just fill space?
- Is the illustration on-brand or stock?

**Fix:**
- Text-only empty states for product chrome are fine — better than generic art
- Branded symbols or wordmarks if visual anchor is needed
- Custom illustrations only when commissioned for the brand
- Empty state copy should do the explanatory work, not the illustration

## 9. DesignSystemDrift

**Symptom:** Raw hex codes in component code. One-off `border-radius: 7px`. `margin-top: 11px` that matches no token. New colors invented inline. New shadows defined per-component.

**Why it happens:** LLMs paste values they "remember" from training data rather than referencing the token system, especially when the token names are non-obvious.

**Detection prompts:**
- Grep for raw hex codes (`#[0-9a-fA-F]{3,8}`) in component code
- Grep for non-scale numeric values in spacing, sizes, radii
- Grep for `box-shadow:` declarations outside the elevation system
- Check whether new components are using the existing token vocabulary

**Fix:**
- Replace every raw value with a token reference
- If the value doesn't have a token and is genuinely needed, propose adding it to DESIGN.md / tokens.css
- Never inline-invent a hex or numeric value to skip the proposal step
- Lint or pre-commit hook can catch this — `stylelint` with `color-no-hex` and `declaration-property-value-disallowed-list`

## 10. StateMatrixHoles

**Symptom:** Loading state is missing or shows the same as default. Empty state is missing. Error state is a console.log. Disabled state is just `opacity: 0.5`. Selected state is invisible to keyboard users.

**Why it happens:** LLMs design for the happy path. Other states are added as afterthoughts when remembered at all.

**Detection prompts:**
- Open `assets/state-matrix.md` for the component. How many rows are filled?
- For each state, can you produce a screenshot or describe the visible result?
- What happens to the surface when the API is slow / fails / returns empty?

**Fix:**
- Design every state in the state matrix at the same time as the default
- Loading: skeleton or progress, not silence
- Empty: explain + offer the primary action
- Error: explain + offer recovery
- Disabled: ≥ 3:1 contrast + a way to know why it's disabled
- Selected: non-color signal (icon, weight, position) plus color

## 11. CenteredEverything

**Symptom:** All headings centered. All body text centered. Long paragraphs centered (ragged on both sides — painful to read). Center-aligned card grids on wide viewports with vast empty margins.

**Why it happens:** "Centering looks balanced" is a learned default for LLMs that lack layout judgment.

**Detection prompts:**
- Is body text (> 2 lines) center-aligned?
- Are card grids breathing on a wide viewport or centered with empty margins beside them?
- Are headings center-aligned without a center-aligned hero treatment surrounding them?

**Fix:**
- Default to left-align for text in LTR locales
- Reserve center for:
  - Short headlines (1–2 lines)
  - Hero blocks where the entire composition is centered
  - Dialog actions (Cancel | Primary)
  - Marketing CTAs
- Right-align only for numbers in tables, optional metadata in lists

## 12. RandomRadii

**Symptom:** `rounded-xl` on one card, `rounded-2xl` on a sibling, `rounded-md` on a third. Buttons with one radius, inputs with a different radius, chips with a third. Or worse: each surface has its own `border-radius: <weird px>`.

**Why it happens:** LLMs pick radius values from training data per-component instead of following the project's radius scale.

**Detection prompts:**
- Compare the radius on every rounded element. How many distinct values?
- Does the radius hierarchy align with elevation (smaller for chips, larger for overlays)?
- Are radii referenced via token (`--radius-md`) or hard-coded (`8px`)?

**Fix:**
- Pick a radius scale (typically 4 steps: chip, control, surface, overlay)
- Assign per role: chips/badges = `radius-sm`, controls = `radius-md`, surfaces = `radius-lg`, overlays = `radius-xl`
- Apply consistently across the product
- Pills (`radius-full`) for tags, avatars, switches — never for buttons unless brand demands

## 13. GhostFocus

**Symptom:** `outline: none` with no replacement. Focus ring styled identical to hover. Focus ring contrast ≤ 1.5:1 against the background. Focus ring removed on buttons "because it looked ugly."

**Why it happens:** Default browser focus styling is ugly, and `outline: none` is a common quick fix that LLMs propagate without restoring the indicator.

**Detection prompts:**
- Tab through the surface with the keyboard. Can you tell where focus is at every step?
- Is the focus indicator distinct from the hover indicator?
- Run color picker against the focus ring at every interactive element. ≥ 3:1?

**Fix:**
- Restore `:focus-visible` styling:
  ```css
  :focus-visible {
    outline: 2px solid var(--color-focus-ring);
    outline-offset: 2px;
  }
  ```
- Pick a high-contrast focus color that works on every background
- Verify ≥ 3:1 against the element and against the page background
- Never remove the focus indicator without a replacement

## 14. MagicNumbers

**Symptom:** `width: 327px`. `margin-top: 11px`. `gap: 13px`. `padding: 9px 17px`. Values that match no scale and have no rationale.

**Why it happens:** LLMs eyeball values from training data screenshots. The resulting layouts don't snap to the project's spacing rhythm.

**Detection prompts:**
- Grep for numeric values in styles. How many don't match the spacing/size scale?
- Are component dimensions powers of the base unit (4 or 8)?
- Could you rebuild the layout from the design system tokens alone?

**Fix:**
- Snap to the spacing scale (typically 4 / 8 / 12 / 16 / 24 / 32 / 48 / 64)
- For sizes, prefer fluid (`flex-grow`, `min-width`) or scale-based (`width: 240px` from a size token)
- For gaps, use the project's scale tokens where that contract applies
- Lint with `stylelint` rules: `declaration-property-value-allowed-list` for spacing-related properties

## Focused visual inspection

Choose the probes that can expose the changed behavior's failure: locate the primary action, navigate by keyboard, exercise supported loading/error states, try representative long content, resize to affected breakpoints, or inspect supported themes and reduced motion. Reuse observations already collected.

An inspection supplies evidence for those states only. Fix confirmed defects within authorization and disclose gaps; a short visual sweep does not establish complete release readiness. Do not add a fixed timer, state quota, or another approval stage.

## Sources

- TheNewStack — From Prompt to Production: AI-Generated Design Systems (failure modes catalog)
- UXDesign.cc — "Dear LLM, here's how my design system works" — discipline on tokens
- Medium — Frontend in the Age of AI (integration patterns + common pitfalls)
- arxiv.org — Patterns of AI-Generated UI failure (research)
