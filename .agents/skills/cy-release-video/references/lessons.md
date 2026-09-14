# Lessons and evidence

## What changed in the beta.26 workflow

The 2026-09-14 conversation began with a 30–45s Marketplace video for PR #636,
using authenticated Chrome and Ultramock. User feedback rejected editorial text,
small screenshots with empty margins, and simplistic camera motion. A subsequent
request added Session Context from PR #635 and allowed roughly ten more seconds.
The user later explicitly asked for a reusable animated release opening and
Remotion interactions, then placed Context before Marketplace because it had
fewer images. The final correction required the actual context indicator and
sidebar in the session, not just a close-up of sidebar content.

| Evidenced failure | Decision that prevents it |
| --- | --- |
| Low-resolution screenshots pixelated during camera zoom | Recapture native UI at adequate DPR before export; bitrate is not a source-detail fix. |
| Higher-resolution isolated panels remained incomprehensible | Establish the parent session, composer ring, click, and resulting inspector together. |
| Static image holds separated by cuts | Trim holds, use useful moving overlap, and show a causal cursor interaction where it helps. |
| Screens were small with excessive empty space | Check frame occupancy and readable focal targets; avoid fitting every screenshot entirely within margins. |
| Repeated generic zooms looked weak despite a capable camera tool | Use deliberate focus targets, pan, shallow perspective, distance/FOV and easing; review each endpoint. |
| Ultramock Zoom was initially interpreted in the wrong direction | Verify the selected scene visually; larger observed values moved the camera farther away. |
| New source seemed uploaded but old portrait image persisted | Inspect native picker closure, source thumbnail, aspect ratio, and rendered scene before continuing. |
| Remotion registry used fixed frame timings | Convert authoring seconds to each primitive's clock and verify at the final fps. |
| Title measurement font differed from rendered font | Measure with loaded Geist and verify long-title framing, including requested opening ratios. |
| Feature order changed in only one editing layer | Reorder Ultramock, re-export, then re-map every Remotion source start. |

The corrected integrated sequence used production Storybook components with
fixtures: full session → 35% composer tooltip → click opens sidebar → click expands
context → camera views of details and tokens/costs. The final file was rendered,
fully decoded, and inspected through representative output frames. It was 42.3s,
1920×1080, 60fps, H.264, silent. These values and fixture counts describe that
release; they are not requirements for every future video. The user did not
explicitly approve its final visual style; describe it as the last rendered and
inspected revision. Full playback/audio
analysis should not be inferred from a contact-sheet review.

## Reference-video research

Ten X videos were downloaded and sampled into frames at the user's request.
Analysis described visible effects; it did not recover source keyframes or analyze
audio. Reuse this corpus when relevant; ten new downloads are not a recurring
production requirement. Do not claim to have watched or measured references that
were only discovered by search.

| Reference | Transferable observation |
| --- | --- |
| [Pedro, 5s](https://x.com/pedronauck/status/2089895410307399692) | Oblique-to-frontal camera, close graph traversal, angled return. |
| [CompozyOS, 20.23s](https://x.com/CompozyOS/status/2089432640176717927) | Explicit graph/inspector targets, lateral travel, varied list/detail views. |
| [rameerez, 28.05s](https://x.com/rameerez/status/2015859121661059569) | Readable control-first framing and shallow perspective; title-card structure was unsuitable. |
| [Remotion, 8.09s](https://x.com/Remotion/status/2013626968386765291) | Cropped terminal entry with perspective; later logo/title segment was not reused. |
| [HeyGen, 49.83s](https://x.com/HeyGen/status/2044827454460871072) | Scale contrast and overlapping layouts; low relevance for the main camera style. |
| [Motion, 22.11s](https://x.com/motion_so/status/2094123213735305359) | Decisive scale changes, but excessive text/whitespace for this brief. |
| [Ultramock auto-motion, 23.33s](https://x.com/joshmillgate/status/2093489921180791012) | Multiple focus rectangles and movement toward specific lower controls. |
| [Ultramock launch, 15.83s](https://x.com/joshmillgate/status/2038786557772005793) | Oblique dashboard traversal and selective focus that preserves the target. |
| [Ultramock Eve, 21.30s](https://x.com/joshmillgate/status/2072697893182443730) | Directional continuity, depth, code-region close-up; omit long title interruptions. |
| [Ultramock timeline, 13s](https://x.com/joshmillgate/status/2065197563351871741) | Close diagonal sweeps across dashboard details; adapt portrait framing to landscape. |

## Evidence ownership and limits

- PRs: [Marketplace #636](https://github.com/compozy/compozy/pull/636) and
  [Session Context #635](https://github.com/compozy/compozy/pull/635). Inspect current
  implementation when making a new claim; historical screenshots are not live QA.
- Conversation provenance: Codex thread
  `01a0a080-b2e9-7a12-9d0e-8622cfd58e53`, 2026-09-14. An explicitly requested
  read-only subagent reviewed the whole JSONL history for this skill. Raw logs
  remain under the user's Codex home; they are not bundled into the repository.
- `packages/launch-video/research/remotion-components-2026-09-14/` preserves Exa
  discovery, official documentation, registry responses, and a dated recommendation.
  That report describes the pre-integration state; the current source and README
  supersede its old version/title/order/setup descriptions.
- `packages/launch-video/research/session-context-in-session-2026-09-14/` preserves
  the temporary integrated capture story and reproducible capture instructions.
- `packages/launch-video/research/x-references-2026-09-14/reference-study.md`
  preserves the historical ten-video analysis. Its initial image-only duration
  and no-opening contract were superseded later in the conversation. Referenced
  downloaded videos remain in the original local Downloads research folder.
- Exporting every camera shot separately with extra handles was a recommendation;
  the verified implementation used one 42-second Ultramock timeline with explicit
  Remotion source offsets. Either can work; do not describe a proposal as executed.
- The package is partly configurable, not an arbitrary video generator: title,
  version, sequence, trims and cues are data, while current screenshot interaction
  framing/hover timing still depends on the beta.26 layout in `shot.tsx`.

This skill was statically reviewed against the writing-skills checklist and the
recorded failures. Its future-run efficiency has not been measured in a comparative
model/harness evaluation.

## Tool compatibility encountered

The video helper downloaded references but its frame extraction failed on a
FFmpeg build that rejected `-vsync`. Direct FFmpeg extraction recovered the
frames; use supported options such as `-fps_mode vfr` where applicable, without
claiming a failed helper run completed the analysis. The system Python lacked
Pillow; the provided dependency runtime supplied it. Discover actual runtimes
rather than hard-coding that machine's cache path.

The first Remotion render contained a silent audio stream despite muted media
components. Subsequent renders used CLI `--muted`; inspect streams to verify the
requested no-audio deliverable. Silence and absence of an audio track differ.
A fade-end frame once appeared blank; inspect before/during/after the cut before
diagnosing a missing scene. Freeze detection was used on an earlier cut, not the
last revision, and is supplementary rather than a mandatory test.

The user's explicit Ultramock + Remotion choice owns this workflow. An installed
video skill's default framework does not authorize migrating it to HyperFrames.
