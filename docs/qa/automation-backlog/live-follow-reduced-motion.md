# Live-follow streaming and reduced-motion sweep

- Legacy ID: AB-006
- Source: J-13 / RT-054, RT-058, RT-059 / `_tests.md` E2E-web 2/8/9, E2E-runtime 2; `_qa.md` §6 J-C flag
- Why automate: streaming indicators, the three-mode scroll machine, and composer queue are unit/component and screenshot-owned today; the full follow-and-steer flow under reduced motion has no real-daemon browser assertion.
- Suggested layer: E2E browser (streaming render + scroll hold + queue order) + a Storybook reduced-motion capture gate.
- Spec sketch: stream a 1k+ event turn; assert incremental apply without stall, scroll-hold plus follow-pill restore, two queued prompts dispatching in order, and reduced motion degrading the working pulse to a static label. True end state: a settled turn is readable with truthful status and in-order queue landing.
- Status: proposed
