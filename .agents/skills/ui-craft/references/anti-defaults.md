# Anti-Defaults

Use this catalog when a design choice fails to express the product's purpose or creates a concrete usability problem. `ai-slop-patterns.md` covers broader failure modes. Familiar patterns are valid when they fit the audience, content, brand, and existing interface.

## How To Use This File

Inspect only choices relevant to the task. Explain the observed cost before recommending a change: lost hierarchy, poor legibility, misleading content, interruption, inaccessible interaction, or rendering cost. Use the project's severity policy for actual defects. Visual preferences alone do not block approval or merge, and several preferences do not add up to a defect score.

Reuse the accepted brief and design decisions. Infer routine choices from the interface; no scene interview, alternative-font exercise, or written override is required for an intentional choice. Preserve the user's direction and report only material consequences.

## Patterns to inspect

### 1. Emoji as icon

Use the product's icon inventory for consistent controls. Emoji may fit reactions, user content, or an intentional brand. Check recognition, rendering, and accessible names instead of treating every emoji as a blocker.

### 2. "Inter" as default font

Preserve the established typeface when it works. Change typography for a brand, readability, language-support, or rendering need; being common is not a defect.

### 3. Centred hero one-column

A centered introduction can suit a short message. Use asymmetric composition, a product view, or a different hierarchy when the content needs comparison, explanation, or stronger emphasis. Do not require a new composition solely to avoid a familiar pattern.

### 4. Placeholder names

Use representative fixtures that demonstrate the actual content shape and are clearly sample data where needed. Never present invented customers, metrics, endorsements, or transactions as real. Preserve intentionally anonymous or documented example identities.

### 5. 3-column equal-card pricing/feature grid

Equal cards suit comparable options. Use unequal emphasis when the product actually recommends one option or content has different importance. Let the number of meaningful options determine the layout.

### 6. Gradient text

Check the rendered text's least legible regions and supported contrast modes. Use a solid foreground when the gradient fails those checks; retain an intentional gradient when it remains readable.

### 7. Glassmorphism as default

Check whether revealing the backdrop helps the surface, whether text remains legible, and whether compositing meets device constraints. Use a solid reading plane where needed. For requested glass work, use the relevant `glassmorphism` guidance; a surface-count quota is not evidence of rendering cost.

### 8. Side-stripe colored border

Keep status stripes when they reinforce a clear hierarchy and consistent status vocabulary. Add text or another non-color cue when color alone carries meaning. Replace stripes only when they add noise or contradict the design system.

### 9. Hero-metric template

Lead with a metric when its source, period, comparison, and consequence support the message. Otherwise use the concrete product behavior. Do not invent a statistic to fill a visual slot.

### 10. Modal as first thought

Use a modal for a task that needs focused input or a consequential decision. Prefer inline editing or a side panel when the user needs to retain surrounding context. Preserve the existing interaction contract and verify focus, dismissal, and recovery.

### 11. Neon glow / oversaturated purple

Judge color by brand fit, contrast, state distinction, and visual competition. Purple, gradients, and glow can be intentional. Reduce saturation or decoration where it obscures the primary action or information.

### 12. Bounce / elastic easing in product

Motion should explain state or continuity without delaying interaction. Tune spring or easing behavior to the component, preserve reduced-motion support, and fix measured jank or distracting repetition.

### 13. Generic 3D / isometric illustration

Keep an illustration when it explains the product or fits the brand. Remove decorative filler when it competes with content; a screenshot, specific drawing, concise text, or empty space may communicate better.

### 14. "Welcome!" / "Hi there!" / "Let's get started!"

A greeting can fit onboarding or a personal surface. Remove it when it displaces the next useful action or repeats information. Match the established product voice.

### 15. Decorative skeleton

Reserve a useful approximation of the final layout while loading. Reuse the product's loading behavior, avoid flicker for short operations, and stop animation when work settles. Follow `performance.md` for a measured loading problem.

### 16. "Click here" / "Submit" / "OK"

Use labels that clarify the action and result. Conventional short labels are fine when context makes the outcome unambiguous. Verify the accessible name and avoid relying on pointer-specific instructions.

### 17. Always-visible label on every icon

Use visible labels where recognition or task importance benefits from them. Familiar icon-only controls can work with accessible names and appropriate focus/hover help. A tooltip does not replace a control's accessible name; touch users also need to understand the action.

## Sources and further detail

This catalog adapts design-review ideas from `taste-skill`, `impeccable`, `ui-ux-pro-max`, and Refactoring UI. They supply options for judgment, not repository release policy. Use the relevant sections of `component-patterns.md`, `accessibility-floor.md`, and `archetypes.md` for a specific component or design problem.
