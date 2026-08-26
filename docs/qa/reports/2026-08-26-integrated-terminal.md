# QA Run Report — 2026-08-26 — Integrated Terminal

- **Scope:** Integrated terminal runtime, browser, CLI, HTTP/UDS, native-tool, hook, profile, configuration, audit, and window-manager behavior.
- **Cadence tier:** release-targeted
- **Build at bootstrap:** `ae5c2eb1c`
- **Environment:** isolated `devtool-oss-launch` lab; API `http://127.0.0.1:50584`
- **Started:** 2026-08-26T07:45:28Z
- **Status:** local targeted PASS; exact-head CI pending
- **Verdict:** PASS for targeted behavior; full exact-head verification remains owned by PR CI
- **Bootstrap manifest:** `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/bootstrap-manifest.json`

## Automated Evidence

| Lane | Result | Evidence |
|---|---|---|
| Runtime E2E after remediation | Pass: daemon 175/175, HTTP 21/21, UDS 46/46, harness 8/8, CLI 4/4 | `qa-artifacts/qa/test-e2e-runtime-after-fix.log` |
| Initial browser E2E discovery run | 245 passed, 22 failed, 3 skipped | `qa-artifacts/qa/test-e2e-web.log` |
| Terminal browser suite | 17/19 in the broad focused run; E2E-007 and E2E-020 then passed together after fixes | focused Playwright output from this run |
| Terminal stability checks | E2E-014, E2E-016, E2E-020 passed | focused Playwright output from this run |
| Adjacent regressions | Loops E2E-029, sandbox session, extension E2E-030, and session cancel/clear/delete passed after remediation | focused Playwright output from this run |
| Session deletion unit invariants | 41/41 passed through Turborepo | focused Vitest output from this run |
| Web production build | Pass | `make web-build` output from this run |
| Strict evidence audit | Pass: 0 blockers, 0 warnings | `qa-artifacts/qa/qa-audit-report.md` |
| Cross-surface Web read | Pass: three persisted tasks matched CLI, API, runtime, and Web | `qa-artifacts/qa/web-cross-surface.png` |
| Repository scoped gate | Blocked in internal lint after current codegen, integration, and Mage lanes passed | `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/gate-verify.log` |
| Exact-head required checks | Pending | Pull request CI |

The initial full browser lane was used as a discovery run. Every observed failure was either fixed and
retested in its focused owning suite or confirmed as an environmental limitation. Per operator direction,
the expensive full browser rerun is delegated to exact-head CI instead of being repeated locally.

## Journey Results

| Journey area | Verdict | Notes |
|---|---|---|
| Lease fencing, approval, takeover, and handoff | Pass | Runtime contracts and focused browser flows passed. |
| Profile selection and isolation | Pass | CLI attach now preserves profile scope; isolation regressions passed. |
| Redaction and terminal protocol boundaries | Pass | Runtime suite and focused protocol tests passed. |
| Journal, recording, and fail-closed behavior | Pass | Public workspace identity is resolved before durable journal access. |
| Streaming and raw terminal input | Pass | CLI raw mode preserves the detach chord without terminating the client. |
| CLI, HTTP/UDS, native tools, and hooks | Pass | Structured runtime lane passed after remediation. |
| Configuration lifecycle and limits | Pass | Terminal keys are now supported by structured `config set`; invalid scope remains rejected. |
| Browser terminal lifecycle and session handoff | Pass | Terminal/session focused browser regressions passed. |
| Official Compozy skill discovery | Pass | Existing discovery evidence remains valid; public surface names are unchanged. |
| Window-manager canary | Pass | Dock sizing and window routing canaries passed. |
| Windows capability parity | Blocked verify | This macOS lab cannot execute Windows ConPTY behavior. |
| Packaged desktop fidelity | Blocked verify | The packaged desktop shell was not available in this lab. |
| Published site docs first-run walk | Blocked verify | A deployed documentation-reader walk was not available in this lab. |

All in-scope scenario files now have `pass` or `blocked-verify`; none remain `untested` or `fail`.

## Defects Found and Fixed

| Bug | User-visible failure | Resolution |
|---|---|---|
| `BUG-20260826-terminal-journal-workspace-id` | Journal access rejected the workspace id exposed by the product. | Resolve the public registration id to the durable workspace identity before journal access. |
| `BUG-20260826-terminal-attach-profile-scope` | CLI attach could lose the selected profile. | Propagate the profile selector through attach and stream requests. |
| `BUG-20260826-terminal-cli-raw-mode` | The detach chord could terminate the CLI client. | Keep terminal input in raw mode and intercept the detach sequence locally. |
| `BUG-20260826-terminal-config-set-unsupported` | Terminal settings could not be changed through the CLI. | Add typed terminal configuration setters and validation. |
| `BUG-20260826-workspace-profile-agents-unavailable` | Profile-scoped agents could fail to start sessions. | Preserve workspace/profile agent lookup through the session path. |

No open product defect remains from this QA pass. The three remaining blocked verdicts require a different
platform or delivery environment and are not known failures.

## Platform Matrix

| Surface | macOS isolated lab | Windows | Packaged desktop | Deployed docs |
|---|---|---|---|---|
| Go runtime / CLI / HTTP / UDS | Pass | Not exercised | Not applicable | Not applicable |
| Browser application | Pass in focused remediation suites | Not exercised | Not applicable | Not applicable |
| ConPTY capability | Not available | Blocked verify | Not applicable | Not applicable |
| Desktop clipboard, accelerators, zoom, IME | Not exercised | Not exercised | Blocked verify | Not applicable |
| First-run published docs flow | Source contract checked | Same content expected | Same content expected | Blocked verify |

## Human Verifications Needed

1. Run the Windows ConPTY parity scenario on a Windows host.
2. Run the packaged desktop fidelity scenario with the release desktop build.
3. Walk the published terminal documentation from a deployed site as a first-time reader.

## Final Status

- **Runtime E2E:** pass after remediation.
- **Focused browser and unit regressions:** pass.
- **Open product defects:** 0.
- **Scenario coverage:** all in-scope scenarios resolved to `pass` or `blocked-verify`.
- **Local scoped gate:** blocked after its internal lint exceeded the configured timeout; passing lane evidence and the operator-directed CI handoff are recorded in `gate-verify.log`.
- **Lab teardown:** PASS — `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/teardown.json` records `"clean": true` and `survivors: []`.
- **Exact-head full verification:** delegated to required pull request CI.
- **Verdict:** targeted behavior PASS; ready for commit, rebase, and exact-head CI.
