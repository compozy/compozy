# Bridge progress-storm and transcript-purity soak

- Legacy ID: AB-014
- Source: J-watch-agent-work-channel / NB-bridge-tool-progress, NB-provider-progress-rendering / CH-bridge-progress-stress
- Why automate: provider-specific edit, append, typing, and reaction policies are exercised behind one canonical progress projection. A burst can expose ordering, redaction, coalescing, terminal-state, queue-pressure, or transcript-pollution defects that isolated unit cases miss.
- Suggested layer: daemon-side E2E soak over fake endpoints for all eight providers, with a persisted-session transcript assertion.
- Spec sketch: emit a deterministic tool-event storm containing secrets, repeated updates, success, and failure. Assert each provider's permitted wire behavior, no GitHub or Linear issue progress writes, bounded calls under pressure, terminal-state preservation, and zero progress chrome in the ACP/session transcript.
- Status: proposed
