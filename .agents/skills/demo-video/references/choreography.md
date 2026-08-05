# Choreography — making the flow read as a human demo

The recorder gives mechanics (glide, dwell, ripple, beats); this file is the craft of using them. Values marked *shipped* came from a delivered video — starting points, not mandates.

## Shape of a demo

Establish → act → reveal. Open at full frame (`beat(null, 1)`), push in as the action starts, hold tight framings only while something happens there, pull back to full frame for the result, hold ≥2s before `finish()`. A demo that ends the instant the result appears reads as truncated.

- One intent per gesture: frame the target with `beat()`, then move-and-click it. Never move during a camera ease if avoidable — two motions compete for the eye.
- Wait after every transition the app animates (dialog open, window mount) — 800-1100ms *shipped* — so the viewer parses what appeared before the next gesture.
- Park the cursor (`park()`) before capture-relevant motion starts: the overlay fades in at a neutral spot instead of popping mid-screen.

## Gestures

- **Speed:** 850 px/s *shipped*. At ~25fps capture that leaves ~34px between frames (smooth glide); 1350 px/s leaves 54px (stutter). Slower for cinematic, never faster than ~1000.
- **Dwell before click:** 260ms default; 320-420ms *shipped* for clicks the viewer must anticipate (primary buttons, list picks).
- **Typing:** `page.keyboard.type(text, { delay: 130 })` *shipped*. 100-150ms reads as human; type into a framed field only.

## Camera beats

- **Zoom range:** 1.25-1.5. Above 1.5 the crop upscales visibly (see the quality math in recording-findings.md) and small-element fit clamping kicks in anyway.
- **Ease:** 700-1000ms *shipped*. Longer ease for bigger framing changes; `ease: 0` only for the establishing beat.
- Frame *regions*, not pixels: beat on the dialog, not the input — the encoder pads 35% around the rect, and tiny rects clamp to their fit zoom.
- End on a full-frame beat (`reveal`) so the product, not a crop, is the last impression.

## Verify before showing anyone (self-eval)

After encoding, judge the output the way a viewer would — before presenting it:

1. `ffprobe` the file: duration ≈ choreography span, resolution and fps as configured.
2. Extract and *look at* frames (`ffmpeg -ss <t> -i demo.mp4 -frames:v 1 …`): first and last frame, each beat boundary, one mid-ease frame. Check: cursor overlay visible and on-path, framing centered on the intended target, no half-open transition caught mid-beat, text readable at the final resolution.
3. Recorder stats: delivered fps ≥20 (lower reads as choppy), no gesture whose `actual` wildly exceeds `target` (a stall mid-glide).
4. Trim the settle head and dead tail with `START`/`END` at encode time — re-encoding is seconds; re-recording is minutes.

Fix and re-encode until these pass; only then present the video.
