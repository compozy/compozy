# Agent-operability CLI↔HTTP↔UDS↔native-tool parity harness

- Legacy ID: AB-003
- Source: J-07 / LP-025..LP-028 / `_tests.md` E2E-runtime-3, E2E-runtime-5, Integration-27, Integration-28
- Why automate: regression-prone cross-surface contract; every `agh__loop_*` verb must match CLI/HTTP/UDS on the same inputs, the no-self-approval capability gate must hold, and `Unavailable(ReasonDependencyMissing)` must be deterministic.
- Suggested layer: API/integration (Go harness) driving the full verb set across all four surfaces + a native-tool-vs-HTTP diff assertion.
- Spec sketch: for run/dry-run/configure/pause/resume/stop/approve/list/inspect/status/runs/edit/delete, assert identical terminal outcomes and structured payloads; assert an agent cannot approve its own gate and token redaction stays hash-form only. True end state: structured agent operation equals operator operation.
- Status: proposed
