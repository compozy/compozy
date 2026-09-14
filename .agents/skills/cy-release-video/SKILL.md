---
name: cy-release-video
description: "Create or revise CompozyOS release videos with high-resolution product captures, Ultramock camera scenes, and the reusable Remotion package. Excludes publishing tweets and general video editing."
metadata:
  author: Pedro Nauck
  repository: https://github.com/compozy/compozy
---

# CompozyOS Release Video

Deliver a reviewable release/launch MP4 and its reusable sources in
`packages/launch-video` (`@compozy/videos`). Ultramock owns the camera treatment;
Remotion owns the opening, pacing, scene overlaps, cursor interactions, and final
assembly. Continue the existing project instead of rebuilding its animation stack.

## Establish the release

- Read the requested PRs and current implementation. Map each feature to an
  observable UI action and result; a PR description or attractive panel alone is
  not enough to demonstrate the feature.
- Recover current project configuration, captures, latest export, and outstanding
  feedback before editing. Respect the latest requested feature order, duration,
  text policy, and audio choice. Preserve previous deliverables before replacement.
- In this workflow, editorial text belongs in the reusable opening; product shots
  contain real UI. Do not add captions, title cards, logos, or an outro to the demo
  unless requested. The initial beta.26 image-only instruction was later amended
  to include the existing release opening.
- Default to 16:9, 1080p60 for this release family, but use the user's current brief.
  The historical beta.26 final was 42.3s, not a universal duration requirement.
- Choose an action sequence before collecting assets: establish the screen,
  approach the control, show the click, reveal its result, then inspect details.
  A feature with fewer useful states can lead; do not give every feature equal time.

## Read the applicable guide

| Work being done | Read |
| --- | --- |
| New or inadequate screenshots, integrated feature demonstration | [Capture and visual direction](references/capture-and-direction.md), all sections |
| Editing or exporting the camera timeline in Chrome | [Ultramock operation](references/ultramock.md), camera/edit/export sections; recovery only when needed |
| Opening, cursor, trim, transition, or release configuration changes | [Remotion assembly](references/remotion.md), relevant section and validation |
| Reusing the project's visual research or understanding a prior rejection | [Lessons and evidence](references/lessons.md), matching failure or research section |

Use `eng-ui-screenshot` for Compozy capture mechanics and `exa-search` when new
component discovery is needed or requested. Inspect installed components first;
research is not a reason to add another framework. Browser operations follow the
active browser/computer tool's supported API and the user's requested Chrome
session; saved tab IDs and accessibility indices are not reusable instructions.

## Production order

1. Capture matching before/hover/after states at sufficient native resolution.
   Verify the feature still belongs visibly to its session or parent surface.
2. Update Ultramock sources, scene order, and camera targets. Inspect actual frames
   after uploads or parameter changes, then export and verify the downloaded file.
3. Update Remotion source offsets after any Ultramock reorder. Keep screenshot
   interactions and their cursor under the same camera transform. Trigger the
   matching result on the click; movement alone does not communicate behavior.
4. Render a short proof of changed interactions and transitions. Inspect click
   contact, opening/closing states, text sharpness, and continuity; fix source
   captures or geometry before spending time on the complete render.
5. Render the final file. Verify metadata and full decoding, inspect changed
   intervals and adjacent cuts at delivery resolution, and preview playback for
   timing. Report the distinction if only extracted frames were inspected.
6. Preserve the original PNGs, camera export, release configuration, licenses,
   and final MP4. Update package instructions when the reusable workflow changes.
   Remove owned temporary capture stories, stop only owned capture servers, reset
   temporary viewport overrides, and keep the edited project/preview available.

## Completion

Delivery means an accessible final MP4 that demonstrates the requested feature,
plus the sources needed to revise it. A configured timeline, successful upload,
typecheck, or completed export button is not sufficient. State duration,
resolution/fps, and any material limitation (for example, fixtures versus a live
provider measurement). Link/show the actual artifact; publishing a release tweet
is a separate action requiring user authorization. Do not require a new research
corpus, subagent round, approval, or full application QA for every video revision.
