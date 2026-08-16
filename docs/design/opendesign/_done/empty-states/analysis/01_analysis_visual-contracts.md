# Analysis: visual-contracts

## Scope

- Slice question: What exact in-scope layout, typography, spacing, component anatomy, responsive behavior, and visible interaction states do the three OpenDesign empty-state artifacts require? Classify prototype-only host chrome and copy/data mismatches as placement context or authorized differences.
- Primary sources: `docs/design/opendesign/empty-states/*.html` and the matching `*.artifact.json` files only.
- Sources read in full vs. sampled: all six in-scope files were read in full; no outside sources were used.
- Total candidate sources surveyed: 6.

## Overview

The three artifacts specify one shared zero-inventory composition: an existing window/page shell, a quiet centered introduction, then a bordered panel as the visual center of gravity. The shell uses a 1080px maximum window, a 720px maximum content column, token-backed colors/radii, and a 22px vertical gap. Jobs and Triggers use the same suggestions anatomy; Tasks uses the same anatomy for curated templates. The target empty-state contract is the intro plus panel and its visible states, not the illustrative desktop chrome surrounding it (`jobs-empty-suggestions.html:21-35`, `jobs-empty-suggestions.html:158-180`, `tasks-empty-templates.html:22-35`, `tasks-empty-templates.html:142-164`, `triggers-empty-suggestions.html:23-36`, `triggers-empty-suggestions.html:164-186`).

All three files are self-contained visual prototypes and their artifact records report `status: complete`, `metadata.inferred: true`, and `metadata.reconciled: true`. The JSON `title`/`entry` values use route-like prefixes (`automation/...` or `tasks/...`) that are placement metadata, not a requirement to reproduce those paths in the running app (`jobs-empty-suggestions.html.artifact.json:1-18`, `tasks-empty-templates.html.artifact.json:1-18`, `triggers-empty-suggestions.html.artifact.json:1-18`). Jobs presents three consent-first suggestions, Triggers presents three event-led suggestions, and Tasks presents four editable templates.

## Mechanisms / Patterns

- **Shared frame and column:** `body` is a centered grid with `min-height: 100vh`, `padding: 28px`, a dotted/radial/linear backdrop, and `overflow: auto`; `.win` is `width: min(1080px, 100%)` and `height: min(780px, 92vh)`; `.win-body` is flex; the domain column is `max-width: 720px`, `width: 100%`, `margin: auto`, a vertical flex stack, `gap: 22px`, and `padding: 28px 24px` (`jobs-empty-suggestions.html:21-35`, `tasks-empty-templates.html:22-35`, `triggers-empty-suggestions.html:23-36`).
- **Quiet intro hierarchy:** each intro is a centered column with `gap: 10px` and centered text. Its glyph is a 38px square with 11px content-box padding, token large radius, canvas background, line border, and muted color; the title is 18px, weight 500, with `letter-spacing: -.022em`; supporting copy is 13px with `line-height: 1.6`; the neutral action has a 2px top offset (`jobs-empty-suggestions.html:37-44`, `tasks-empty-templates.html:38-45`, `triggers-empty-suggestions.html:39-46`). The visible titles/actions are “No jobs yet” + “Create from scratch” (`jobs-empty-suggestions.html:158-164`), “No tasks yet” + “Blank task” (`tasks-empty-templates.html:142-148`), and “No triggers yet” + “Create from scratch” (`triggers-empty-suggestions.html:164-170`).
- **Panel and row anatomy:** `.sug-panel` has a 1px line border, large token radius, canvas-soft fill, and hidden overflow. The header is a flex row aligned to the start with `gap: 14px`, `padding: 14px 16px 12px`, and a bottom border; eyebrow/count are an 8px-gap row; the note is 12px, 1.5 line-height, muted, with a 3px top margin and 56ch cap (`jobs-empty-suggestions.html:47-53`; same structure in `tasks-empty-templates.html:48-53` and `triggers-empty-suggestions.html:49-54`). List items are separated by line-soft top rules, except the first. Each row is a two-column grid (`minmax(0,1fr) auto`) with 14px gap and 16px horizontal padding; the disclosure/main button is a three-column grid (`12px 28px minmax(0,1fr)`), 11px gaps, 13px vertical padding, left-aligned, and width 100% (`jobs-empty-suggestions.html:55-60`, `tasks-empty-templates.html:54-59`, `triggers-empty-suggestions.html:55-60`).
- **Typography and compact metadata:** the icon chip is 28px square with a medium token radius and 14px SVG; accent/info/warning tones use the corresponding token tint/color, while neutral rows retain canvas-tint/muted. Main names are 13.5px, weight 560, with a slight negative letter spacing and single-line ellipsis; descriptions are 12.5px, 1.5 line-height, muted, and clamped to one line. Row actions are a non-shrinking flex row with a 6px gap (`jobs-empty-suggestions.html:62-75`, `tasks-empty-templates.html:61-74`, `triggers-empty-suggestions.html:62-74`).
- **Disclosure contract:** detail content is collapsed by default with `grid-template-rows: 0fr`; `data-open="true"` changes it to `1fr`, with a 0.26s token easing transition and overflow-hidden/min-height-zero wrappers. Desktop detail content is inset with `padding: 2px 16px 16px 78px`; Jobs/Tasks use an 8px inner gap, while Triggers uses no gap because its `When / If / Then` card is internally segmented (`jobs-empty-suggestions.html:79-85`, `tasks-empty-templates.html:77-83`, `triggers-empty-suggestions.html:79-90`). Every opener starts `aria-expanded="false"` and points to its detail id (`jobs-empty-suggestions.html:185-203`, `tasks-empty-templates.html:168-185`, `triggers-empty-suggestions.html:191-209`).
- **Jobs review content:** the panel is “Suggested jobs”, count 3, and says “Drafted by Dream — nothing runs until you accept.” Three rows show a name, cron/every pill, one-line description, “Create job”, and “Dismiss”; expanded content is a bordered prompt plus mono facts. The concrete samples are Nightly repo digest, Weekly dependency audit, and Stale run sweeper (`jobs-empty-suggestions.html:168-180`, `jobs-empty-suggestions.html:182-284`).
- **Tasks template content:** the panel is “Start from a template”, count 4, and says “Curated defaults — everything stays editable.” Four rows show One-shot, Recurring via automation, Human-in-the-loop, and Remote from peer, each with a semantic pill, one-line description, “Use template”, and an expandable prompt/facts preview (`tasks-empty-templates.html:152-294`). The footer provides the alternate CLI affordance `compozy tasks new` (`tasks-empty-templates.html:291-294`).
- **Triggers review content:** the panel is “Suggested triggers”, count 3, and says “Drafted by Dream — nothing fires until you accept.” Each row has a trigger/event pill, description, “Create trigger”, and “Dismiss”; its expanded review is a bordered three-row `When / If / Then` card plus a mono guard line. The samples cover `session.stopped`, `hook.deploy.completed`, and a webhook endpoint (`triggers-empty-suggestions.html:174-330`).
- **Visible interaction states:** Jobs and Triggers start with all details closed. Clicking an opener toggles `data-open` and `aria-expanded` (`jobs-empty-suggestions.html:315-321`, `triggers-empty-suggestions.html:358-364`). Accepting replaces the live row with a `role="status"` “Created — [slug]” state, adds an “Open job”/“Open trigger” button, increments the zero header count, and decrements the pending suggestion count (`jobs-empty-suggestions.html:324-339`, `triggers-empty-suggestions.html:367-382`). Dismissing sets `data-state="dismissed"`, removes the live row through the collapsed grid transition, moves the opener to `tabindex="-1"`, and reveals an Undo affordance; Undo restores the most recent dismissal and its focusability (`jobs-empty-suggestions.html:341-366`, `triggers-empty-suggestions.html:384-409`). Tasks wires only the opener disclosure; its `Use template` buttons are present visually but have no prototype click handler (`tasks-empty-templates.html:300-311`).
- **Responsive behavior:** at `max-width: 959px`, body padding becomes zero, the window becomes full-width/full-viewport (`100dvh`) with no radius/border, rows collapse to one grid column, actions move below the main content with `padding-left: 62px` and start alignment, and detail content loses the 78px desktop inset (`jobs-empty-suggestions.html:107-114`, `tasks-empty-templates.html:91-98`, `triggers-empty-suggestions.html:112-119`). Triggers also changes its detail key column from 52px to 44px (`triggers-empty-suggestions.html:118-119`).
- **State matrix:**

  | Artifact | Initial panel | Expand | Primary visible action | Removal/undo |
  | --- | --- | --- | --- | --- |
  | Jobs | 3 suggestions | Prompt + facts | Create job | Dismiss collapses; Undo restores |
  | Tasks | 4 templates | Prompt + facts | Use template (no handler in prototype) | None |
  | Triggers | 3 suggestions | When / If / Then | Create trigger | Dismiss collapses; Undo restores |

## Relevant Sources

- `docs/design/opendesign/empty-states/jobs-empty-suggestions.html:21-114` — shared layout, intro, panel, row, detail, state, and responsive CSS.
- `docs/design/opendesign/empty-states/jobs-empty-suggestions.html:120-366` — Jobs host placement, three rows, copy, and interaction script.
- `docs/design/opendesign/empty-states/tasks-empty-templates.html:22-98` — Tasks layout, template panel, detail, and responsive CSS.
- `docs/design/opendesign/empty-states/tasks-empty-templates.html:104-314` — Tasks host placement, four templates, copy, and disclosure-only script.
- `docs/design/opendesign/empty-states/triggers-empty-suggestions.html:23-119` — Triggers layout, `When / If / Then` detail, state, and responsive CSS.
- `docs/design/opendesign/empty-states/triggers-empty-suggestions.html:126-409` — Triggers host placement, three rows, copy, and interaction script.
- `docs/design/opendesign/empty-states/jobs-empty-suggestions.html.artifact.json:1-18` — Jobs artifact status, route-like title/entry, and reconciled metadata.
- `docs/design/opendesign/empty-states/tasks-empty-templates.html.artifact.json:1-18` — Tasks artifact status, route-like title/entry, and reconciled metadata.
- `docs/design/opendesign/empty-states/triggers-empty-suggestions.html.artifact.json:1-18` — Triggers artifact status, route-like title/entry, and reconciled metadata.

## Transferable Patterns

- **Intro-then-panel empty state** → use for the three live zero-inventory routes: retain a short definition and secondary blank/from-scratch action, but let the suggestion/template panel carry the visual weight.
- **Reusable suggestion row** → implement one domain-prefixed row composite with disclosure, semantic icon chip, title/pill, one-line description, and action slot; Jobs and Triggers can share it, while Tasks supplies a single `Use template` action.
- **Consent-first review** → expose the full prompt/facts or `When / If / Then` detail before creation; preserve the collapsed default, explicit `aria-expanded`/`aria-controls`, and the visible accepted/dismissed morphs.
- **Responsive action stacking** → at the artifact’s 959px breakpoint, stack row actions under the content and remove the desktop detail inset; keep the mobile full-viewport treatment without adding new copy or controls.
- **Placement-context shell** → compare the implementation inside its real route shell, using the artifact’s `.win` header/toolbar/backdrop only to position and frame the empty state. The close/minimize/zoom controls, search field, Rows/Cards or List/Kanban toggles, and top New button are prototype host chrome, not new empty-state requirements (`jobs-empty-suggestions.html:120-154`, `tasks-empty-templates.html:104-140`, `triggers-empty-suggestions.html:126-160`).

## Risks / Mismatches

- **Triggers contract is explicitly proposed:** its source says the daemon currently serves consent-first suggestions for Jobs only and that this screen extends the contract to Triggers (`triggers-empty-suggestions.html:10-20`). Treat the visual anatomy as the intended reference, but treat trigger sample data and accept/dismiss behavior as an authorized prototype difference until the real trigger catalog and create endpoint exist.
- **Tasks action behavior is not represented by the prototype script:** the comment promises that “Use template” opens a prefilled create dialog, but the only listener in the script targets `.sug-open` disclosure buttons (`tasks-empty-templates.html:15-20`, `tasks-empty-templates.html:300-311`). Do not claim dialog behavior is implemented from this artifact alone; verify the owning runtime surface or record it as an authorized interaction gap.
- **Static counts and copy are not invariants:** the visible 3/4 counts and example names are hardcoded in the HTML (`jobs-empty-suggestions.html:168-284`, `tasks-empty-templates.html:152-294`, `triggers-empty-suggestions.html:174-330`). A live implementation should source data from its contract while preserving the anatomy; it must not freeze the samples or silently invent additional rows.
- **Prototype host chrome can cause false parity failures:** `.win`, desktop backdrop, window controls, search, view toggles, and top New actions frame the prototypes but belong to the host route. Rebuilding or replacing the live shell solely to match them would compare placement context rather than the empty-state contract (`jobs-empty-suggestions.html:120-154`, `tasks-empty-templates.html:104-140`, `triggers-empty-suggestions.html:126-160`).
- **Prototype-only post-action controls are inert:** the accepted morph injects an “Open job”/“Open trigger” button through `innerHTML`, but the scripts do not attach a handler to that new button (`jobs-empty-suggestions.html:328-339`, `triggers-empty-suggestions.html:371-382`). Match the visible accepted state, but confirm the real route/navigation behavior before treating it as a shipped interaction.
- **Artifact route names are not local file names:** the JSON records use `automation/...` and `tasks/...` for `title`/`entry` while the sources live directly under `empty-states/` (`jobs-empty-suggestions.html.artifact.json:4-5`, `tasks-empty-templates.html.artifact.json:4-5`, `triggers-empty-suggestions.html.artifact.json:4-5`). Treat that mismatch as artifact placement context, not a required URL or filesystem move.

## Open Questions

- What live API/resource owns Trigger suggestions, and is `createAutomationTrigger`/durable dismissal approved for the target implementation?
- Does the production Tasks route already own the `Use template` dialog, or must this redesign add that interaction? What should the `Blank task`, `New task/job/trigger`, and `Create from scratch` actions do in the empty state?
- Are the sample Jobs/Tasks/Triggers names, counts, pills, and detail text only illustrative, or are they the exact seeded payloads the live routes must display?
- Which portions of the existing route shell should remain unchanged when applying the reference (window controls, search, view tabs, top CTA), and are any of those host controls intentionally out of scope?
- After accept/dismiss, should “Open job”/“Open trigger” navigate to a live route and should undo be persisted across reloads, or is the prototype’s in-memory state sufficient for visual parity?

## Evidence

- `docs/design/opendesign/empty-states/jobs-empty-suggestions.html`
- `docs/design/opendesign/empty-states/jobs-empty-suggestions.html.artifact.json`
- `docs/design/opendesign/empty-states/tasks-empty-templates.html`
- `docs/design/opendesign/empty-states/tasks-empty-templates.html.artifact.json`
- `docs/design/opendesign/empty-states/triggers-empty-suggestions.html`
- `docs/design/opendesign/empty-states/triggers-empty-suggestions.html.artifact.json`
