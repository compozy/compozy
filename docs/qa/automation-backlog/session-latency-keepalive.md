# Session open-fast latency and keep-alive proxy soak

- Legacy ID: AB-008
- Source: J-12 / RT-046, RT-047, RT-044; J-15 / RT-023, RT-050 / `_tests.md` E2E-web 3/5/7, E2E-runtime 1; `_qa.md` §6 J-B/J-E flags
- Why automate: the open-fast budget (single loading phase, ≤2 round trips, no `/sessions` waterfall, no full-pane reflash) has no pinned latency assertion, and keep-alive-through-proxy behavior has no automated soak.
- Suggested layer: E2E web with route-loader prefetch and loading-phase-count assertions + an environment-gated keep-alive proxy soak.
- Spec sketch: cold-open and deep-link sessions of increasing size while asserting one loading phase and a first-paint budget; separately, hold an idle stream behind a buffering proxy and assert keep-alive comment frames arrive within the heartbeat. True end state: open feels instant at every size and idle streams survive proxy buffering.
- Status: proposed
