# QA Run Report — 2026-08-07 — Remote Gateway

- **Scope:** Remote Gateway Tasks 01–09 — fail-closed exposure, pairing, public ingress, remote CLI, SSH forwarding, self-audit, operator UI, extension authoring, and teardown
- **Cadence tier:** full
- **Build:** `2ca0168ccfd7` plus the Task 09 QA working tree
- **Environment:** targeted lab `remote-gateway-20260807-202655-957508`, daemon `127.0.0.1:52055`, derived web proxy `http://127.0.0.1:52055`, browser-use, [bootstrap manifest](/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/bootstrap-manifest.json)
- **Started:** 2026-08-07T20:16:58Z · **Completed:** 2026-08-07T21:47:49Z · **Status:** complete — overall verdict remains blocked-verify for named external prerequisites

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Iris | Power User | laptop / wifi-slow / en-US | live revocation, no-device recovery, remote CLI interruption |
| Dora | Power User | desktop / wifi-fast or flaky / en-US | provider degradation, audit and teardown |
| Bruno | Power User | desktop / flaky / en-US | mid-delivery exposure, SSH ownership |
| Ada | Developer | desktop / wifi-fast / en-US | extension template and official-skill discovery |

## Journeys Walked

- `J-expose-and-pair-gateway` — local pairing, rename, revoke, empty-inventory recovery, consent, provider refusal, degradation, and restart; real remote-device legs blocked by missing provider authorization.
- `J-deliver-through-public-gateway` — one real signed workspace webhook dispatched one attributed Loop; public address, bridge callback, and sender downtime legs blocked by missing provider authorization.
- `J-operate-remote-gateway-cli` — deterministic unreachable failure and atomic cleanup re-walked; real HTTPS profile, remote work, and reconnect blocked by missing provider authorization.
- `J-connect-gateway-ssh` — unreachable failure proved no mutation or owned resources; real launch/reuse and teardown blocked by the absence of an authorized SSH host.
- `J-audit-and-teardown-gateway` — degraded finding, remediation, Web state, byte-equivalent CLI/HTTP/UDS/native no-findings output, and clean lab teardown passed.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Status | Result / blocker | Fix |
|---|---|---|---|---|---|
| 1 | CH-gateway-live-revocation | expose and pair / paired device, public consent, stream reconnect | Blocked (needs human verify) | Local device lifecycle passed; a real remote stream requires an authorized Tailscale address. | |
| 2 | CH-gateway-provider-degradation | expose and pair / local-only boot, provider route, trust, operator truth | Fixed | Missing binding degraded safely, stayed unadvertised, and restarted local-only with an actionable cause. | live config copy; provider cause; degraded boot |
| 3 | CH-gateway-no-device-recovery | expose and pair / no-device recovery, paired device, public consent | Blocked (needs human verify) | Empty inventory and replacement pairing passed locally; remote back/refresh requires an authorized address. | |
| 4 | CH-gateway-mid-delivery-exposure | public ingress / ingress bindings, offline delivery, TA-056, TA-060, bridge setup | Blocked (needs human verify) | Local HMAC pipeline and boundary failures passed; public route, bridge callback, and sender downtime require an authorized provider. | |
| 5 | CH-gateway-remote-cli-interruption | remote CLI / profile, reconnect, no-device recovery | Blocked (needs human verify) | A TCP dial failure now leaves no profile, credential, or journal; real remote work/reconnect requires an authorized address. | profile recovery |
| 6 | CH-gateway-ssh-ownership | SSH / scoped forward | Blocked (needs human verify) | Unreachable target failed before mutation with no process or listener; launch/reuse needs an authorized SSH host. | |
| 7 | CH-gateway-audit-teardown | audit / self-audit, provider route, local boot, redaction | Fixed | Provider finding cleared through its remediation; four structured planes became identical after native wiring fix. | native gateway wiring |

Status legend: `Pass | Fixed | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Device lifecycle and recovery

- Minted and redeemed real local browser device sessions, renamed one, revoked it, observed zero active devices, and redeemed a replacement without restart.
- Public consent named the operator UI, management API, paired-device gate, lack of public pairing, immediate disable behavior, and restart semantics. A provider-less submit left the surface off.
- Evidence: `qa/screenshots/06-two-paired-devices.png`, `08-self-device-revoked-local-recovery.png`, `09-zero-device-local-recovery.png`, `11-replacement-device-new-identity.png`, `12-public-operator-consent.png`, `13-public-operator-refused-no-provider.png`; `qa/test-cases/22-device-list-after-keyboard-rename.json`, `23-device-list-zero-active.json`, `24-replacement-device-list.json`, `36-public-operator-consent.json`.

### Provider degradation and local-only boot

- Default posture and live ceiling remained local-only. Missing `TS_AUTHKEY` produced a redacted, actionable cause, no endpoint, and an audit finding.
- Restart initially aborted even after safe degradation; production now distinguishes compensated provider degradation and continues local-only.
- Evidence: `qa/test-cases/07-live-enable-config-set.json`, `12-provider-enable-no-authkey.json`, `13-provider-status-no-authkey.json`, `14-provider-down-audit.json`, `16-provider-disable-remediation.json`, `17-audit-after-provider-remediation.json`, `33-provider-restart-rewalk.json`.

### Public delivery

- Invalid signature and stale timestamp returned 401, a valid delivery returned 200 and created one completed attributed Loop, and replay returned 409.
- A disabled trigger returned 409 and a 1 MiB-plus request returned 413. Neither added a run. Trigger reads projected `reachability: off` and no invented URL.
- Evidence: `qa/test-cases/34-signed-webhook-local-pipeline.json`, `40-webhook-boundaries-and-projection.json`.

### Remote CLI and SSH interruption

- Before the fix, a refused TCP connection left recovery pending indefinitely. After the fix, the same failure returned `gateway_reachability_failed`, kept `connect list` usable, and left zero profile, credential, or journal files.
- An unreachable SSH target returned `gateway_ssh_unreachable` before any profile, credential, process, or listener appeared.
- Evidence: `qa/test-cases/35-remote-cli-interruption.json`, `39-ssh-unreachable-no-mutation.json`.

### Audit, native tool, and extension discovery

- CLI, HTTP, UDS, and Web agreed on degraded and remediated posture. Native invocation initially panicked because it captured a typed-nil pre-boot service; late binding made all four structured payloads canonical-hash identical and redacted.
- CLI help initially omitted both connectivity templates. It now derives from the embedded catalog; both templates scaffold. The official skill serves their authoring and Gateway-operation references.
- Clean external provider builds remain blocked because the published Go SDK lacks the new API and the TypeScript package is inside this machine's minimum-release-age window. No repository-local override was used.
- Evidence: `qa/test-cases/37-audit-local-only-parity.json`, `38-native-audit-parity.json`, `41-extension-template-discovery.json`; `qa/screenshots/14-audit-local-only-no-findings.png`.

## Defects Found and Re-Walked

- `BUG-20260807-gateway-live-config-copy` — verified: Settings now states that `gateway.enabled` applies immediately.
- `BUG-20260807-gateway-provider-cause` — verified: missing `TS_AUTHKEY` crosses provider RPC as an actionable value-free error; unknown failures remain masked.
- `BUG-20260807-gateway-provider-boot` — verified: compensated provider degradation no longer aborts local daemon startup.
- `BUG-20260807-gateway-profile-recovery` — verified: a TCP dial failure rolls back the unused local pairing transaction.
- `BUG-20260807-gateway-native-tool-wiring` — verified: the native gateway tool resolves the post-boot service and matches every audit plane.
- `BUG-20260807-extension-template-help` — verified: extension init help lists every embedded scaffold.

## Runtime Errors Observed

- Expected fail-closed refusals when a tier or surface lacked an active provider.
- Expected Tailscale degradation because the authorized provider environment had no `TS_AUTHKEY`; no stub or alternate provider replaced it.
- One pre-fix daemon abort during degraded-provider reconciliation and one pre-fix native-tool panic; both were fixed in production and re-walked.
- One pre-fix persistent pairing recovery journal after a refused dial; the corrected path now cleans it atomically.

## Human Verifications Needed

- Bind an authorized `TS_AUTHKEY`, enable each tier, verify the real HTTPS address and challenge, pair a second machine, exercise fresh stream tickets, revoke during live work, and repeat zero-device back/refresh recovery.
- Through the same public provider path, bind a webhook and bridge callback, change exposure during delivery, stop the daemon, and confirm sender-visible failure plus exactly-once sender redelivery.
- Use an authorized SSH host with matching Compozy version to verify absent/running daemon paths, non-default remote home, accepted work, tunnel loss, and teardown of owned resources only.
- Publish a Compozy extension SDK release containing the connectivity-provider API, then repeat clean Go and TypeScript build/validate without local dependency overrides.

## Decisions for a Human

None. Missing external authorization and the unpublished public SDK are verification blockers, not design choices.

## Scenario Verdicts

- **Pass (6):** ET-compozy-official-skill-discovery; MS-gateway-config-ceiling; RT-gateway-local-only-boot; RT-gateway-self-audit; TA-056; TA-060.
- **Blocked-verify (16):** ET-connectivity-provider-trust; ET-extension-agent-guided-authoring; ET-extension-code-first-authoring; ET-extension-dx-scorecard; ET-extension-manifest-v2-surfaces; NB-bridge-provider-setup; RT-connectivity-provider-route; RT-gateway-browser-stream-reconnect; RT-gateway-no-device-recovery; RT-gateway-offline-delivery-redelivery; RT-gateway-operator-surface-truth; RT-gateway-paired-device; RT-gateway-public-ingress-bindings; RT-gateway-public-ui-consent; RT-gateway-remote-cli-profile; RT-gateway-ssh-forward.
- **Untested/fail:** none.

## Final Status

- **Focused validation:** PASS — focused Go race suites covered the CLI profile transaction, extension help, daemon native gateway wiring, compensated provider policy, bundled provider RPC errors, and degraded-provider integration boot; the Web lint, typecheck, codegen check, and all 4,485 Web tests passed through Turborepo.
- **Workstream gate evidence:** `qa/logs/make-verify.log` — the earlier full escalation failed on 10 Go lint findings; all 10 were remediated and passed focused lint and race validation. The required final `make gate-full` remains intentionally deferred until after the single end-of-batch deep review.
- **Evidence audit:** PASS — the strict auditor reported zero blockers and zero warnings in `qa/qa-audit-report.json`.
- **Teardown:** PASS — `qa/teardown.json` records `clean: true`, no survivors, and both registered daemon and Web processes stopped; neither isolated port retained a listener.
- **Issues by user impact:** Blocks-Completion 3 · Data-Loss 0 · Trust-Damage 2 · Friction 1 · Cosmetic 0 — all six fixed and re-walked
- **Coverage:** 5/5 journeys walked; 7/7 charter sessions settled; 22/22 changed scenarios have pass or blocked-verify verdicts
- **Verdict: BLOCKED** — every locally executable journey and defect re-walk passed. Remote provider, public ingress, real SSH, and clean public-SDK sign-off remain blocked by the named external prerequisites.
