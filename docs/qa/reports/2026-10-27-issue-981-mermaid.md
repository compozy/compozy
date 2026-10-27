# Mermaid sizing and fullscreen — issue #981

## Scope and charter

Bruno reads architecture diagrams in a session, follows a streamed explanation, opens a diagram
for closer inspection, and copies its source. Targeted J-12 / RT-web-mermaid-diagrams walk;
visual and interaction tour, including narrow/wide layouts and light/dark appearance.

The branch frontend runs against an isolated installed v0.4.2 daemon with a separate home,
HTTP port, socket and workspace. The user's daemon is untouched. Real Codex GPT-6.1-Sol,
Medium reasoning produces the transcript; no mocked transcript or renderer is used.

| Check | Status | Evidence |
| --- | --- | --- |
| Streaming geometry and multiple diagrams | Pass | Six live diagrams, 466 samples at 50ms intervals; every observed block remains 282px from source through image |
| Fullscreen, zoom, pan, reset, source/copy | Pass | Real chat: 125% zoom, 100px/80px drag, reset to 100%, original source copied |
| Storybook narrow/wide, light/dark | Pass | 320×800 dark, 1100×700 dark, 1680×1050 light; no page overflow; three real Mermaid diagrams each |
| Natural user journey and fresh transcript load | Pass | Order-service architecture request produced three diagrams and prose; all nine transcript diagrams survived reload; Escape dismissed the viewer and reopening reset to 100% |
| Live transcript viewport/theme evidence | Pass | 390×844 dark and 1440×1000 light/dark; dialog inset 16px from viewport edges |

## Observations

The original renderer replaced a growing code block with an image without reserved height.
Mermaid also emitted percentage SVG widths, so image sizing lacked reliable natural dimensions.
The chat's existing ResizeObserver pins following readers to the end as content changes; it
responds to these height changes but does not require a separate scrolling modification.

The fix reserves a bounded frame for source and image, normalizes SVG intrinsic dimensions from
the viewBox, and limits inline images to half the column without enlarging their natural size.
The deliberate tradeoff is unused vertical space around short diagrams. Fullscreen uses the shared
Dialog outside the desktop window's overlay container and adds zoom and pointer pan.

The first synthetic live probe explicitly requested UI-test diagrams; its measurements establish
rendering behavior, not an in-persona verdict. The final journey uses a natural architecture request.

## Automated evidence

- Root Turborepo focused StreamMarkdown suite: 18 tests passed.
- `mise exec bun@1.4.0 -- make gate`: passed, including 143 UI files / 878 tests,
  typecheck, codegen check, and lint with zero warnings/errors.
- MermaidSizing Storybook play checks stable source/image height, half-column bounds, natural
  size and viewport height limits using actual browser geometry and decoded images.
- React Doctor found no issues in the changed files; external score retrieval was blocked by automatic approval review because the transmitted source/diff data was not established. No external score is claimed.
- The isolated lab's strict evidence audit passed with no blockers or warnings.

## Session debrief and evidence

Bruno requested order-service documentation and received a seven-stage checkout-to-delivery flow,
a customer/API/warehouse sequence, and a service overview. The explanations stayed separate from
the diagrams. He enlarged a diagram, zoomed, reset, dismissed with Escape, and reopened it at 100%.
Reload preserved the transcript. The earlier six-diagram probe additionally exercised pointer pan
and copied the original Mermaid source. No further issue-specific friction was observed.

Captured artifacts under `docs/qa/evidence/2026-10-27-issue-981-mermaid/`:

- `live-1440-light.png`, `live-1440-dark.png`, `live-390-dark.png`: real transcript.
- `live-fullscreen-1440-light.png`, `live-fullscreen-1440-dark.png`, `live-fullscreen-390-dark.png`: viewport dialog at 125%.
- `inline-1100-dark.png`, `inline-320-dark.png`, `inline-1680-light.png`: multi-diagram Storybook.
- `streaming.png`: source inside the reserved frame.
- `results.json`, `live-results.json`: measured image/frame/viewport bounds.

## Limits and delivery

Targeted Chromium/macOS browser evidence; packaged desktop and other operating systems are not
retested in this round. Existing invalid-source, renderer-failure and security coverage remains in
the canonical StreamMarkdown suite/stories. PR CI and external reviews are separate delivery gates
and are not established by this report. Lab teardown and public evidence links will be recorded
after the remaining delivery checks. Targeted browser QA: PASS. PR delivery remains in progress.
