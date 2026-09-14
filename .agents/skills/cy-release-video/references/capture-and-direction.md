# Capture and visual direction

## Demonstrate behavior in its parent surface

Capture from the implemented UI. Prefer an existing reproducible Storybook
scenario using production components, or a safe live session when live behavior
is required. Be explicit about demo fixtures: they demonstrate presentation and
interaction, not evidence that a provider reported those values in a live run.
Do not rebuild a lookalike application to obtain cleaner screenshots.

For Session Context the useful sequence is:

1. Session window, conversation, and composer establish the setting.
2. Hover the circular composer indicator: occupied context and capacity appear.
3. Click that same indicator: the Context sidebar opens in that same session.
4. Expand delivered-context details; move to tokens/costs inside the sidebar.

A standalone image of the sidebar's contents failed this brief: it hid the
composer indicator and the relationship to the session. Keep the conversation
visible when establishing the sidebar, even if later detail shots crop tightly.
Capture matching states of the same session, not different examples that only
look similar. Likewise, an extension click must lead to that same extension's
configuration, not another plugin's install dialog.

Use actual UI controls for hover, disclosure, and scrolling. A temporary composite
story can join existing components with deterministic fixtures; archive its
source outside production and remove only the temporary file you own afterward.
Retain the capture URL, fixture identity, viewport, device scale, and filenames
in the project so the next release can reproduce the states. Never persist browser
credentials or unrelated conversation/account data in the skill or assets.

## Source resolution and framing

- Increase capture resolution before zooming. Browser CSS size controls layout;
  device pixel ratio controls raster density. The corrected beta.26 captures used
  1440×900 CSS at DPR 3, yielding 4320×2700 PNGs with stable layout.
- Set the viewport before capturing the whole sequence, wait for fonts/content,
  and check dimensions. A larger viewport can change responsive layout and target
  coordinates; a DPR change generally preserves the CSS layout.
- Native screenshot pixels must cover their maximum projected size in the output.
  In a flat transform, projected width is CSS width × composition scale. With
  DPR 3, scale 2.7 has source headroom; camera perspective can demand more locally.
  Inspect the closest crop at output size instead of treating DPR 3 as a guarantee.
- Prefer lossless original PNGs. Enlarging a small screenshot, raising bitrate,
  or exporting at 4K cannot recover missing lettering. Recapture before adding
  sharpening or AI upscaling; altered UI text undermines the demo.
- Capture the app cleanly: no browser chrome, development overlays, disconnection
  toasts, selection handles, or unrelated personal data. Do not hide real product
  state to make an unsupported behavior claim.

## Motion direction

Give each move a target: a button, inspector, search result, toggle, or resource
list. Establish briefly, travel with easing, then leave enough readable time at
the target. Short reading beats are useful; long motionless holds between stills
were explicitly rejected.

Make the UI occupy the image. Historical study targets were about 90–115% screen
width and at least 85% during brief establishing shots; details can exceed 150%.
These are composition heuristics, not Ultramock Zoom values. Crop unimportant
margins while preserving the visual context needed to understand the action.

Vary pan, shallow pitch/yaw, distance, and field of view deliberately. Avoid
repeating the same centered slow zoom on every screenshot. Strong perspective
or blur must not make the target unreadable. Keep dark backgrounds consistent,
use restrained motion blur during travel, and overlap moving material between
shots. Do not add decorative movement that obscures what changed.
