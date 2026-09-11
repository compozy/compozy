# Craft reference

Use the sections relevant to the surface being changed. The project's design system, existing components, approved references, and user direction own visual choices. Numeric style ranges below are starting points, not reasons to replace an intentional design. Accessibility and truthful behavior still apply.

## Readable hierarchy

- Establish one dominant entry point per visual region, then supporting structure and incidental metadata. A long page can establish another primary after a deliberate section break. Competing titles, badges, and buttons flatten priority.
- Combine at least two of scale, weight, surrounding space, tracking, and alignment to establish emphasis. A 1.25× size step is a useful starting point; smaller steps need a clearer weight or spacing change. Demote competitors before enlarging everything.
- Use proximity to express ownership: a heading belongs closer to its content than to the previous section. A section gap around 1.5× an internal gap can distinguish groups without adding containers. Preserve meaningful DOM and reading order when visual emphasis differs from semantic heading levels.
- Reuse the type scale. When none exists, start with a 1.2 or 1.25 ratio, 6–8 sizes across the artifact, and roughly three visible functional levels in one region. Avoid a ladder of nearly identical sizes such as 18/20/22 px.

| Role            | Starting size            | Latin line-height                                       |
| --------------- | ------------------------ | ------------------------------------------------------- |
| Display         | 48–72 px                 | 1.0–1.2                                                 |
| H1 / H2 / H3    | 32–48 / 24–32 / 20–24 px | Tighten only while multiline text stays clear           |
| Body            | 15–18 px                 | 1.5–1.6                                                 |
| Small / caption | 13–14 / 11–12 px         | About 1.5; do not put essential reading at caption size |

- Keep body measure around 50–75 characters; `max-inline-size: 65ch` is a useful baseline. Apply character measures to the text element at its own font size. A narrow `ch` limit on a body-sized parent can squeeze a large heading into one-word lines; give display text its own responsive width. Use start alignment instead of justified web body copy.
- Use a coherent font family or a purposeful display/body pair, with working fallbacks. Three weights often suffice: read at 400/450, emphasize at 500/550, announce at 600. Follow approved weights instead of forcing this preset.
- For Latin fonts, begin with body tracking at `0`, small text at `0.01–0.02em`, capitals at `0.06–0.1em`, headings above 32 px at `-0.01–-0.02em`, and display above 48 px at `-0.02–-0.03em`. Check the actual font; tracking must not impair reading or other scripts.
- For editorial work specifically, larger jumps can create pacing: display 56–96 px, deck 18–24 px, body 16–18 px, pull quote 28–40 px. Sustained serif reading often benefits from 60–70ch and 1.6–1.7 leading. Use space and occasional typographic interruption; repeated bold phrases or identical callout boxes erase their effect.

## Intentional composition and real content

- Organize around the user's task and content relationships. Use rows for comparison, columns for parallel content, and containers when a boundary communicates grouping or interaction. Repeated cards, uniform section padding, and a fixed hero/features/pricing sequence need a content reason.
- Keep familiar interaction patterns; put distinction into a considered proportion, type treatment, image crop, or product detail. One strong visual choice is easier to sustain than unrelated flourishes. Symmetry, gradients, rounded corners, and any particular hue are choices, not automatic defects.
- Use approved assets and accurate product imagery. Preserve logos, artwork, and brand identity. Avoid emoji substituting for a coherent icon set, random stock imagery, decorative blobs, and unrelated placeholder URLs. Match icon size, optical weight, and alignment to nearby controls.
- Write actual task labels: “Start tracking” conveys more than “Get started.” Do not invent metrics, endorsements, customers, screenshots, or product capability. For an authorized mockup, clearly label illustrative content; otherwise work with known content and expose missing material honestly. Do not fill empty space with filler prose.

## Color and contrast by state

- Reuse semantic tokens for background, surface, foreground, muted text, border, accent, and status. Keep palette primitives within the existing system instead of scattering raw colors through components.
- In a neutral interface, 70–90% neutral area and 5–10% accent are useful balance heuristics. Start with one primary accent and one or two focal uses per region, then judge the composition. Navigation, links, focus indicators, charts, and meaningful status must remain identifiable; never ration them away to satisfy an accent quota.
- Use effects to communicate hierarchy or depth. A gradient or glow that only fills unused space usually competes with content. Derive dark surfaces and borders from the approved palette; pure black or white is not inherently wrong.
- Normal text needs at least 4.5:1 contrast. Large text qualifies for 3:1 at 18 pt regular (24 CSS px) or 14 pt bold (about 18.67 CSS px), not 18 px regular. Do not round a failing ratio upward.
- Required visual boundaries and state indicators need 3:1 against adjacent colors. Check actual backgrounds, including transparency, and default, hover, focus, selected, and error states. Muted informative text is still text; disabling a control does not justify hiding the reason it is unavailable.
- Pair status color with text, shape, icon, or another cue. Keep links recognizable beyond hue. If an accent fails as text, use an accessible text variant while retaining the approved bright color where it works as a fill.

## Targets, focus, and semantics

- Prefer native buttons, links with real destinations, inputs, and landmarks. Give controls accessible names; supply useful image alternatives and empty alternatives for decoration. Give charts a textual account of their information. Use the platform's accessibility primitives for native UI.
- Pointer targets should be at least 24×24 CSS px, or meet the WCAG spacing/equivalent/inline exceptions where applicable. Prefer 44×44 CSS px for touch controls when space permits; enlarge the hit area rather than the visible icon. This is a comfort target, not the AA minimum.
- Retain an obvious keyboard focus indicator; a 2 px outline with 3:1 contrast is a practical starting point. Keep it visible against each surface and clear of sticky overlays or clipped containers. Never remove an outline without a usable replacement.
- Keep keyboard order meaningful without positive `tabindex`. Buttons support Enter and Space; links support Enter. Modal focus stays inside while open, supports the appropriate dismissal behavior, and returns to the invoking control. Background status updates should not steal focus.

## Data and form states

Cover states the changed surface can actually reach; do not add workflows merely to fill a checklist.

| State     | Useful behavior                                                                                                            |
| --------- | -------------------------------------------------------------------------------------------------------------------------- |
| Loading   | Stable shell or layout-matched skeleton; indicate the operation when a delay becomes noticeable.                           |
| Empty     | Distinguish first use from no results; explain the situation and offer a useful creation or filter action when available.  |
| Error     | State what failed, why if known, and an available recovery action; preserve entered data and working surrounding sections. |
| Populated | Render real content and the primary task without decoration overwhelming it.                                               |
| Edge      | Handle long text, missing optional data, relevant volume limits, and partial results.                                      |

- Avoid flashing a loader for sub-300 ms work; roughly 300 ms–2 s may warrant a small indicator, and longer waits need a named operation. Around 15 s, explain an unexpected delay. Show determinate progress only when measurable, and cancellation only when supported. A long-running job needs truthful status and a recovery path, not a fabricated percentage or arbitrary failure timer.
- Keep field labels persistent and associated with inputs. Expose requiredness, formats, and relevant constraints before submission. Use suitable types and autocomplete; `inputmode` chooses a keyboard, not validation. Identifiers such as postal codes and card numbers are text, not quantities with numeric spinners.
- Begin untouched fields without error styling. Validate after an edited field loses focus or after submit; once invalid, revalidate while editing so correction clears promptly. Keep helpful instructions visible. Do not move focus on keystrokes.
- On rejected submission, identify the precise failed rule. Connect messages with `aria-describedby` and set `aria-invalid`. Focus a summary linking to invalid fields, or the first invalid field. Avoid making the same focused summary an assertive alert and announcing it twice.
- While submitting, prevent duplicate actions and announce status politely without clearing values. Restore the usable form on failure. Do not disable fields before reading their values: disabled controls are omitted from native form submission.
- Reuse the existing validation contract; the server remains authoritative. Background checks may debounce around 250–500 ms, but stale responses must not overwrite newer input or leave submission disabled indefinitely. Preserve allowed paste/autofill and avoid asking for the same information twice without a real need.

## Language and motion when relevant

- Set document and mixed-content language correctly. For CJK, start display leading at 1.3–1.4 and body at 1.7–1.8, with zero display tracking; large multiline heroes still need that clearance. Do not inherit tight Latin settings into CJK blocks.
- For Arabic and other joining scripts, use zero tracking and enough vertical clearance for diacritics; Arabic body often needs 1.5–1.75 leading. For RTL, set `dir` as well as `lang`, use logical spacing and start/end alignment, and preserve meaningful reading order.
- Use `dir="auto"` for unknown-direction text and `<bdi>` for embedded values. Force LTR for intrinsically LTR email, URL, phone, or account values, including weak-character-heavy numbers. Mirror directional navigation when appropriate, not logos, photos, clocks, or media playback timelines.
- Animate to confirm feedback, orientation, expansion, or progress. Useful starting durations: 50–100 ms for a press, 150 ms for state feedback, 200–300 ms for entering UI, 300–500 ms for a screen transition. Keep frequent interactions around 200 ms or less, following existing motion tokens.
- Honor reduced motion by removing translation, scale, rotation, and parallax; retain a static state cue, with an optional gentle opacity transition. Do not rely on motion alone. Provide pause/stop/hide for nonessential automatic movement lasting over five seconds alongside other content, and avoid rapid flashing. End ambient effects when their surface leaves.

## Check the changed surface

Use the smallest representative checks that can expose its risks: narrow and wide supported layouts, a touched breakpoint, keyboard operation, text enlargement, and the relevant data states. Include long real labels and absent optional content; add RTL, reduced motion, large volume, or alternate themes when the change affects them. Fix clipping, hidden actions, broken grouping, or inaccessible focus where observed. Match verification effort to the change and reuse valid evidence.
