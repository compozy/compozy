# Goal public parity and crash-phase replay harness

- Legacy ID: AB-012
- Source: J-29 / GL-025..GL-032, GL-034, GL-036, GL-037, GL-039 / Codex round-8 R-002
- Why automate: cross-surface parity and no-blind-replay guarantees span HTTP/UDS/CLI/native tools plus several crash boundaries; the borrowed-origin daemon adoption route also lacks one direct persisted-witness assertion.
- Suggested layer: API/integration parity harness + E2E runtime crash matrix + direct real-GlobalDB daemon binder integration.
- Spec sketch: byte/semantic-compare every structured Goal operation, prove internal `/goal` text remains literal, restart at pre-claim/queued/post-claim/post-response/post-judge/control-notifier phases, and assert generation-scoped origin adoption persists/replays the exact attempt. True end state: one workspace-scoped audit and no duplicate external effect.
- Status: proposed
