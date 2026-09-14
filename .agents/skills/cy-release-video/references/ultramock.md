# Ultramock operation

## Camera and timeline editing

Open the user's authenticated Chrome project through the supported browser tool.
Inspect the current scene list, source preview, duration, and playhead. Keep the
existing Marketplace or other approved sections when revising a single feature.
Inspect the start, middle, and end: a path can traverse empty space even when
both endpoints look good. Select a scene and its start/end keyframes explicitly; do not assume clicking a
clip selects the intended endpoint.

Use real image states as separate source shots. Inspect both the source thumbnail
and rendered scene after replacement. A toast referring to an older shot is not
proof that the intended source changed. Source upload can be asynchronous.

The observed editor offered Simple/Advanced timelines, camera X/Y/Z axes, FOV,
Zoom, Pan X/Y, easing, depth/focus, and motion blur. Auto-motion can generate a
starting move from multiple focus rectangles; inspect and refine the targets.
Do not use one broad rectangle and call the resulting generic zoom finished.

**The observed Zoom control behaved like distance:** increasing it made the
image smaller. Verify direction visually on the selected scene before applying
values across the timeline. FOV, source aspect ratio, perspective, and pan all
change framing, so copied parameter sets need visual adjustment.

Historical integrated-session example (not a preset): FOV 32; first scene moved
from X −3 / Y −7 / Zoom 2.15 / Pan X 0 to X 3 / Y 6 / Zoom 1.85 / Pan X −0.25.
The next scene reversed toward X −3 / Y −5 / Zoom 2.15 / Pan X 0. Both retained
conversation and sidebar. Earlier study ranges of 8–18° pitch/yaw and FOV 24–35
were creative proposals, not settings recovered from reference videos.

When changing feature order, reorder the actual Ultramock scenes, export again,
then update all Remotion source offsets. Reordering only Remotion would leave the
user's editable camera project inconsistent. Keep an explicit scene/time map;
repeated scene labels such as “Shot 7” are not unique identifiers.

## Export

1. Select video, correct orientation and resolution, 60fps when matching this
   master, suitable quality, and restrained motion blur. Recheck every setting
   rather than relying on the last export's label.
2. Keep the exporting Chrome tab open and foreground; the observed exporter
   pauses when switching tabs/minimizing. Independent local editing can continue.
3. Confirm a new download after completion. The historical filename contained
   `5s` even for a complete 42-second timeline. Use media metadata, not its name.
4. Verify dimensions, frame rate, duration, actual frames, and full decoding.
   Stale editor toasts were absent in the inspected exports; check the export
   before changing content to “fix” an editor-only artifact.
5. Copy the new source to a distinct versioned filename under the release media
   folder. Update configuration only after confirming the file is the intended
   export. Preserve the previous camera source and final cut.

## Recovery, only when the symptom occurs

- An export or uploaded source appears unchanged: inspect the active scene,
  source aspect ratio, and native file dialog before repeating the action.
- Chromium file chooser automation returned “Not allowed” in this run. Follow
  the active tool's upload documentation; do not silently broaden extension
  permissions. The authorized native file picker was a working alternative.
- In the macOS picker, Command–Shift–G opens the path field. Setting its full
  value worked more reliably than simulated typing, which lost characters to
  autocomplete. Return selects the file; inspect the file preview/dimensions,
  then Return confirms Open. Clicking an apparently correct Open AX item left
  the dialog open in one attempt, so verify it actually closes and applies.
- Accessibility indices, tab IDs, project IDs, and screen coordinates from old
  runs are historical only. Reacquire them from the current visible state.
- Do not use guessed storage writes or private app state to bypass editor UI.
  Keep the normal project save/export behavior observable and reviewable.
