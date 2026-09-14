# CompozyOS release videos

Reusable Remotion opening and release-film assembly. Ultramock supplies camera
movement; Remotion trims the holds, overlaps scenes, and adds screenshot-based
cursor interactions. The demo contains no editorial text overlays.

```bash
# From the Compozy repository root:
bun install
bunx turbo run studio --filter=@compozy/videos
bunx turbo run lint typecheck --filter=@compozy/videos
bunx turbo run render:release --filter=@compozy/videos
```

## Compositions

| ID                        | Size               | Current duration                   |
| ------------------------- | ------------------ | ---------------------------------- |
| `ReleaseFilm`             | 1920 × 1080, 60fps | 42.3s, computed from configuration |
| `ReleaseAnnounce`         | 1920 × 1080, 60fps | 3.2s                               |
| `ReleaseAnnounceSquare`   | 1080 × 1080, 60fps | 3.2s                               |
| `ReleaseAnnounceVertical` | 1080 × 1920, 60fps | 3.2s                               |

`bunx turbo run render:all --filter=@compozy/videos` exports the three opening formats. The opening uses the
official SVG wordmark, an animated version chip, and one or two kinetic title
lines. Long titles scale to the available width.

## Reusing the release

Edit `src/releases/beta26.json`. Its `version` and `features` populate both the
film and all standalone openings. Change `introDuration` for the film's opening.
Use the Studio sidebar to experiment; keep final values in the JSON file.

For a separate release, copy the JSON and media folder, then change the imports
in `src/Root.tsx` and `src/compositions/release-announce/props.ts`. Alternatively,
render a complete alternate film configuration without editing the defaults:

```bash
bunx turbo run render:release --filter=@compozy/videos -- --props=src/releases/next-release.json
```

Each shot defines its source path under `public/`, source start in seconds,
duration in seconds, and outgoing transition (`push`, `left`, `right`, `up`, or
`fade`). `overlap` is the transition duration; it is subtracted from the total
running time. The last shot has no outgoing transition. Source videos should be
normalized to the composition's 60fps before authoring trim points.

Cursor waypoints also use seconds: `at` is arrival time, `duration` is travel
time ending at that arrival, and `click` triggers the press and ripple. Coordinates
belong to the source screenshot and share its camera transform. The two beta.26
interaction scenes are specifically framed for the captured UI:

- `catalog-click`: a 1440 × 900 CSS viewport, targeting the installed shelf.
- `context-click`: a complete 1440 × 900 CSS session viewport. Four captures
  preserve the same conversation: closed, indicator hovered, sidebar open, and
  context details expanded. Two click cues open the inspector and its disclosure.
  Screenshot layers and cursor share one camera transform; the camera moves into
  the composer ring, then back out to establish the inspector in the session.
  `hoverSrc`, `secondarySrc`, and `expandedSrc` supply those states. Different UI
  layouts need their coordinates and framing adjusted in `shot.tsx`.

## Media quality and provenance

The beta.26 sources are in `public/releases/beta26/`.
`ultramock-session-context.mp4` is the revised 42-second 1080p60 export, with
Session Context followed by Marketplace. Its first two camera scenes show the
whole session with its context inspector, replacing the isolated panel crops.
The edit adds a seven-second composer-to-inspector interaction before them.
Earlier exports remain available as `ultramock-context-first.mp4` and
`ultramock.mp4`.

The five `session-*.png` captures are 4320 × 2700 (1440 × 900 CSS pixels at DPR 3).
They compose the actual session thread, context control, and inspector components
in Storybook with deterministic demo fixtures. The 35% usage figure and token
counts are fixture values, not a live provider measurement. The hover and open
states were captured through the actual UI controls; Remotion reconstructs their
sequence and adds cursor/camera movement. The temporary capture story is retained
in `research/session-context-in-session-2026-09-14/`, outside production source.

For future releases, capture the UI at a high device pixel ratio before applying
camera zoom. Export Ultramock at the highest available resolution and avoid
upscaling already compressed video. A higher final bitrate cannot restore detail
missing from the source. Keep the original PNGs and camera export separately.

The final beta.26 delivery is `out/compozyos-beta26-release.mp4`, H.264, 1080p60,
CRF 16, without audio. The source export is also silent.

## Workspace and local assets

This package is `@compozy/videos` at `packages/launch-video`. Bun's root lockfile
owns dependency resolution. Use root Turbo tasks for Studio, typecheck, and
release rendering. Direct `bunx remotion` commands below run from this package.
Rendering is opt-in and uncached; it is not part of the application build.

Camera MP4s under `public/` and generated `out/` files are local and Git-ignored.
The migration preserved them in place; a fresh clone needs the Ultramock exports
listed above before rendering the film. PNG sources, release configuration,
composition code, research, and notices are versioned. Imported skill libraries
and historical raw research are excluded from repository reformatting to preserve
their upstream/evidence contents. Standalone openings do
not require camera exports. Do not replace a missing export with an empty file.

The local pre-migration lockfile and integrity inventory are retained under
`out/migration/`. The previous sibling project directory has moved here.

## Preview and verification

```bash
bunx remotion render ReleaseFilm out/proof.mp4 --scale=0.5 --concurrency=2
bunx remotion still ReleaseFilm out/click.png --frame=228
bunx remotion render ReleaseFilm out/compozyos-beta26-release.mp4 --codec=h264 --crf=16 --pixel-format=yuv420p --muted --concurrency=3
```

Review the full export, especially cursor arrival, disclosure movement, and scene
boundaries. Configuration validation rejects unordered/out-of-scene cursor cues
and overlaps longer than their adjacent scenes. Typechecking does not replace
visual review.

Component research and registry responses are in
`research/remotion-components-2026-09-14/`. Remotion packages are version-matched;
vendored Remocn component attribution and license are in `THIRD_PARTY_NOTICES.md`.
