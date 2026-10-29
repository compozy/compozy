# QA Run Report — 2026-10-29 — Clear conversation latency

- Scope: issue #1033, asynchronous provider startup after durable conversation reset.
- Cadence: targeted; CH-021 clear subset and next-message canary, RT-017.
- Build: `fix/issue-1033-clear-conversation-latency`.
- Environment: isolated daemon home, socket, HTTP listener and project; real Claude Sonnet 5 native CLI with operator-home policy. No operator daemon/session mutation or provider environment stripping.
- Verdict: PASS for targeted journeys; current-head CI and external reviews are tracked in the PR.

## Observations

| Observation | Installed baseline | Final implementation |
| --- | ---: | ---: |
| POST clear | 2764.58 ms | 270.35 ms |
| Concurrent outline read, started 200 ms after clear | 2531.21 ms | 61.69 ms |
| Transcript read after response | 1.14 ms | 7.57 ms |
| Response state | active | starting |
| Entries after clear | 0 | 0 |

These are single-run observations in the same lab, not a percentile benchmark. The original incident recorded 12.725 s total with 12.465 s in provider startup. The lab baseline was faster. A separate Web clear observed 2143 ms initialize plus 12830 ms session/new while the thread displayed the explicit starting state.

## Journey results

| Journey | Result | Evidence |
| --- | --- | --- |
| Clear a completed Claude conversation in the same session | Pass | Real confirmation dialog; independent HTTP returned empty history |
| Show readiness and prevent sending during startup | Pass | Rendered “Getting conversation ready…” and disabled “Session is starting…” composer; no transcript skeleton, Thinking indicator or turn-cancel control |
| Reload after readiness | Pass | Empty thread remained in the same session |
| Send after readiness | Pass | Claude returned a new welcome greeting after clear/reload |
| Durable repeated-clear epochs | Pass | Real SQLite regression and final isolated HTTP check |
| Presence during provider startup | Pass | Same acquired lease renewed during real startup; blocked-start race test also renews the same lease |
| Provider startup failure | Pass, automated | Existing clear suite retains empty history and typed startup failure, then resumes the same session |

The initial shared-browser observations were discarded after another executor reported a tab collision. The final journey used a dedicated headless Chrome instance at 1440 × 900, with a fresh profile and guaranteed cleanup. Rendered screenshots are attached through durable R2 links in the PR.

## Focused verification

- Go `-race`: clear, HTTP clear, lifecycle, presence and session-action suites passed. After the epoch wiring change, the complete clear suite passed again (17.505 s).
- Starting-state rendering plus two adjacent thread cases: 3 tests passed after reproducing the misleading turn-cancel control.
- Web actions/presence: 52 tests passed through root Turborepo, including pending-clear cache retention.
- Web typecheck and lint passed through root Turborepo, with zero lint warnings/errors.
- React Doctor through root Turborepo: 100/100, no findings.
- Test-shape checks passed for both modified Go suites.

## Limits

Heavy gates belong to CI by assignment; no local `make gate`, full E2E or full gate run. The default Turborepo dependency codegen-check hit an existing OpenDesign embedded-linter freshness mismatch under local Bun 1.4.4; focused tasks used `--only`. Local golangci-lint could not load the config because it was compiled with Go 1.26 while the repository requires Go 1.27.1. Required current-head CI and both external reviews remain the delivery authority.

Final HTTP validation advanced epoch 2 to 3, with 24 successful renewals (204) of the same lease during 23 seconds of provider startup; transcript reads remained empty and took 1.06–13.91 ms.

Large-history performance and other providers were not re-benchmarked. The evidence audit accepts the captured surfaces but flags C14 (missing local full gate); that gate is explicitly forbidden by this assignment and remains CI-owned.

The provider-failure recovery check uses the existing unit-test driver boundary; the success/readiness/reload path uses real Claude.
