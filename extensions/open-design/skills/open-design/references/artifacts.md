# Artifact composition

Read the section matching the requested output. These are adaptable techniques,
not required templates. The project's current components, tokens, approved assets,
and visual references take precedence. Use [craft.md](craft.md) for shared finish,
accessibility, and state guidance.

Write directly openable HTML under the workspace's `docs/design/`, respecting any
requested filename. One file with inline CSS and small local scripts is enough for
a small request; use relative local assets when needed. The result must work
without the installed extension, a build, a service, or a custom viewer.

## Interfaces

Start with the task and the existing product shell. A feature addition may need
only a focused panel or a few related states on one board. Do not turn a list,
dialog, or settings change into an analytics dashboard.

- Establish the reading order: page context, primary task, working content, then
  supporting information. Keep the main action close to the objects it affects.
- Match the application's density. Use aligned rows for repeated records and
  columns for attributes people compare; reserve cards for independently useful
  groups. Preserve existing navigation and component anatomy.
- For a dashboard, lead with the question it answers. Use a summary metric only
  when supplied data supports it; follow with a primary chart and relevant detail
  table when those help the task. Do not manufacture KPIs to fill a row.
- Simple charts can be inline SVG. Include readable labels, units, periods, and
  an accessible description; make numeric columns easy to compare with tabular
  figures. Use accent to identify the relevant series or state.
- Preserve content space on narrow screens: collapse secondary rails, reflow
  controls, and keep dense tables in their own labeled scroll region. Choose
  sticky headers or independent scrolling only when the shell requires them.

Demonstrate the interaction requested, using local illustrative records and local
state. Search, filters, tabs, selection, menus, and dialogs should change the
visible prototype when they are part of the proposed flow. A demonstrated delete
must never reach a real product API. Distinguish provisional behavior from an
approved product contract.

For example, a sessions bulk-delete sketch can be one HTML containing the existing
list layout, selectable rows, a selected-count action bar, and the requested
confirmation/result states. Keep selection and button availability consistent.
Include pending, empty, or partial-failure states when they belong to the brief;
do not invent recovery guarantees. Related states can share a board instead of
requiring a separate file for every variation.

## Sites and landing pages

Let the content determine the sequence. Lead with a clear proposition or thesis,
then supply the evidence and detail needed for the visitor's next decision.
Remove sections without useful content instead of filling a template.

| Content need                    | Useful composition                                                  |
| ------------------------------- | ------------------------------------------------------------------- |
| A concise proposition           | Focused headline and lead, followed by the primary action           |
| A product that needs to be seen | Copy beside a relevant product image or local UI demonstration      |
| A deeper explanation            | Unequal text/visual split with a clear dominant region              |
| Dated entries or an index       | Ruled rows: date or category, title, short description              |
| Comparable options              | Table with shared row labels and aligned values                     |
| A supported outcome             | A prominent sourced number with its context, or an attributed quote |
| A concluding action             | A concise closing statement and an explicit next step               |

Vary section width, alignment, and information density where the narrative changes.
A spacious introduction can lead into a compact comparison or detailed product
view. Avoid repeating the same feature-card row through the whole page. Maintain
a recognizable grid and type hierarchy while changing composition.

Use actions that describe their destination. Local anchors must reach real
sections; prototype controls must visibly demonstrate their stated local effect.
Do not present a form as submitted to a service when it only changes local state.

Preserve the full frame and intrinsic proportions of content images. Crop only
deliberately decorative imagery, with the subject still visible. Use approved
assets when available; do not substitute invented logos, testimonials, or business
results. A missing asset can be an explicitly labeled placeholder if necessary.

Check how the reading order survives a single-column layout. Keep headings and
leads bounded, stack split sections coherently, and allow navigation to adapt
without obscuring the main action.

## HTML slide decks

Build a deck as discrete slides with navigation. A tall scrolling article does
not satisfy a presentation request. Use the requested aspect ratio; otherwise
16:9 is a useful starting point. Fit the composition within that canvas without
clipping text or reducing it to unreadable sizes.

Give each slide one principal message and a short identifying title. Choose
layouts by the argument: a cover, a thesis with supporting detail, a comparison,
a small sequence of steps, an evidence chart, a quotation, or a final decision.
Use only the slides the story needs. A major number deserves space when it is
supported by a source; unsupported data is not a composition device.

Create rhythm through changes in scale, density, alignment, and image use. Let
a dense evidence slide breathe into a concise takeaway. Contrasting light/dark
surfaces are an option when the visual system supports them, not a required
alternation. Keep titles, captions, folios, and content margins consistent.

- Provide visible Previous/Next controls and Left/Right keyboard navigation,
  with a current-slide indicator. All navigation updates the same slide state.
- Do not capture navigation keys while the user is editing a field. Keep focus
  visible, and keep hidden slides out of the focus order.
- If wheel or touch navigation is included, move predictably by one slide and
  preserve scrolling inside content that needs it. Do not auto-advance unless
  requested. Respect reduced-motion preferences.
- Preserve the active slide when the viewport changes. Refit the canvas or
  reduce layout complexity instead of cropping the footer or navigation.
- Provide a readable no-script fallback. Print styles must reveal every slide,
  hide navigation, and place each slide on its own page without clipped content.

An editorial deck can use generous outer margins, hairline-separated comparisons,
asymmetric columns, restrained metadata, and occasional large quotations. Those
techniques do not require a particular font, palette, slide count, or decoration.

## Visual documents

Reports, proposals, guides, and brandbooks need a reading structure suited to
their material. Establish title, summary, section hierarchy, body, captions,
tables, and source notes as distinct roles. Start with a readable text measure
(roughly 60–75 characters for sustained prose) and adjust for the typeface and
content. Keep side notes subordinate to the main argument.

Use editorial composition deliberately: an opening statement with room around
it, a compact metadata line, a large diagram with its explanation, a ruled
comparison, or a pull quote with attribution. Keep adjacent comparisons aligned.
A label or hairline can separate material without wrapping every paragraph in a
card. Do not stretch a short document into a prescribed page or word count.

For documentation, a central article with section anchors is the core. Add a
grouped navigation rail for multiple topics and a table of contents for a long
page when useful. Collapse the rails on smaller screens while preserving access
to their destinations. Use logical CSS properties where direction may vary.
Code blocks should scroll within their own region; copy controls should actually
copy when presented as working controls. Examples and API claims must come from
the supplied material or be clearly identified as illustrative.

For printable documents, include `@media print` rules that remove screen-only
controls, reset sticky/fixed positioning, and expose all content. Avoid page
breaks inside short figures, tables, and callouts; keep headings with their
following text. Let long content paginate rather than clip. Use `@page` size and
margins when the requested format calls for them, and preserve legibility without
background printing. Link source notes to their actual references when provided.
