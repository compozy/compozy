# Blank-on-return hero — network drop to reconnect

- Legacy ID: AB-005
- Source: J-11 / RT-045, RT-043, RT-023 / `_tests.md` E2E-web 1, E2E-runtime 3; `_qa.md` §6 J-A flag
- Why automate: the headline network-drop→reconnect branch is regression-prone and only unit/component-owned today; the SSE drop/restore path with `after_sequence`, `epoch`/`generation`, explicit reset reasons, and REST-tail self-heal lacks a browser assertion.
- Suggested layer: E2E browser (`make test-e2e-web`) + a genuinely running background-session fixture.
- Spec sketch: open a running session, background it beyond `gcTime`, go offline, then restore; assert the thread never renders `ThreadEmpty`, reconnects gap-free, bounds matching deltas, handles a stale fence with a reset snapshot, self-heals a transient REST 5xx, and never fires the empty-while-active counter. True end state: the transcript is current on return with a truthful badge.
- Status: proposed
