# Integrated session context captures

Captured on 2026-09-14 using the production SessionThread, SessionContextControl,
and SessionInspector components with existing Storybook fixtures. No provider
calls or production code changes are required.

To recapture, temporarily place the adjacent story in
`web/src/systems/session/components/stories/`, start the existing Storybook, and
open `iframe.html?id=release-sessioncontext--session&viewMode=story`.
Use a 1440 x 900 viewport with device scale factor 3. Capture the initial state,
hover the composer context ring at (158, 776), click it, then click the inspector
context disclosure at (1130, 331). Finally scroll the inspector to tokens/costs.
Reset the viewport and remove the temporary story after capture.

Assets: `public/releases/beta26/session-{closed,tooltip,open,expanded,tokens}.png`.
Ultramock project: https://www.ultramock.io/?project=cmu1eadzy000204js9kdosdyp
Export: ultramock-timeline-16-9-5s-2026-09-14T17-29-28-674Z.mp4.
The first two five-second camera scenes were replaced; marketplace scenes retain
their existing timing and camera settings. Remotion starts with a seven-second
interaction that establishes the session and indicator before opening the panel.
