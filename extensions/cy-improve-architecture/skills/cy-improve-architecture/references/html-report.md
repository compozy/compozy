# HTML Report Format

Render `.compozy/arch-reviews/<slug>.html` as the visual twin of the markdown source of truth. Use semantic HTML and a small inline style layer so every finding remains readable offline. Load Tailwind and Mermaid from CDNs only as visual enhancement.

## Publication contract

- Render from the same canonical candidate records as the markdown report.
- Keep candidate IDs, order, titles, module names, badges, decision callouts, and deletion-test verdicts identical to markdown.
- Escape all repository-derived text before inserting it into HTML.
- Stage the entire document in a temporary sibling file. Replace the prior HTML only after exploration, rendering, parity checks, and markup completion succeed.
- Never stream output directly into an existing good report. A failed re-audit leaves the prior file byte-for-byte intact.

## Scaffold

Use this order so the report reads as a decision rather than a menu:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Architecture audit — {{target}}</title>
    <style>
      /* minimal offline-readable layout and badge styles */
    </style>
    <script src="https://cdn.tailwindcss.com"></script>
    <script type="module">
      import mermaid from "https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs";
      mermaid.initialize({ startOnLoad: true, theme: "neutral", securityLevel: "loose" });
    </script>
  </head>
  <body>
    <main>
      <header id="audit-context">...</header>
      <section id="top-pick">...</section>
      <nav aria-label="Candidate index">...</nav>
      <section id="candidates">...</section>
      <section id="sequence">...</section>
    </main>
  </body>
</html>
```

Keep the only scripts to the Tailwind CDN and Mermaid ESM import. Use inline CSS for a legible single-column fallback, typography, borders, badges, tables/lists, and side-by-side diagrams. If both CDNs fail, headings, candidate facts, Mermaid source, and textual before/after descriptions must remain readable.

## Header

Show the workspace, canonical target, slug, audit date, full/sampled scope, source-file count, and a compact legend:

- solid box = module;
- dashed line = seam;
- red edge = leaked knowledge;
- thick dark box = deep module.

Do not bury the result below an introductory essay.

## Dominant top-pick CTA

Place the CTA immediately after the header and make it the page's dominant card. For a non-empty audit, render exactly one element with the `top-pick` role and include:

- the module;
- the current depth problem;
- the concrete deepening move;
- the maintainability cost or bug-risk evidence that makes it first;
- an anchor to the full candidate card;
- the next action: grill this candidate.

Do not render competing buttons or equal-weight recommendation cards. Put runners-up below as secondary navigation.

For exactly one candidate, make its full card the CTA destination but omit a `1 of 1` candidate menu. For zero candidates, render one visually dominant `Healthy target` outcome panel with no module pick, no fabricated CTA, and no candidate list.

## Candidate card

Render each candidate as `<article id="candidate-<stable-id>">` with:

- module and concise deepening title;
- strength badge: `Strong`, `Worth exploring`, or `Speculative`;
- dependency tag: `in-process`, `local-substitutable`, `ports & adapters`, or `mock`;
- involved files/modules and representative callers/tests;
- one-sentence problem;
- one-sentence deepening move;
- deletion-test verdict plus evidence;
- maintainability-cost evidence;
- before/after visualization and text alternative;
- short wins in locality, leverage, and interface/test-surface terms;
- warning callout naming an active settled decision when the evidence justifies reopening it;
- cross-reference links for non-merged overlapping candidates.

An uncertain deletion test always renders `Speculative`. Never let styling imply higher confidence than the badge.

## Diagram patterns

Choose the pattern that communicates the module's depth problem. Mix patterns across candidates.

### Mermaid call graph or sequence

Use a Mermaid `flowchart` or `sequenceDiagram` for dependencies, calls, and round trips. Use stable node aliases and escaped labels. Keep the source text visible when Mermaid cannot initialize.

```html
<div class="diagram" aria-label="Before: caller knowledge leaks across the pricing seam">
  <pre class="mermaid">
    flowchart LR
      A[Order intake] --> B[Validation]
      B --> C[Pricing adapter]
      C -. leaked rules .-> D[Caller]
  </pre>
</div>
```

### Mass diagram

Show interface height beside implementation height. A shallow before-state has similarly sized surfaces; a deep after-state has a small interface over a larger hidden implementation.

### Cross-section

Stack narrow bands for repeated pass-through modules, then show one thicker deep module that absorbs their caller knowledge.

### Call-graph collapse

Show scattered public calls before and faded internal operations behind one smaller interface after.

### Hand-built boxes and arrows

Use semantic `<div>` boxes and inline SVG only when Mermaid layout obscures the seam. Add an adjacent text description so the evidence does not depend on graphics.

Keep diagrams approximately 320px tall on wide screens and stack before/after vertically on narrow screens.

## Secondary candidates and sequence

Render remaining candidates below the CTA as a compact anchor index followed by full cards. End with a suggested sequence that starts with the top pick and explains later ordering. Keep the ordering identical to markdown and deterministic under equal fix-value.

## Tone and vocabulary

Use plain language and the bundled `cy-codebase-design` nouns precisely: module, interface, implementation, depth, deep, shallow, seam, adapter, leverage, locality.

Prefer concrete phrasing:

- `Order intake is shallow: callers must know validation order and pricing errors.`
- `Deepen behind one submit interface; keep transport adapters at the seam.`
- `Locality: one change site instead of six.`
- `Deletion test: complexity spreads to four callers, so keep and deepen this module.`

Avoid unearned phrases such as `cleaner`, `best practice`, or `easier to maintain`. Name the observed cost.

## Open the report

After atomic publication:

1. Use `open <absolute-path>` on macOS.
2. Use `xdg-open <absolute-path>` on Linux when available.
3. Use `start <absolute-path>` through the Windows command shell on Windows.
4. If the opener is absent, headless, or returns non-zero, print `HTML report: <absolute-path>` and continue successfully.

Opening is a convenience, never a publication gate.
