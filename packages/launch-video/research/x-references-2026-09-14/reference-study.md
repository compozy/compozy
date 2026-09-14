# Marketplace — Camera Reference Study

10 downloaded X references, September 14, 2026. Visual findings are based on extracted frames; no audio analysis or exact reconstruction of source keyframes.

## 1. pedro — Primary camera reference

Source: https://x.com/pedronauck/status/2089895410307399692

Duration: 5.00s. Local video: `pedro/download/video.mp4`.

0.0–1.0s: oblique screen rotates toward frontal. 1.3–3.8s: perspective changes and the screen expands beyond frame edges; the camera traverses the graph. 4.4–5.0s: returns to an angled establishing view. Use the changing yaw, tight crop and continuous movement; avoid a perpetual generic zoom.

## 2. compozy — Primary sequence reference

Source: https://x.com/CompozyOS/status/2089432640176717927

Duration: 20.23s. Local video: `compozy/download/video.mp4`.

0.0–1.3s: rapid move from a small establishing screen to a close crop of graph nodes. 2.5–5.1s: the right inspector becomes the visual target. 6.3–10.1s: lateral traversal over another graph. 11.4–14.0s: angled Loops list. 15.2–19.0s: closer detail screen. Use an explicit destination for each move and alternate graph/list/detail views.

## 3. javi — Selective reference

Source: https://x.com/rameerez/status/2015859121661059569

Duration: 28.05s. Local video: `javi/video.mp4`.

0–3.5s: isolated license control at readable scale with visible changing state. 8.8–12.3s: shallow perspective code panel. 17.5–21s: dashboard enters cropped from the bottom and fills most of the width. Use control-first framing and staggered reveals. Reject the title-card structure for this demo.

## 4. remotion — Selective reference

Source: https://x.com/Remotion/status/2013626968386765291

Duration: 8.09s. Local video: `remotion/video.mp4`.

0.5–4.0s: terminal panel rises from below, with a trapezoidal perspective and lower edge out of frame; the command stays near the visual center. 4.5s onward: titles/logos. Use the screen-entry perspective only; its second half is unsuitable for the requested image-only film.

## 5. hyperframes — Rhythm reference, low camera relevance

Source: https://x.com/HeyGen/status/2044827454460871072

Duration: 49.83s. Local video: `hyperframes/video.mp4`.

3.1–6.2s: video within a rounded panel, followed by an expansion toward the presenter. 18.7–22s: fast contrast between a grid and full-frame imagery. 37.4–40.5s: layered timeline cards rotate/fan. Use scale contrast and overlapping transitions sparingly. Reject presenter, explanatory text and decorative visual sections.

## 6. motion — Reference to avoid as main style

Source: https://x.com/motion_so/status/2094123213735305359

Duration: 22.11s. Local video: `motion/video.mp4`.

2.8–4.1s: oversized typography deliberately cropped by frame edges. 6.9–8.3s: push into prompt control. 15.2–19.3s: nested preview shifts and expands. Useful for decisive changes in scale, but dominated by whitespace and text; this is not the visual direction for the marketplace demo.

## 7. ultra-automotion — Tool technique reference

Source: https://x.com/joshmillgate/status/2093489921180791012

Duration: 23.33s. Local video: `ultra-automotion/video.mp4`.

0–14.6s shows setup, including several focus rectangles on distinct UI regions. 16.0–17.5s: angled phone with substantial zoom. 19.0–21.9s: camera advances down to the lower controls while retaining perspective. Use multiple focus targets, 3D mode and deliberate close crops, instead of one broad rectangle.

## 8. ultra-launch — Depth and focus reference

Source: https://x.com/joshmillgate/status/2038786557772005793

Duration: 15.83s. Local video: `ultra-launch/video.mp4`.

0–11s: full-frame oblique dashboard with lateral/vertical traversal across rows. A selective blur leaves a band of detail legible while distant regions soften. 11.9–14.8s: light scene variation with the same camera language. Use shallow perspective and targeted focus; do not blur the feature being demonstrated.

## 9. ultra-eve — Cinematic reference

Source: https://x.com/joshmillgate/status/2072697893182443730

Duration: 21.30s. Local video: `ultra-eve/video.mp4`.

4.0–5.3s: monitor viewed obliquely, traversing the frame. 9.3s: close layered directory illustration. 12.0–14.6s: camera moves across a code region with pronounced perspective and selective blur. Use directional continuity, depth and detail crops. Reject the long logo/title interruptions for this film.

## 10. ultra-timeline — Primary product-detail reference

Source: https://x.com/joshmillgate/status/2065197563351871741

Duration: 13.00s. Local video: `ultra-timeline/video.mp4`.

0–4.9s: close oblique dashboard, moving across graph and metric; screen extends beyond every edge. 5.7–8.1s: change of angle and subject to the activity/notification column. 9.8–12.2s: another diagonal sweep through graph and values. Use distinct camera segments, close framing and dark continuous background. Source is portrait; adapt its framing principle to 16:9.

## Revised demo contract

- Images only, no intro/outro titles, captions or added logos.
- 16:9, target 32–36 seconds, 1080p.
- Six real screenshot subjects: catalog, search, install configuration, installed inventory, resources, sources.
- Screen width normally 90–115% of frame; close details may exceed 150%. Brief establishing view at least 85%. These are visual occupancy targets, not the Ultramock Zoom control units.
- Target region stays readable for roughly 1–2 seconds after each move settles.
- Change perspective and focal region deliberately: oblique catalog reveal; lateral search move; frontal installation close-up; low-angle inventory; overhead resources traversal; source-management finish.
- Proposed camera ranges: modest yaw/pitch around 8–18 degrees, roll around 0–4 degrees, FOV around 24–35. Tune against actual frame and UI coordinate conventions; do not assume source settings.
- Use acceleration/deceleration around the larger reframes, restrained motion blur during travel, and sharp stationary UI. Avoid repetitive slow center zooms, long fades and blank gaps.
- Verify exported frames at all scene boundaries and representative close-ups; source screenshot text must not become unreadable through excessive blur or extreme perspective.

## Implemented revision

Project: https://www.ultramock.io/?project=cmu1eadzy000204js9kdosdyp

Six image-only scenes, 32-second timeline, 1920×1080, 60 fps, H.264, High quality, Low motion blur. Dark background #111111. Camera keyframes explicitly vary X/Y axes, roll, FOV, zoom and pan. Scene durations: 5, 5, 5, 5, 6, 6 seconds. Main travel occupies about four seconds, leaving short final holds. No added titles, captions or logos.

Scene subjects and paths: catalog overview to right-column close-up; search and herdr result; installation dialog with large readable framing; installed plugin labels to enable controls; resource inventory traversal; plugin marketplace sources to contextual closing frame.

Observed export QA: original revision decoded without errors; 18 representative timestamps across all six scenes were visually inspected. That inspection prompted a tighter inventory framing before the final export.
