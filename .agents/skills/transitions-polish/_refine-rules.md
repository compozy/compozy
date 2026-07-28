Polish rules (transitions-polish) — apply on top of the motion tokens, match on USAGE not the nearest number:

DISTANCES (non-resting translate the element animates FROM, settling to 0):
  - 4px Micro: text swap
  - 6px Small: error shake (small segment)
  - 8px Base: badge diagonal reveal, page slide, error shake (large segment)
  - 12px Medium: text reveal
  - 30px Large: check badge appear
  A `transform` lane may carry `translate` (px) + `translateVarName` — check it against these by usage and propose a `distance` tweak when it differs; pass `translateVarName` through in the patch. A translate over ~40px on anything but a full panel/drawer reads sluggish — pull toward 8px Base.

OPEN/CLOSE ASYMMETRY — closes are faster and quieter than opens:
  - dropdown/modal: open 250ms, close 150ms. panel: open 400ms, close 350ms. toast close 350ms.
  - SYMMETRIC (same duration+easing both ways, do NOT split): page side-by-side 250ms, tabs sliding 250ms, accordion 250ms, icon swap 250ms, text swap 150ms.
  - overshoot easing belongs to entrances only (badge pop, number pop-in) — never bounce a close. most opens/closes share Smooth ease out.

HOVER IN vs OUT:
  - in: quick + direct, <=250ms, Smooth ease out.
  - out: softer/springier — may take longer and use cubic-bezier(0.34, 3.85, 0.64, 1) (avatar return) so it settles instead of snapping.

STAGGER & DELAY:
  - stagger offset 40ms per item (80ms for a few large items). keep total (offset x count) under ~300ms; for long lists shrink the offset or cap staggered items.
  - intent delay 80ms (tooltip appear, success-check path) filters accidental triggers / sequences two things — not padding for slow motion.
  - if motion feels late, trim DURATION before adding delay. NEVER delay a close or a hover-out.
