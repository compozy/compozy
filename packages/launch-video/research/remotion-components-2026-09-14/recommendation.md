# Remotion components for CompozyOS release videos

Research date: 2026-09-14. Discovery used Exa Search; shortlisted documentation was retrieved through Exa Contents. Registry source and npm metadata were inspected directly. This is a research and integration recommendation, not a rendered implementation.

## Recommendation

Extend the existing Remocn setup and add `@remotion/transitions` at exactly `4.0.516`, matching the project's installed Remotion packages. Use Ultramock for the camera treatment and Remotion for the reusable opening, editorial timing, transitions, cursor choreography, and final assembly.

Do not adopt a second complete video framework for this release. Most of the useful pieces already fit the project's copy-in component model.

## Verified local baseline

- `package.json`: Remotion packages pinned to `4.0.516`; React `19.2.0`; no transitions dependency yet.
- `components.json`: `@remocn` registry already configured.
- `src/components/remocn/`: existing Backdrop and locally customized MaskRevealUp.
- `src/compositions/release-announce/schema.ts`: editable version and one or two feature strings already exist.
- `props.ts`: currently beta.23 / Integrated terminal, 150 frames at 30 fps.
- `README.md`: intentionally reserves approximately 50 motionless frames at the end for manual editing. This is about 1.67 seconds at the current frame rate.
- The directory is not a Git checkout. Only this research directory was added during this investigation; composition source and dependencies were not changed.

## Shortlist

| Resource | Applicable pieces | Decision |
| --- | --- | --- |
| [Remocn Cursor](https://remocn.dev/docs/ui/components/cursor) | `Cursor`, `useCursorPath`, arrival frames, eased movement, click ripple, press and drag feedback | First choice for accurately timed interactions. |
| [Remocn camera transitions](https://remocn.dev/docs/transitions/whip-pan) | Whip Pan, [Push Through](https://remocn.dev/docs/transitions/push-through), [Focus Pull](https://remocn.dev/docs/transitions/focus-pull) | Use a small, consistent selection. These are presentations for TransitionSeries. |
| [Remocn Kinetic Center Build](https://remocn.dev/docs/typography/kinetic-center-build) | Short titles assemble word by word and rebalance around the center | Candidate for the opening. Preserve the official SVG wordmark. |
| [Official Remotion transitions](https://www.remotion.dev/docs/transitions/transitionseries) | Scene overlap, transition timing, standard slide/fade/wipe | Required foundation for joining the clips. |
| [RemotionUI Simulated Cursor](https://remotionui.com/docs/components/simulated-cursor) | Percentage coordinates, arrival frames, spring movement, click frames and target rings | Good alternative; unnecessary to maintain two cursor implementations. |
| [Remotion Bits](https://github.com/av/remotion-bits) | Text reveals, motion utilities, particles, gradients and 3D components | Optional future resource. Adds little to this particular camera-and-cursor task. |
| [remotion-cinematic](https://github.com/codeverbojan/remotion-cinematic) | Geometry-aware cursor, scene-relative camera, visual authoring framework | Useful reference, but adopting the framework would duplicate Ultramock and increase integration work. Capabilities were reviewed from its published README, not tested. |

## Source-level findings

- The Cursor registry provides two files and installs the shared Remocn UI core: six more files and `culori`. It supports explicit waypoint arrival frames and separate move durations. This is more precise for click-to-transition synchronization than fixed travel times.
- Remocn also offers [Simulated Cursor](https://remocn.dev/docs/effects/simulated-cursor): one file, only Remotion as a dependency. Its movement uses fixed 24-frame legs plus holds; prefer the newer Cursor API for the planned choreography.
- Whip Pan, Push Through and Focus Pull are each a single source file depending on `remotion` and `@remotion/transitions`.
- The `@remotion/transitions@4.0.516` package exists and depends on matching `4.0.516` Remotion paths/shapes/core packages. No Remotion upgrade is required for the basic transition plan.
- Current official documentation includes v5 behavior. Implement against the pinned v4 package and types; do not copy newer media/overlay APIs without checking availability.
- Kinetic Center Build measures words with a system font but displays the configured Geist font. Adapt font measurement and loaded-font timing before relying on perfect title centering. Test long titles and the three output aspect ratios.
- Several primitives express timings in fixed frame counts. Author durations in seconds or explicitly scale their timelines for a 60 fps master; changing only `FPS` from 30 to 60 would halve the existing opening duration.
- Remotion Bits `0.2.0` declares React >=18 and Remotion >=4, but also brings Three.js, Prism and MCP dependencies. This is metadata compatibility, not proof of a successful project render.

Raw documentation, component registry files and package metadata are stored alongside this report. One initial Kinetic Center Build documentation path returned 404; the canonical typography URL above was recovered from the official `llms.txt` and its Markdown document was retrieved successfully.

## Proposed Ultramock → Remotion workflow

1. Keep the high-resolution screenshots as source assets. Export camera-treated shots separately from Ultramock, preferably with 0.3–0.5 seconds of useful moving material available around each intended edit. Camera direction, scale and focal point should agree across adjoining shots.
2. The current 42-second export can be split at its known scene boundaries without re-encoding for authoring purposes. It has baked cuts and holds, however: timing must be trimmed before adding overlaps. Adding an effect across an existing long pause does not remove the pause.
3. Assemble the opening and shots in TransitionSeries. Begin the transition while the outgoing camera still moves, and bring in an already-moving incoming shot. Transition duration is subtracted from the combined duration; calculate final duration from this overlap.
4. Use cursor interactions sparingly at meaningful handoffs. The cursor approaches a real visible control, presses, emits a small orange ripple and triggers the matching next UI state. Examples: an extension's Install button opens that same extension's configuration; the context ring opens the Context panel.
5. Author click targets in final screen coordinates, or track their projected locations through the shot. A flattened Ultramock video has no DOM element geometry. The cursor must remain attached to the moving target; it cannot automatically discover a button by ID in an MP4.
6. For complicated perspective moves, use a brief screenshot-based interaction bridge with known geometry, then transition into the next Ultramock shot. Avoid simulating a click on one extension and cutting to a different extension's dialog; recapture matching before/after states where necessary.
7. Render the full master at 1920×1080 / 60 fps. Preview buffering and deliberately frozen source frames are separate issues: premounting helps the former, editorial trimming fixes the latter.

Suggested starting points, subject to visual review: directional transitions around 0.25–0.45 seconds, and short click feedback around 0.12–0.20 seconds. These are creative proposals, not universal component defaults.

## Reusable opening

Preserve the SVG logo, Geist typography, dark background and Compozy orange. Aim for approximately 3 seconds: logo entry overlaps the version appearance, then the feature title enters; the exit moves directly into the first product shot. Keep a separate longer-hold opening variant if it is still useful for manual editing.

For beta.26, use `v0.3.0-beta.26` and the feature lines `Marketplace` / `Session Context`. Keep added editorial text confined to the opening, consistent with the image-only demo direction.

The release configuration should own version, feature/title strings, shot sources and trims, transition choices, and cursor cues. Preserve the existing standalone opening compositions, then add a complete-release composition that reads that configuration. The next release should change configuration and media rather than animation code.

## Integration order and acceptance

1. Add the matching official transitions package and copy only the selected Remocn components, preserving existing local changes in MaskRevealUp.
2. Build and render a short proof containing opening → one Ultramock shot → one correctly synchronized click → the next UI state.
3. Validate the exact click frame, target tracking, moving overlap, absence of black/duplicate frames, font layout and output fps.
4. Apply the proven pattern to the whole beta.26 video, trim idle holds, and inspect every transition plus representative full-resolution frames.

No component has been installed or rendered as part of this research. Source review supports this selection; the short proof above is the next validation step.
