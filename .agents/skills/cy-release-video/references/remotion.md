# Remotion assembly and validation

## Workspace ownership

The package is `packages/launch-video`, named `@compozy/videos`. From repository
root:

```bash
bun install
bunx turbo run studio --filter=@compozy/videos
bunx turbo run lint typecheck --filter=@compozy/videos
bunx turbo run render:release --filter=@compozy/videos
bunx turbo run render:all --filter=@compozy/videos
```

The root Bun lockfile owns dependencies. Rendering is opt-in and uncached, outside
the application build. The package retains local camera MP4s and `out/` exports
but Git ignores them; a fresh clone needs those sources. Check actual source
existence before rendering. PNGs, code, configuration, and notices are versioned.

## Reusable opening and release configuration

`src/releases/beta26.json` owns version, feature lines, intro duration, shot order,
source paths/start offsets, durations, overlaps, and cursor cues. The opening and
full film read the same data. For another release, copy its configuration/media
and update imports in `src/Root.tsx` and `release-announce/props.ts`, or pass a full
alternate configuration with `--props`. Also choose a distinct output filename;
do not overwrite the previous release by leaving beta.26's default output path.

Keep the official SVG wordmark, Geist, JetBrains Mono, dark background, and orange
accent from existing brand files. Preserve landscape, square, and vertical
standalone openings. Measure and display title words with the same loaded font;
system-font measurement caused centering risks in the registry component. Test
changed long titles in the requested aspect ratios. Do not keep the old opening's
long static tail when integrating it directly into the product demo.

## Continuous editing and correct interaction

Use the existing `TransitionSeries`, push-through, whip-pan, and fade components.
Start transitions while the outgoing camera still moves and bring in useful
incoming motion. Trim baked source holds first: adding an overlap after a frozen
hold does not fix it. Media buffering and baked stillness are different problems.

Frame counts come from seconds × composition fps. Transition overlap subtracts
from total duration; the opening also overlaps the first shot. Use the existing
`timeline`/`filmDuration` helpers rather than a separately hard-coded total. Trims
must remain within the intended source scene after any reorder. Inspect boundaries
for duplicate, black, or wrong-scene frames.

For cursor bridges, screenshots and cursor share their CSS coordinate system and
one parent camera transform. A flattened Ultramock MP4 has no target DOM geometry.
Do not place a fixed overlay cursor on a moving perspective shot unless its
projected target is tracked. A short screenshot interaction between camera clips
is the proven simpler option.

Waypoints use seconds: `at` is arrival; `duration` is travel ending at arrival;
`click` triggers feedback. The vendored cursor has a 30fps internal clock, so the
adapter converts cues to that clock and uses `speed: 30 / fps`. Changing only the
composition fps would halve fixed-frame animations or desynchronize click pulses.

For `context-click`, supply four full-session images: `src` closed, `hoverSrc`,
`secondarySrc` sidebar open, and `expandedSrc`. Two click cues open and expand.
The current beta.26 camera/hover timings and coordinates are specialized; update
those with the images for another layout rather than assuming configuration alone
can describe an arbitrary interaction. The 35% value is a demo fixture.

A click must contact a visible real control and cause the matching next state.
Small press/ripple feedback is enough. Keep hovered tooltips readable and do not
crossfade unrelated sessions or plugins to fake continuity. Typical overlaps of
0.25–0.45s and short click feedback are starting points, subject to visual review.

## Components and dependencies

Reuse the vendored Remocn Cursor, KineticCenterBuild, and transition presentations
before adding dependencies. Keep all `remotion` and `@remotion/*` packages on the
same version; inspect the installed API/types before copying newer documentation.
The historical integration used 4.0.516 and found newer v5 examples that did not
apply. Do not freeze future releases to this version solely because of this note.

Preserve `THIRD_PARTY_NOTICES.md` and local component adaptations. Existing Exa
research is under `research/remotion-components-2026-09-14/`. RemotionUI's cursor,
Remotion Bits, and remotion-cinematic were alternatives researched, not adopted
or render-validated here. Add a package with Bun only for a concrete missing need;
metadata compatibility is not proof of a successful render.

## Validation and delivery

Run scoped typecheck through Turbo from repository root. Run appropriate source
lint/format checks for code changes. No new test should merely freeze screenshot
existence, prose, styling, or generated frames. A real render is required.

For direct Remotion commands, work inside `packages/launch-video`:

```bash
bunx remotion render ReleaseFilm out/proof.mp4 --frames=0-1199 --scale=0.5 --concurrency=2
bunx remotion render ReleaseFilm out/release-reviewed.mp4 --codec=h264 --crf=16 --pixel-format=yuv420p --muted --concurrency=3
ffprobe -v error -show_entries format=duration,size:stream=codec_name,width,height,r_frame_rate,nb_frames -of json out/release-reviewed.mp4
ffmpeg -v error -i out/release-reviewed.mp4 -f null -
ffmpeg -v error -ss 5.3 -i out/release-reviewed.mp4 -frames:v 1 out/review.png
```

Adjust proof range to the changed sequence and actual duration. Use silent output
only when it matches the brief. Inspect output-resolution close-ups, cursor
arrivals, before/after states, and adjacent cuts. A low-resolution contact sheet
helps compare composition but cannot certify text sharpness. Preview playback for
pacing; frame inspection alone does not establish smooth perceived motion or audio.
Copy to the canonical delivery path only after checks pass, retaining the previous
cut. Show the final MP4 and distinguish static/code checks from visual validation.
