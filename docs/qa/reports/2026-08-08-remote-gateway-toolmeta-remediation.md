# QA Run Report — 2026-08-08 — Remote Gateway tool metadata remediation

- **Scope:** Focused re-walk of Gateway native-tool bridge progress metadata, plus a Gateway audit canary
- **Cadence tier:** targeted
- **Build:** `4b507fe83246` plus the final review-remediation working tree
- **Environment:** isolated lab `remote-gateway-toolmeta-remediation-20260808-060444-758800`, daemon `127.0.0.1:50493`, [bootstrap manifest](/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-toolmeta-remediation-20260808-060444-758800-lab/qa-artifacts/qa/bootstrap-manifest.json)
- **Started:** 2026-08-08T05:33:33Z · **Completed:** 2026-08-08T06:55:23Z · **Status:** closed — one provider-dependent observation remains blocked-verify

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Maya | Teammate | laptop / wifi-slow / en-US | CH-bridge-progress-stress |
| Dora | Admin | desktop / wifi-fast / en-US | CH-gateway-audit-teardown |

## Flows in Scope

- `J-watch-agent-work-channel` — follow bounded, redacted tool progress without channel noise (`../journeys/J-watch-agent-work-channel.md`)
- `J-audit-and-teardown-gateway` — confirm the Gateway native tool remains operational through its public audit action (`../journeys/J-audit-and-teardown-gateway.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Result |
|---|---|---|---|---|---|---|
| 1 | CH-bridge-progress-stress | J-watch-agent-work-channel / NB-bridge-tool-progress | Maya | Garbage Tour | Blocked (needs human verify) | Deterministic bridge routing, redaction, terminal preservation, and transcript purity passed after one production fix; no authorized ACP/provider turn or fixture can visibly render `compozy__gateway` as `Reading` with `🌐`. |
| 2 | CH-gateway-audit-teardown | J-audit-and-teardown-gateway / RT-gateway-self-audit | Dora | Feature Tour | Pass | CLI, HTTP, UDS, and `compozy__gateway` returned the same local-only no-findings report; the native result completed with redaction enabled. |

Status legend: `Pass | Fixed | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Bridge progress

- The first deterministic bridge replay exposed `BUG-20260808-bridge-enable-without-config`: enabling a bridge without optional provider configuration returned HTTP 500 before the turn began.
- The store query now normalizes a nullable `provider_config` at the SQL boundary. A real-SQLite regression covers both canonical resource records and materialized bridge rows.
- The same `-race` bridge integration harness then passed routing, progress redaction, terminal preservation, and transcript purity.
- Public provider probes for Claude and Codex remained `classification=unknown`; daemon-side probes could not establish live status because neither declaration carries `auth_status_command`. The repository fixture invokes `compozy__terminal`, not `compozy__gateway`, so it cannot prove the new visible label and emoji.
- Evidence: `qa/test-cases/06-claude-provider-auth.json`, `07-codex-provider-auth.json`, `08-claude-provider-auth-local.json`, `09-codex-provider-auth-local.json`, `10-bridge-progress-integration.log`, and `qa/provider-attempt.json`.

### Gateway audit canary

- `compozy gateway audit -o json`, public HTTP, UDS, and `compozy tool invoke compozy__gateway --input '{"action":"audit"}' -o json` all returned the same report.
- The report was explicit: `ran=true`, `no_findings=true`, `local_only=true`, zero findings, and zero addresses. The native invocation completed and marked the structured output redacted.
- Evidence: `qa/test-cases/01-cli-gateway-audit.json` through `05-gateway-audit-canary.json`.

## What Was Fixed

- Registered `compozy__gateway` in the canonical native progress metadata inventory as `Reading`, `🌐`, with automatic safe preview.
- Changed the SDK Go generator regression to compare semantic field signatures instead of `gofmt` spacing.
- Normalized nullable bridge provider configuration in `GetGatewayBridgeIngressSubject`, preventing bridge enablement from failing before unavailable-target classification.

## Defects Found and Re-Walked

- `BUG-20260808-bridge-enable-without-config` — verified: bridges without optional provider configuration resolve as target-unavailable, and the complete bridge integration replay reaches its terminal state.

## Runtime Errors Observed

- One pre-fix HTTP 500 from the nullable `provider_config` scan; fixed in production and re-walked.
- Expected provider probe refusals because live auth status cannot be classified from the isolated declarations.
- One observer-side shell quoting mistake on the first native invocation; the corrected identical command completed successfully and is the retained evidence.

## Human Verifications Needed

- With an operator-authorized ACP provider and a configured editable bridge, trigger a bridged turn that calls `compozy__gateway`, confirm the public progress line uses `Reading` and `🌐`, then fresh-read the session transcript to confirm no progress chrome was persisted.

## Decisions for a Human

None. The remaining item is an external verification prerequisite, not a product decision.

## Learnings

- Optional JSON selected from a union must be normalized at the SQL boundary before it enters a non-nullable generated field.
- Generic bridge progress fixtures prove lifecycle safety but cannot prove tool-specific presentation unless they emit the exact native tool ID.

## Final Status

- **Focused automated validation:** PASS — the real-SQLite regression, full Gateway ingress lifecycle suite, deterministic bridge integration harness, scoped Go lint, codegen check, four-surface Gateway audit canary, and full repository gate passed. The verbatim gate evidence is `/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-toolmeta-remediation-20260808-060444-758800-lab/qa-artifacts/qa/final-make-verify.log`.
- **Scenario verdicts:** `RT-gateway-self-audit` pass; `NB-bridge-tool-progress` blocked-verify with fresh provider-boundary evidence.
- **Issue status:** one Blocks-Completion defect found, fixed, and re-walked; zero open defects from this focused run.
- **Evidence audit:** PASS — [the strict audit](/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-toolmeta-remediation-20260808-060444-758800-lab/qa-artifacts/qa/qa-audit-report.json) records zero blockers and zero warnings.
- **Teardown:** PASS — [the manifest-owned teardown](/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-toolmeta-remediation-20260808-060444-758800-lab/qa-artifacts/qa/teardown.json) records `clean: true`, the registered daemon terminated, and no survivors.
- **Verdict: BLOCKED** — every locally executable check passed, but the exact public `Reading` / `🌐` observation requires an authorized ACP/provider turn that this environment cannot establish.
