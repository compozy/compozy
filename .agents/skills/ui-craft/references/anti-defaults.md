# Anti-Defaults

Read this when the surface is heading toward a generic AI-trained default — when a request feels like it could be answered by 50 other products' average. `ai-slop-patterns.md` catalogues *modes of failure*. This file catalogues *literal artifacts* to refuse on sight.

Defaults are not neutral. They are votes for what the model has seen most.

## How To Use This File

1. **Match-and-refuse.** When you catch yourself reaching for an item below, stop. Ask: *what scene-sentence forced this choice?* If there is no scene, the choice is reflex, not intent.
2. **Severity decides the response.** Critical = block merge. Serious = block review approval. Moderate = nit unless used together with others (three moderates compound to serious).
3. **Defaults are allowed when justified.** "Inter" is a fine typeface. "Inter because it was the first font I thought of" is the problem. The artifact is not banned — the reflex is.
4. **Cite this file when refusing.** Saying "this is `anti-defaults.md` #5" is faster than re-arguing every time.

## The Blocklist

### 1. Emoji as icon

**Forms:** 🚀 in headings, ✨ in CTAs, 🎉 on success, ⚡ on speed claims, 🔥 in marketing copy, 📊 instead of a chart icon, ✅ instead of a checkmark.

**Severity:** Critical in product chrome (nav, buttons, dialogs, settings); Moderate in marketing or user-generated content; allowed when culturally explicit (a flag picker, a reaction picker).

**Why:** Emojis bypass the icon set. They render inconsistently across platforms (Apple, Google, Microsoft, Twemoji all differ). They communicate "I added decoration to feel friendly" — the opposite of craft.

**Refuse with:** Heroicons / Lucide / Phosphor / Tabler / Radix — pick one icon family and stick to it. Personality comes from copy, layout, and color.

### 2. "Inter" as default font

**Forms:** `font-family: "Inter"` without an alternative considered. `system-ui` rejected for "looking unprofessional." Designs that say "use Inter" with no scene context.

**Severity:** Moderate. Inter is a strong typeface; using it is fine. Using it because you did not consider anything else is the problem.

**Why:** Inter is the unofficial AI training default for "modern product UI." It is everywhere because it was the first widely available variable sans with strong screen readability. That does not make it the right answer for your brand.

**Refuse with:** Run the scene sentence. Then pick from: a system stack (fast, free, OS-native), Geist (Vercel), JetBrains Mono (for code), a brand-specific commission, or yes — Inter — *with intent*.

### 3. Centred hero one-column

**Forms:** Full-bleed hero with centred headline + centred subhead + single centred CTA. Three-tile feature row underneath. "Trusted by" logo wall. "Get started today" CTA repeat.

**Severity:** Serious on a serious product or B2B page; Moderate on side projects.

**Why:** This is the dominant SaaS landing-page template from ~2020–2024. Every YC batch ships it. Every AI-generated landing reaches for it first. It signals "I did not have a point of view."

**Refuse with:** Asymmetric hero (left-aligned text + right-aligned visual), bento layout for features, scene-sentence-driven composition. If the brief truly is "explain the product and ask for signup," that is fine — but make the *type* and *spacing* carry character. Center-alignment is not point of view; it is the absence of one.

### 4. Placeholder names

**Forms:** "John Doe", "Jane Smith", "Acme Inc.", "Lorem ipsum", "User Name", "user@example.com" in screenshots, demos, video frames, or product UI placeholder content.

**Severity:** Serious in any user-facing surface; Critical if shipped to production (real users see the placeholder).

**Why:** Realistic-but-fake content makes the product feel real. Lorem ipsum makes it feel like a wireframe. "John Doe" makes it feel like a tutorial.

**Refuse with:**
- Names: domain-realistic ("Maria Souza", "Tomás Andrade", "Priya Iyengar")
- Companies: plausible-sounding without being trademarked ("Tidepool Logistics", "Granite Robotics")
- Emails: real-shape addresses on owned domains ("maria@yourcompany.com")
- Copy: short, plausible product-domain sentences. Never lorem ipsum past the wireframe stage.

### 5. 3-column equal-card pricing/feature grid

**Forms:** "Basic / Pro / Enterprise" pricing cards. "Fast / Reliable / Secure" feature cards. Three icons in a row underneath any hero.

**Severity:** Moderate. The pattern works occasionally; it is the *reflex* that is the problem.

**Why:** Three equal cards visually communicate "we have three things of equal importance" — which is almost never true. Pricing tiers have a recommended one. Features have a primary. The visual treatment lies.

**Refuse with:** Asymmetric weights (recommended tier larger), a featured row plus secondary tile, or bento grid (see `archetypes.md`). For features: lead with the strongest, support with weaker — never lock to three.

### 6. Gradient text

**Forms:** `background-clip: text` on headlines. "Vibrant" gradients on h1. Animated gradient text on landing pages.

**Severity:** Serious. Contrast is unpredictable across the gradient.

**Why:** Part of the gradient passes contrast; part fails. You cannot verify accessibility for a single-color value because the value is a range. Reduced-color displays and high-contrast modes break it entirely.

**Refuse with:** Solid color. One color. Verified contrast. Brand expression goes into the *type*, *spacing*, and *layout* — not into a Photoshop filter on the most important word.

### 7. Glassmorphism as default

**Forms:** `backdrop-filter: blur(...)` on every elevated surface — sidebars, headers, cards, modals. Translucent everything.

**Severity:** Moderate when used on one surface; Serious when used on multiple competing surfaces.

**Why:** Blur is expensive (paint cost). Text on blur is hard to verify for contrast (depends on what is behind). The aesthetic peaked around 2021 and has since become AI-default-modern.

**Refuse with:** Solid surfaces with the surface-lightness elevation system (see `dark-mode.md`). Reserve blur for one surface — usually the topmost modal or command palette — and verify text contrast against worst-case backgrounds.

### 8. Side-stripe colored border

**Forms:** Cards with a 3–8px coloured left or top border indicating category. "Status indicator" stripes on alerts. Dashboard-cliché 2020.

**Severity:** Moderate.

**Why:** The pattern is not wrong — it is overused. Every observability dashboard, every error message, every CRM card reaches for it. It substitutes for actual visual hierarchy.

**Refuse with:** Background tint (subtle, accessible), icon + label, or both. A `5px solid var(--color-warning)` strip should be the last choice, not the first.

### 9. Hero-metric template

**Forms:** Big number with gradient background. "10x faster." "100M+ users." Marketing hero that is one giant stat over a gradient.

**Severity:** Serious in serious B2B contexts; Moderate in consumer.

**Why:** The pattern is exhausted. Big number + gradient = "I am pitching to investors" = "I do not have product narrative." A real number with a real source and a real consequence is more compelling than 144pt purple gradient.

**Refuse with:** Smaller number + supporting prose + cited source. Or skip the metric entirely and lead with a product demo.

### 10. Modal as first thought

**Forms:** Every interaction opens a modal. Edit a name → modal. Delete a row → modal. Filter a list → modal. Confirm anything → modal.

**Severity:** Serious. Modals fracture the user's task into a sequence of context switches.

**Why:** Modals interrupt. They cover content. They demand attention. They are the right answer when the action is destructive, irreversible, or requires substantial input. They are the wrong answer for most edits.

**Refuse with:** Inline editor (see `archetypes.md` #7), expandable row, side panel, contextual menu. Reserve modals for: confirm-destructive, structured-multi-input, or context-shifting flows where the user genuinely needs to focus.

### 11. Neon glow / oversaturated purple

**Forms:** `box-shadow: 0 0 40px var(--purple-500)` on a hero element. Oversaturated `#8B5CF6` / `#A855F7` / `#7C3AED` accents everywhere. Purple-to-pink gradients.

**Severity:** Moderate in product UI; Serious on a serious brand.

**Why:** Purple-to-pink is the AI-trained default for "tech / AI / modern." The Midjourney aesthetic, Replit's first chrome, every "AI startup" cookie-cutter. It became visual hygiene to avoid.

**Refuse with:** Run the scene sentence. Pick from the full hue wheel — green, amber, blue, red, teal, orange. Make the brand earn its colour. If purple is correct for your brand, make it earn the choice; do not arrive at it by default.

### 12. Bounce / elastic easing in product

**Forms:** `cubic-bezier(0.34, 1.56, 0.64, 1)` on buttons, dialog entries, navigation transitions. Overshoot on accordions. Spring physics on tabs.

**Severity:** Serious in product chrome; Moderate as a one-off brand moment.

**Why:** Bounce reads as toy-like and dated. It tells the user the interface is performing for them, not serving them. Product tasks want decisive motion.

**Refuse with:** `ease-out-quart` / `ease-out-quint` for entries; `ease-in-expo` for exits. Reserve spring physics for one-off delight moments — a confetti, a celebration — never for chrome (see `motion-patterns.md`).

### 13. Generic 3D / isometric illustration

**Forms:** Storyset / undraw / icons8 hero illustrations. Isometric "people pointing at laptops." 3D bevelled cloud + chart + checkmark composition. Same illustration that appears on 500 other products.

**Severity:** Serious in product surfaces; Moderate on side projects.

**Why:** Generic illustration adds nothing to the brand and clearly signals "I needed something to fill this space." See `ai-slop-patterns.md` #8 (`GenericIllustration`).

**Refuse with:** Text-only empty states. Branded wordmark or symbol. Custom illustration commissioned for the brand. A single product screenshot in context. Empty space — empty space is fine.

### 14. "Welcome!" / "Hi there!" / "Let's get started!"

**Forms:** Empty greeting at the top of dashboards. "Hi {firstName}!" cards. "Welcome back!" toasts. Decorative greetings.

**Severity:** Serious. Empty greetings waste attention and add no information.

**Why:** Greetings are conversational reflex from chatbots. In a dashboard, the user is not arriving — they are working. The greeting takes a row of vertical space and gives nothing back.

**Refuse with:** Lead with information. "3 workflows failed overnight." "12 new audit events." "Deployment to production is in progress." The dashboard greets through utility, not pleasantries.

### 15. Decorative skeleton

**Forms:** Skeleton screens that do not match the final content shape. Generic gray-bar placeholders unrelated to what loads. Animated shimmer that runs forever even when nothing is loading.

**Severity:** Moderate (CLS risk) to Serious (when shimmer never stops).

**Why:** A skeleton's job is to reserve the exact final dimensions. Wrong-size skeleton = layout shift when content arrives = CLS regression = real UX harm.

**Refuse with:** Skeletons drawn to the exact final shape (same row heights, same column widths). Stop the shimmer when content is ready. Skip the skeleton if the operation finishes under 100ms (see `performance.md`).

### 16. "Click here" / "Submit" / "OK"

**Forms:** CTAs with no verb-object. Generic "Submit" / "OK" / "Click here" buttons. "Learn more" with no specifics.

**Severity:** Serious. Generic CTAs are a microcopy failure (see `microcopy-quality.md`) and a usability failure (the user does not know what will happen).

**Why:** A button label is a promise about what happens when pressed. "OK" promises nothing. "Submit" promises nothing. "Click here" actively wastes the button.

**Refuse with:** Verb + object. `Delete project` not `Delete`. `Save changes` not `OK`. `Read deployment guide` not `Learn more`.

### 17. Always-visible label on every icon

**Forms:** Every icon paired with a permanent text label, even when the icon is universally recognized (close, menu, search). Sidebars where every item shows both icon + full label even in collapsed mode.

**Severity:** Moderate. The opposite is worse — icon-only nav that requires learning — but redundancy is also a failure.

**Why:** A universally-recognized icon (close X, hamburger ≡, search 🔍) does not need a label in chrome. A specialty icon does. Always-on labels turn iconography into decoration.

**Refuse with:** Tooltips on hover for icon-only chrome. Labels on first-use or in expanded mode. Reserve permanent labels for non-obvious icons (export, archive, sync — all icon families disagree on these).

## When the User Insists

The user asks for a banned default. They have authority over their own product. Your job:

1. Name the anti-default. "This is the centred-hero default from `anti-defaults.md` #3."
2. Run the scene sentence with them. If a real scene emerges, the choice is no longer default — it is intent. Proceed.
3. If no scene emerges, document the override in the deliverable: "Shipping centred hero per explicit request. Slop debt acknowledged."
4. Never pretend the default is correct. Never invent justifications. Refuse silently is dishonest; refuse aloud is craft.

## Compounding

Three Moderates compound to Serious. Three Serious compound to Critical-by-pattern. A surface that has:

- "Inter" default
- 3-column feature grid
- Centred hero
- Purple gradient
- Lorem ipsum
- Stock isometric illustration

…is not six small failures. It is one big failure: the product has no point of view. Refuse on sight.

## Strong Opinion

> Defaults are not neutral. They are votes for what the model has seen most. Use a default with intent, or you have decided nothing.

## Sources

- `taste-skill` (Leonxlnx, open-design ecosystem) — explicit anti-slop literal blocklist pattern
- `impeccable` (pbakaus/impeccable) — reflex-reject lists and scene-driven decision framework
- `ui-ux-pro-max` (open-design ecosystem) — pre-delivery anti-pattern checklist
- `ai-slop-patterns.md` (this skill) — categorized failure modes; this file is the literal-artifact companion
- Refactoring UI — *Designing With Color*, *Typography*, *Visual Hierarchy*
